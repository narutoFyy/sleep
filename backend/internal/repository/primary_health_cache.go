package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const primaryHealthKeyPrefix = "primary_health:v1:"

var primaryHealthRecordFailureScript = redis.NewScript(`
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('HDEL', KEYS[2], ARGV[1])

for index = 4, #ARGV do
  if redis.call('HEXISTS', KEYS[1], ARGV[index]) == 0 then
    return 0
  end
end

redis.call('SET', KEYS[3], ARGV[3], 'PX', ARGV[3], 'NX')
return 1
`)

var primaryHealthProbeSuccessScript = redis.NewScript(`
if redis.call('HEXISTS', KEYS[1], ARGV[1]) == 0 then
  redis.call('HDEL', KEYS[2], ARGV[1])
  return {0, 0}
end

local successes = redis.call('HINCRBY', KEYS[2], ARGV[1], 1)
if successes >= tonumber(ARGV[2]) then
  redis.call('HDEL', KEYS[1], ARGV[1])
  redis.call('HDEL', KEYS[2], ARGV[1])
  redis.call('DEL', KEYS[3])
  return {successes, 1}
end
return {successes, 0}
`)

type primaryHealthCache struct {
	rdb *redis.Client
}

func NewPrimaryHealthCache(rdb *redis.Client) service.PrimaryHealthStore {
	return &primaryHealthCache{rdb: rdb}
}

func (c *primaryHealthCache) RecordFailure(
	ctx context.Context,
	scope service.PrimaryHealthScope,
	evidence service.PrimaryFailureEvidence,
	enabledPrimaryAccountIDs []int64,
	circuitTTL time.Duration,
) error {
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("marshal primary failure evidence: %w", err)
	}

	ttlMillis := circuitTTL.Milliseconds()
	if ttlMillis < 1 {
		return fmt.Errorf("primary circuit TTL must be positive")
	}
	keys := primaryHealthKeys(scope)
	args := make([]interface{}, 0, 3+len(enabledPrimaryAccountIDs))
	args = append(args, evidence.AccountID, string(evidenceJSON), ttlMillis)
	for _, accountID := range enabledPrimaryAccountIDs {
		args = append(args, accountID)
	}
	return primaryHealthRecordFailureScript.Run(ctx, c.rdb, []string{keys.failures, keys.probeSuccesses, keys.circuit}, args...).Err()
}

func (c *primaryHealthCache) GetCircuitState(ctx context.Context, scope service.PrimaryHealthScope) (service.PrimaryCircuitState, error) {
	remaining, err := c.rdb.PTTL(ctx, primaryHealthKeys(scope).circuit).Result()
	if err != nil {
		return service.PrimaryCircuitState{}, err
	}
	if remaining <= 0 {
		return service.PrimaryCircuitState{}, nil
	}
	return service.PrimaryCircuitState{Open: true, Remaining: remaining}, nil
}

func (c *primaryHealthCache) ListFailureEvidence(ctx context.Context, scope service.PrimaryHealthScope) ([]service.PrimaryFailureEvidence, error) {
	values, err := c.rdb.HGetAll(ctx, primaryHealthKeys(scope).failures).Result()
	if err != nil {
		return nil, err
	}
	evidence := make([]service.PrimaryFailureEvidence, 0, len(values))
	for _, raw := range values {
		var item service.PrimaryFailureEvidence
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("unmarshal primary failure evidence: %w", err)
		}
		evidence = append(evidence, item)
	}
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].AccountID < evidence[j].AccountID })
	return evidence, nil
}

func (c *primaryHealthCache) TryAcquireProbeLease(
	ctx context.Context,
	scope service.PrimaryHealthScope,
	accountID int64,
	owner string,
	ttl time.Duration,
) (bool, error) {
	key := primaryHealthKeys(scope).probeLease(accountID)
	return c.rdb.SetNX(ctx, key, owner, ttl).Result()
}

func (c *primaryHealthCache) RecordProbeSuccess(
	ctx context.Context,
	scope service.PrimaryHealthScope,
	accountID int64,
	successesNeeded int,
) (service.PrimaryProbeResult, error) {
	keys := primaryHealthKeys(scope)
	result, err := primaryHealthProbeSuccessScript.Run(
		ctx,
		c.rdb,
		[]string{keys.failures, keys.probeSuccesses, keys.circuit},
		accountID,
		successesNeeded,
	).Result()
	if err != nil {
		return service.PrimaryProbeResult{}, err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return service.PrimaryProbeResult{}, fmt.Errorf("unexpected primary probe result %T", result)
	}
	successes, err := redisScriptInt64(values[0])
	if err != nil {
		return service.PrimaryProbeResult{}, err
	}
	recovered, err := redisScriptInt64(values[1])
	if err != nil {
		return service.PrimaryProbeResult{}, err
	}
	return service.PrimaryProbeResult{
		ConsecutiveSuccesses: int(successes),
		Recovered:            recovered == 1,
	}, nil
}

type primaryHealthKeySet struct {
	base           string
	failures       string
	probeSuccesses string
	circuit        string
}

func primaryHealthKeys(scope service.PrimaryHealthScope) primaryHealthKeySet {
	base := fmt.Sprintf("%s%d:%s", primaryHealthKeyPrefix, scope.GroupID, scope.ModelHash)
	return primaryHealthKeySet{
		base:           base,
		failures:       base + ":failures",
		probeSuccesses: base + ":probe_successes",
		circuit:        base + ":circuit",
	}
}

func (k primaryHealthKeySet) probeLease(accountID int64) string {
	return k.base + ":probe_lease:" + strconv.FormatInt(accountID, 10)
}

func redisScriptInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse Redis script integer %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected Redis script integer %T", value)
	}
}
