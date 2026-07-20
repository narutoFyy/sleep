package service

import (
	"log/slog"
	"net/http"
	"strings"
)

const (
	RouteModePrimary = "primary"
	RouteModeStandby = "standby"

	maxRouteAuditFailures = 16
)

// RouteFailureEntry is intentionally limited to stable routing metadata.
// It must never contain request bodies, upstream messages, URLs, or credentials.
type RouteFailureEntry struct {
	AccountID  int64  `json:"account_id"`
	Standby    bool   `json:"standby"`
	StatusCode int    `json:"status_code"`
	Kind       string `json:"kind"`
}

func logRouteCompletion(component string, usageLog *UsageLog) {
	if usageLog == nil {
		return
	}
	upstreamModel := ""
	if usageLog.UpstreamModel != nil {
		upstreamModel = strings.TrimSpace(*usageLog.UpstreamModel)
	}
	slog.Info("gateway.route_completed",
		"component", component,
		"request_id", usageLog.RequestID,
		"group_id", usageLog.GroupID,
		"account_id", usageLog.AccountID,
		"upstream_model", upstreamModel,
		"route_mode", usageLog.RouteMode,
		"route_attempt_count", usageLog.RouteAttemptCount,
		"route_failure_count", len(usageLog.RouteFailures),
	)
}

// RouteAudit is persisted on the request's single successful usage record.
type RouteAudit struct {
	Mode        string
	MappingRule string
	Attempts    int
	Failures    []RouteFailureEntry
}

func NewRouteAudit() *RouteAudit {
	return &RouteAudit{Mode: RouteModePrimary}
}

func (a *RouteAudit) RecordAttempt() {
	if a == nil {
		return
	}
	a.Attempts++
}

func (a *RouteAudit) RecordFailure(selection *AccountSelectionResult, failoverErr *UpstreamFailoverError) {
	if a == nil || selection == nil || selection.Account == nil || failoverErr == nil || len(a.Failures) >= maxRouteAuditFailures {
		return
	}
	kind := normalizeRouteFailureKind(failoverErr.FailoverReason, failoverErr.StatusCode)
	a.Failures = append(a.Failures, RouteFailureEntry{
		AccountID:  selection.Account.ID,
		Standby:    selection.Standby,
		StatusCode: failoverErr.StatusCode,
		Kind:       kind,
	})
}

func (a *RouteAudit) Complete(selection *AccountSelectionResult) {
	if a == nil {
		return
	}
	a.Mode = RouteModePrimary
	a.MappingRule = ""
	if selection != nil && selection.Standby {
		a.Mode = RouteModeStandby
		a.MappingRule = strings.TrimSpace(selection.MappingRule)
	}
}

func (a *RouteAudit) Snapshot() RouteAudit {
	if a == nil {
		return RouteAudit{Mode: RouteModePrimary}
	}
	copyAudit := *a
	failureCount := len(a.Failures)
	if failureCount > maxRouteAuditFailures {
		failureCount = maxRouteAuditFailures
	}
	copyAudit.Failures = make([]RouteFailureEntry, failureCount)
	for i := 0; i < failureCount; i++ {
		copyAudit.Failures[i] = a.Failures[i]
		copyAudit.Failures[i].Kind = normalizeRouteFailureKind(copyAudit.Failures[i].Kind, copyAudit.Failures[i].StatusCode)
	}
	if copyAudit.Attempts <= 0 {
		copyAudit.Attempts = 1
	}
	if copyAudit.Mode != RouteModeStandby {
		copyAudit.Mode = RouteModePrimary
		copyAudit.MappingRule = ""
	}
	return copyAudit
}

func normalizeRouteFailureKind(kind string, statusCode int) string {
	switch strings.TrimSpace(kind) {
	case "transport_error", "timeout", "rate_limited", "upstream_4xx", "upstream_5xx", "upstream_error", "abnormal_output":
		return strings.TrimSpace(kind)
	default:
		return stableRouteFailureKind(statusCode)
	}
}

func (a RouteAudit) IsStandby() bool {
	return a.Mode == RouteModeStandby
}

func stableRouteFailureKind(statusCode int) string {
	switch {
	case statusCode == 0:
		return "transport_error"
	case statusCode == http.StatusRequestTimeout || statusCode == http.StatusGatewayTimeout:
		return "timeout"
	case statusCode == http.StatusTooManyRequests:
		return "rate_limited"
	case statusCode >= 500:
		return "upstream_5xx"
	case statusCode >= 400:
		return "upstream_4xx"
	default:
		return "upstream_error"
	}
}

func standbyBillingModel(audit RouteAudit, usageFields ChannelUsageFields, fallback string) string {
	if !audit.IsStandby() {
		return fallback
	}
	if usageFields.BillingModelSource == BillingModelSourceChannelMapped {
		if model := strings.TrimSpace(usageFields.ChannelMappedModel); model != "" {
			return model
		}
	}
	if model := strings.TrimSpace(usageFields.OriginalModel); model != "" {
		return model
	}
	return fallback
}
