package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	PrimaryCircuitOpenDuration  = 30 * time.Second
	PrimaryProbeLeaseDuration   = 60 * time.Second
	PrimaryProbeSuccessesNeeded = 2
)

// PrimaryHealthScope isolates health evidence by group and normalized requested model.
// ModelHash is used instead of a raw model name so Redis keys do not expose model names.
type PrimaryHealthScope struct {
	GroupID   int64
	ModelHash string
}

// PrimaryFailureEvidence records the upstream attempt that proved a primary unhealthy.
type PrimaryFailureEvidence struct {
	AccountID         int64  `json:"account_id"`
	RequestID         string `json:"request_id,omitempty"`
	AttemptID         string `json:"attempt_id,omitempty"`
	Reason            string `json:"reason,omitempty"`
	StatusCode        int    `json:"status_code,omitempty"`
	FailedAtUnixMilli int64  `json:"failed_at_unix_milli"`
}

type PrimaryCircuitState struct {
	Open      bool
	Remaining time.Duration
}

type PrimaryProbeResult struct {
	ConsecutiveSuccesses int
	Recovered            bool
}

// PrimaryHealthStore persists cross-instance primary health and circuit state.
type PrimaryHealthStore interface {
	RecordFailure(ctx context.Context, scope PrimaryHealthScope, evidence PrimaryFailureEvidence, enabledPrimaryAccountIDs []int64, circuitTTL time.Duration) error
	GetCircuitState(ctx context.Context, scope PrimaryHealthScope) (PrimaryCircuitState, error)
	ListFailureEvidence(ctx context.Context, scope PrimaryHealthScope) ([]PrimaryFailureEvidence, error)
	TryAcquireProbeLease(ctx context.Context, scope PrimaryHealthScope, accountID int64, owner string, ttl time.Duration) (bool, error)
	RecordProbeSuccess(ctx context.Context, scope PrimaryHealthScope, accountID int64, successesNeeded int) (PrimaryProbeResult, error)
}

type PrimaryHealthService struct {
	store         PrimaryHealthStore
	now           func() time.Time
	circuitTTL    time.Duration
	probeLeaseTTL time.Duration
}

func NewPrimaryHealthService(store PrimaryHealthStore) *PrimaryHealthService {
	return newPrimaryHealthService(store, time.Now, PrimaryCircuitOpenDuration, PrimaryProbeLeaseDuration)
}

func newPrimaryHealthService(store PrimaryHealthStore, now func() time.Time, circuitTTL, probeLeaseTTL time.Duration) *PrimaryHealthService {
	return &PrimaryHealthService{
		store:         store,
		now:           now,
		circuitTTL:    circuitTTL,
		probeLeaseTTL: probeLeaseTTL,
	}
}

// RecordFailure stores evidence for one enabled primary. The store opens the
// circuit only after every currently enabled primary has independent evidence.
func (s *PrimaryHealthService) RecordFailure(
	ctx context.Context,
	groupID int64,
	requestedModel string,
	enabledPrimaryAccountIDs []int64,
	evidence PrimaryFailureEvidence,
) (PrimaryCircuitState, error) {
	scope, err := newPrimaryHealthScope(groupID, requestedModel)
	if err != nil {
		return PrimaryCircuitState{}, err
	}

	enabledIDs := normalizePrimaryAccountIDs(enabledPrimaryAccountIDs)
	if evidence.AccountID <= 0 || !containsAccountID(enabledIDs, evidence.AccountID) {
		return PrimaryCircuitState{}, nil
	}
	if evidence.FailedAtUnixMilli == 0 {
		evidence.FailedAtUnixMilli = s.now().UnixMilli()
	}

	if err := s.store.RecordFailure(ctx, scope, evidence, enabledIDs, s.circuitTTL); err != nil {
		slog.Warn("primary_health_record_failure_failed", "group_id", groupID, "account_id", evidence.AccountID, "error", err)
		return PrimaryCircuitState{}, err
	}
	return s.circuitState(ctx, groupID, scope)
}

