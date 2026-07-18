package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouteAuditCountsAttemptsAndCompletesStandbyRoute(t *testing.T) {
	audit := NewRouteAudit()
	audit.RecordAttempt()
	audit.RecordAttempt()
	audit.RecordAttempt()
	audit.Complete(&AccountSelectionResult{
		Standby:     true,
		MappingRule: "claude-opus-* -> claude-sonnet-4-6",
	})

	snapshot := audit.Snapshot()
	require.Equal(t, 3, snapshot.Attempts)
	require.Equal(t, RouteModeStandby, snapshot.Mode)
	require.Equal(t, "claude-opus-* -> claude-sonnet-4-6", snapshot.MappingRule)
}

func TestRouteAuditBoundsAndSanitizesFailures(t *testing.T) {
	audit := NewRouteAudit()
	selection := &AccountSelectionResult{Account: &Account{ID: 42}, Standby: true}
	for i := 0; i < maxRouteAuditFailures+5; i++ {
		audit.RecordFailure(selection, &UpstreamFailoverError{
			StatusCode:   http.StatusBadGateway,
			ResponseBody: []byte(`{"error":"secret upstream text"}`),
		})
	}

	require.Len(t, audit.Failures, maxRouteAuditFailures)
	for _, failure := range audit.Failures {
		require.Equal(t, int64(42), failure.AccountID)
		require.True(t, failure.Standby)
		require.Equal(t, http.StatusBadGateway, failure.StatusCode)
		require.Equal(t, "upstream_5xx", failure.Kind)
	}
}

func TestStableRouteFailureKind(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: 0, want: "transport_error"},
		{status: http.StatusRequestTimeout, want: "timeout"},
		{status: http.StatusGatewayTimeout, want: "timeout"},
		{status: http.StatusTooManyRequests, want: "rate_limited"},
		{status: http.StatusBadRequest, want: "upstream_4xx"},
		{status: http.StatusServiceUnavailable, want: "upstream_5xx"},
		{status: http.StatusOK, want: "upstream_error"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, stableRouteFailureKind(tt.status))
	}
}

func TestResolveStandbyMappingRuleReturnsExplicitRule(t *testing.T) {
	mapping := map[string]string{
		"claude-*":        "claude-haiku-4-5",
		"claude-opus-*":   "claude-sonnet-4-6",
		"claude-opus-4-6": "claude-opus-4-6-thinking",
	}

	mapped, rule, matched := resolveStandbyMappingRule(mapping, "claude-opus-4-6")
	require.True(t, matched)
	require.Equal(t, "claude-opus-4-6-thinking", mapped)
	require.Equal(t, "claude-opus-4-6 -> claude-opus-4-6-thinking", rule)

	mapped, rule, matched = resolveStandbyMappingRule(mapping, "claude-opus-4-5")
	require.True(t, matched)
	require.Equal(t, "claude-sonnet-4-6", mapped)
	require.Equal(t, "claude-opus-* -> claude-sonnet-4-6", rule)
}
