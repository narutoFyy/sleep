//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newPrimaryHealthTestCache(t *testing.T) (*primaryHealthCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &primaryHealthCache{rdb: rdb}, mr
}

func primaryHealthTestScope() service.PrimaryHealthScope {
	return service.PrimaryHealthScope{GroupID: 44, ModelHash: "64-character-model-hash"}
}

func TestPrimaryHealthCache_DuplicateFailureDoesNotSubstituteForAllPrimaries(t *testing.T) {
	cache, _ := newPrimaryHealthTestCache(t)
	ctx := context.Background()
	scope := primaryHealthTestScope()

	first := service.PrimaryFailureEvidence{AccountID: 11, RequestID: "request-1", AttemptID: "attempt-1", Reason: "timeout", StatusCode: 504, FailedAtUnixMilli: 1}
	require.NoError(t, cache.RecordFailure(ctx, scope, first, []int64{11, 12}, 30*time.Second))
	first.AttemptID = "attempt-2"
	require.NoError(t, cache.RecordFailure(ctx, scope, first, []int64{11, 12}, 30*time.Second))

	state, err := cache.GetCircuitState(ctx, scope)
	require.NoError(t, err)
	require.False(t, state.Open)
	evidence, err := cache.ListFailureEvidence(ctx, scope)
	require.NoError(t, err)
	require.Len(t, evidence, 1)
	require.Equal(t, "attempt-2", evidence[0].AttemptID)
	require.Equal(t, "timeout", evidence[0].Reason)
	require.Equal(t, 504, evidence[0].StatusCode)
}

func TestPrimaryHealthCache_AllPrimariesOpenCircuitWithoutExtendingTTL(t *testing.T) {
	cache, mr := newPrimaryHealthTestCache(t)
	ctx := context.Background()
	scope := primaryHealthTestScope()

	require.NoError(t, cache.RecordFailure(ctx, scope, service.PrimaryFailureEvidence{AccountID: 11}, []int64{11, 12}, 30*time.Second))
	require.NoError(t, cache.RecordFailure(ctx, scope, service.PrimaryFailureEvidence{AccountID: 12}, []int64{11, 12}, 30*time.Second))
	state, err := cache.GetCircuitState(ctx, scope)
	require.NoError(t, err)
	require.True(t, state.Open)
	require.Equal(t, 30*time.Second, state.Remaining)

	mr.FastForward(10 * time.Second)
	require.NoError(t, cache.RecordFailure(ctx, scope, service.PrimaryFailureEvidence{AccountID: 11}, []int64{11, 12}, 30*time.Second))
	state, err = cache.GetCircuitState(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, 20*time.Second, state.Remaining, "duplicate failures must not extend an open circuit")

	mr.FastForward(20 * time.Second)
	state, err = cache.GetCircuitState(ctx, scope)
	require.NoError(t, err)
	require.False(t, state.Open)
}

func TestPrimaryHealthCache_ProbeLeaseExcludesPeersUntilTTL(t *testing.T) {
	cache, mr := newPrimaryHealthTestCache(t)
	ctx := context.Background()
	scope := primaryHealthTestScope()

	acquired, err := cache.TryAcquireProbeLease(ctx, scope, 11, "instance-a", 60*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)
	acquired, err = cache.TryAcquireProbeLease(ctx, scope, 11, "instance-b", 60*time.Second)
	require.NoError(t, err)
	require.False(t, acquired)

	mr.FastForward(60 * time.Second)
	acquired, err = cache.TryAcquireProbeLease(ctx, scope, 11, "instance-b", 60*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)
}

func TestPrimaryHealthCache_FailureResetsConsecutiveProbeSuccesses(t *testing.T) {
	cache, _ := newPrimaryHealthTestCache(t)
	ctx := context.Background()
	scope := primaryHealthTestScope()
	evidence := service.PrimaryFailureEvidence{AccountID: 11, RequestID: "request-1"}

	require.NoError(t, cache.RecordFailure(ctx, scope, evidence, []int64{11}, 30*time.Second))
	result, err := cache.RecordProbeSuccess(ctx, scope, 11, 2)
	require.NoError(t, err)
	require.Equal(t, 1, result.ConsecutiveSuccesses)
	require.False(t, result.Recovered)

	evidence.RequestID = "request-2"
	require.NoError(t, cache.RecordFailure(ctx, scope, evidence, []int64{11}, 30*time.Second))
	result, err = cache.RecordProbeSuccess(ctx, scope, 11, 2)
	require.NoError(t, err)
	require.Equal(t, 1, result.ConsecutiveSuccesses, "a failure must reset the prior success")
	require.False(t, result.Recovered)
}

func TestPrimaryHealthCache_TwoProbeSuccessesRecoverAndCloseCircuit(t *testing.T) {
	cache, _ := newPrimaryHealthTestCache(t)
	ctx := context.Background()
	scope := primaryHealthTestScope()

	require.NoError(t, cache.RecordFailure(ctx, scope, service.PrimaryFailureEvidence{AccountID: 11}, []int64{11}, 30*time.Second))
	result, err := cache.RecordProbeSuccess(ctx, scope, 11, 2)
	require.NoError(t, err)
	require.False(t, result.Recovered)
	result, err = cache.RecordProbeSuccess(ctx, scope, 11, 2)
	require.NoError(t, err)
	require.True(t, result.Recovered)
	require.Equal(t, 2, result.ConsecutiveSuccesses)

	state, err := cache.GetCircuitState(ctx, scope)
	require.NoError(t, err)
	require.False(t, state.Open)
	evidence, err := cache.ListFailureEvidence(ctx, scope)
	require.NoError(t, err)
	require.Empty(t, evidence)
}

func TestPrimaryHealthCache_RedisErrorsAreReturned(t *testing.T) {
	cache, mr := newPrimaryHealthTestCache(t)
	mr.Close()

	_, err := cache.GetCircuitState(context.Background(), primaryHealthTestScope())
	require.Error(t, err)
}