// CircuitState fails open: callers receive Open=false when Redis cannot be read.
func (s *PrimaryHealthService) CircuitState(ctx context.Context, groupID int64, requestedModel string) (PrimaryCircuitState, error) {
	scope, err := newPrimaryHealthScope(groupID, requestedModel)
	if err != nil {
		return PrimaryCircuitState{}, err
	}
	return s.circuitState(ctx, groupID, scope)
}

func (s *PrimaryHealthService) circuitState(ctx context.Context, groupID int64, scope PrimaryHealthScope) (PrimaryCircuitState, error) {
	state, err := s.store.GetCircuitState(ctx, scope)
	if err != nil {
		slog.Warn("primary_health_circuit_read_failed", "group_id", groupID, "error", err)
		return PrimaryCircuitState{}, err
	}
	return state, nil
}

func (s *PrimaryHealthService) FailureEvidence(ctx context.Context, groupID int64, requestedModel string) ([]PrimaryFailureEvidence, error) {
	scope, err := newPrimaryHealthScope(groupID, requestedModel)
	if err != nil {
		return nil, err
	}
	evidence, err := s.store.ListFailureEvidence(ctx, scope)
	if err != nil {
		slog.Warn("primary_health_evidence_read_failed", "group_id", groupID, "error", err)
		return nil, err
	}
	return evidence, nil
}

// TryAcquireProbeLease permits at most one probe for an account in this
// group/model scope during each lease window.
func (s *PrimaryHealthService) TryAcquireProbeLease(ctx context.Context, groupID int64, requestedModel string, accountID int64) (bool, error) {
	scope, err := newPrimaryHealthScope(groupID, requestedModel)
	if err != nil {
		return false, err
	}
	if accountID <= 0 {
		return false, nil
	}
	owner := strconv.FormatInt(s.now().UnixNano(), 10)
	acquired, err := s.store.TryAcquireProbeLease(ctx, scope, accountID, owner, s.probeLeaseTTL)
	if err != nil {
		slog.Warn("primary_health_probe_lease_failed", "group_id", groupID, "account_id", accountID, "error", err)
		return false, err
	}
	return acquired, nil
}

// RecordProbeSuccess clears an account's failure evidence after two
// consecutive successes. Recovery also closes the group/model circuit so the
// next normal request can attempt the primary pool again.
func (s *PrimaryHealthService) RecordProbeSuccess(ctx context.Context, groupID int64, requestedModel string, accountID int64) (PrimaryProbeResult, error) {
	scope, err := newPrimaryHealthScope(groupID, requestedModel)
	if err != nil {
		return PrimaryProbeResult{}, err
	}
	if accountID <= 0 {
		return PrimaryProbeResult{}, nil
	}
	result, err := s.store.RecordProbeSuccess(ctx, scope, accountID, PrimaryProbeSuccessesNeeded)
	if err != nil {
		slog.Warn("primary_health_probe_success_failed", "group_id", groupID, "account_id", accountID, "error", err)
		return PrimaryProbeResult{}, err
	}
	return result, nil
}

func newPrimaryHealthScope(groupID int64, requestedModel string) (PrimaryHealthScope, error) {
	if groupID <= 0 {
		return PrimaryHealthScope{}, fmt.Errorf("invalid group ID %d", groupID)
	}
	normalizedModel := strings.ToLower(strings.Join(strings.Fields(requestedModel), " "))
	if normalizedModel == "" {
		return PrimaryHealthScope{}, fmt.Errorf("requested model is empty")
	}
	sum := sha256.Sum256([]byte(normalizedModel))
	return PrimaryHealthScope{GroupID: groupID, ModelHash: hex.EncodeToString(sum[:])}, nil
}

func normalizePrimaryAccountIDs(accountIDs []int64) []int64 {
	unique := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID > 0 {
			unique[accountID] = struct{}{}
		}
	}
	result := make([]int64, 0, len(unique))
	for accountID := range unique {
		result = append(result, accountID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func containsAccountID(accountIDs []int64, accountID int64) bool {
	index := sort.Search(len(accountIDs), func(i int) bool { return accountIDs[i] >= accountID })
	return index < len(accountIDs) && accountIDs[index] == accountID
}
