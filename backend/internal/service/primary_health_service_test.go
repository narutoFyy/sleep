//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type primaryHealthStoreStub struct {
	recordScope    PrimaryHealthScope
	recordEvidence PrimaryFailureEvidence
	recordIDs      []int64
	recordTTL      time.Duration
	recordErr      error
	circuitState   PrimaryCircuitState
	circuitErr     error
	leaseTTL       time.Duration
	leaseResult    bool
	leaseErr       error
	probeResult    PrimaryProbeResult
	probeErr       error
	probeThreshold int
}

func (s *primaryHealthStoreStub) RecordFailure(_ context.Context, scope PrimaryHealthScope, evidence PrimaryFailureEvidence, ids []int64, ttl time.Duration) error {
	s.recordScope = scope
	s.recordEvidence = evidence
	s.recordIDs = append([]int64(nil), ids...)
	s.recordTTL = ttl
	return s.recordErr
}

func (s *primaryHealthStoreStub) GetCircuitState(context.Context, PrimaryHealthScope) (PrimaryCircuitState, error) {
	return s.circuitState, s.circuitErr
}

func (s *primaryHealthStoreStub) ListFailureEvidence(context.Context, PrimaryHealthScope) ([]PrimaryFailureEvidence, error) {
	return nil, nil
}

func (s *primaryHealthStoreStub) TryAcquireProbeLease(_ context.Context, _ PrimaryHealthScope, _ int64, _ string, ttl time.Duration) (bool, error) {
	s.leaseTTL = ttl
	return s.leaseResult, s.leaseErr
}

func (s *primaryHealthStoreStub) RecordProbeSuccess(_ context.Context, _ PrimaryHealthScope, _ int64, successesNeeded int) (PrimaryProbeResult, error) {
	s.probeThreshold = successesNeeded
	return s.probeResult, s.probeErr
}

func TestPrimaryHealthService_NormalizesScopeIDsAndDefaults(t *testing.T) {
	store := &primaryHealthStoreStub{circuitState: PrimaryCircuitState{Open: true, Remaining: 30 * time.Second}}
	now := time.Date(2026, 7, 18, 1, 2, 3, 456000000, time.UTC)
	svc := newPrimaryHealthService(store, func() time.Time { return now }, PrimaryCircuitOpenDuration, PrimaryProbeLeaseDuration)

	state, err := svc.RecordFailure(context.Background(), 44, "  CLAUDE-SONNET-4   ", []int64{3, 0, 2, 3, -1}, PrimaryFailureEvidence{
		AccountID:  3,
		RequestID:  "request-1",
		AttemptID:  "attempt-2",
		Reason:     "upstream unavailable",
		StatusCode: 503,
	})
	require.NoError(t, err)
	require.True(t, state.Open)
	require.Equal(t, []int64{2, 3}, store.recordIDs)
	require.Equal(t, PrimaryCircuitOpenDuration, store.recordTTL)
	require.Equal(t, now.UnixMilli(), store.recordEvidence.FailedAtUnixMilli)
	require.Len(t, store.recordScope.ModelHash, 64)
	require.NotContains(t, store.recordScope.ModelHash, "claude")

	otherScope, err := newPrimaryHealthScope(44, "claude-sonnet-4")
	require.NoError(t, err)
	require.Equal(t, store.recordScope, otherScope)

	store.leaseResult = true
	acquired, err := svc.TryAcquireProbeLease(context.Background(), 44, "claude-sonnet-4", 3)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, PrimaryProbeLeaseDuration, store.leaseTTL)

	_, err = svc.RecordProbeSuccess(context.Background(), 44, "claude-sonnet-4", 3)
	require.NoError(t, err)
	require.Equal(t, PrimaryProbeSuccessesNeeded, store.probeThreshold)
}

func TestPrimaryHealthService_IgnoresFailureOutsideEnabledPrimaries(t *testing.T) {
	store := &primaryHealthStoreStub{}
	svc := NewPrimaryHealthService(store)

	state, err := svc.RecordFailure(context.Background(), 44, "claude-sonnet-4", []int64{1, 2}, PrimaryFailureEvidence{AccountID: 3})
	require.NoError(t, err)
	require.False(t, state.Open)
	require.Empty(t, store.recordIDs)
}

func TestPrimaryHealthService_RedisErrorsFailOpen(t *testing.T) {
	redisErr := errors.New("redis unavailable")
	store := &primaryHealthStoreStub{recordErr: redisErr, circuitErr: redisErr, leaseErr: redisErr, probeErr: redisErr}
	svc := NewPrimaryHealthService(store)

	state, err := svc.RecordFailure(context.Background(), 44, "claude-sonnet-4", []int64{1}, PrimaryFailureEvidence{AccountID: 1})
	require.ErrorIs(t, err, redisErr)
	require.False(t, state.Open)

	state, err = svc.CircuitState(context.Background(), 44, "claude-sonnet-4")
	require.ErrorIs(t, err, redisErr)
	require.False(t, state.Open)

	acquired, err := svc.TryAcquireProbeLease(context.Background(), 44, "claude-sonnet-4", 1)
	require.ErrorIs(t, err, redisErr)
	require.False(t, acquired)

	probe, err := svc.RecordProbeSuccess(context.Background(), 44, "claude-sonnet-4", 1)
	require.ErrorIs(t, err, redisErr)
	require.False(t, probe.Recovered)
}
