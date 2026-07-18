//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayRecordUsage_StandbyBillsOriginalClientModelAndIgnoresAccountMultiplier(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
	usage := ClaudeUsage{InputTokens: 120, OutputTokens: 30}
	tokens := UsageTokens{InputTokens: 120, OutputTokens: 30}
	expected, err := svc.billingService.CalculateCost("claude-sonnet-4", tokens, 1.1)
	require.NoError(t, err)
	standbyRate := 9.0

	err = svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:     "anthropic-standby-original-model",
			Model:         "claude-opus-4",
			UpstreamModel: "claude-opus-4",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 101},
		User:    &User{ID: 201},
		Account: &Account{ID: 301, RateMultiplier: &standbyRate},
		RouteAudit: RouteAudit{
			Mode:        RouteModeStandby,
			MappingRule: "claude-sonnet-4 -> claude-opus-4",
			Attempts:    2,
		},
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      "claude-sonnet-4",
			ChannelMappedModel: "claude-sonnet-4",
			BillingModelSource: BillingModelSourceRequested,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, expected.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, expected.ActualCost, userRepo.lastAmount, 1e-12)
	require.NotNil(t, usageRepo.lastLog.AccountRateMultiplier)
	require.Equal(t, standbyRate, *usageRepo.lastLog.AccountRateMultiplier)
	require.Equal(t, RouteModeStandby, usageRepo.lastLog.RouteMode)
	require.Equal(t, 2, usageRepo.lastLog.RouteAttemptCount)
}

func TestGatewayRecordUsage_StandbyBillsOriginalChannelMappedModel(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	usage := ClaudeUsage{InputTokens: 80, OutputTokens: 20}
	expected, err := svc.billingService.CalculateCost("claude-sonnet-4", UsageTokens{InputTokens: 80, OutputTokens: 20}, 1.1)
	require.NoError(t, err)

	err = svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:     "anthropic-standby-channel-model",
			Model:         "claude-opus-4",
			UpstreamModel: "claude-opus-4",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:     &APIKey{ID: 102},
		User:       &User{ID: 202},
		Account:    &Account{ID: 302},
		RouteAudit: RouteAudit{Mode: RouteModeStandby, Attempts: 1},
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      "customer-claude-alias",
			ChannelMappedModel: "claude-sonnet-4",
			BillingModelSource: BillingModelSourceChannelMapped,
		},
	})

	require.NoError(t, err)
	require.InDelta(t, expected.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
}

func TestGatewayRecordUsage_PrimaryBillingRemainsUnchanged(t *testing.T) {
	usage := ClaudeUsage{InputTokens: 90, OutputTokens: 25}
	run := func(audit RouteAudit) float64 {
		repo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result:     &ForwardResult{RequestID: "anthropic-primary", Model: "claude-opus-4", Usage: usage, Duration: time.Second},
			APIKey:     &APIKey{ID: 103},
			User:       &User{ID: 203},
			Account:    &Account{ID: 303},
			RouteAudit: audit,
			ChannelUsageFields: ChannelUsageFields{
				OriginalModel:      "claude-sonnet-4",
				ChannelMappedModel: "claude-sonnet-4",
				BillingModelSource: BillingModelSourceUpstream,
			},
		})
		require.NoError(t, err)
		return repo.lastLog.ActualCost
	}

	require.InDelta(t, run(RouteAudit{}), run(RouteAudit{Mode: RouteModePrimary, Attempts: 1}), 1e-12)
}

func TestOpenAIRecordUsage_StandbyBillsOriginalClientModelAndIgnoresAccountMultiplier(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	usage := OpenAIUsage{InputTokens: 120, OutputTokens: 30}
	expected := expectedOpenAICost(t, svc, "gpt-5.1", usage, 1.1)
	standbyRate := 7.0

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "openai-standby-original-model",
			Model:         "gpt-5.1",
			BillingModel:  "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 104},
		User:    &User{ID: 204},
		Account: &Account{ID: 304, RateMultiplier: &standbyRate},
		RouteAudit: RouteAudit{
			Mode:        RouteModeStandby,
			MappingRule: "gpt-5.1 -> gpt-5.4",
			Attempts:    3,
		},
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      "gpt-5.1",
			ChannelMappedModel: "gpt-5.1",
			BillingModelSource: BillingModelSourceRequested,
		},
	})

	require.NoError(t, err)
	require.InDelta(t, expected.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, expected.ActualCost, userRepo.lastAmount, 1e-12)
	require.NotNil(t, usageRepo.lastLog.AccountRateMultiplier)
	require.Equal(t, standbyRate, *usageRepo.lastLog.AccountRateMultiplier)
	require.Equal(t, RouteModeStandby, usageRepo.lastLog.RouteMode)
	require.Equal(t, 3, usageRepo.lastLog.RouteAttemptCount)
}

func TestOpenAIRecordUsage_StandbyBillsOriginalChannelMappedModel(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	usage := OpenAIUsage{InputTokens: 80, OutputTokens: 20}
	expected := expectedOpenAICost(t, svc, "gpt-5.1", usage, 1.1)

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "openai-standby-channel-model",
			Model:         "customer-gpt-alias",
			BillingModel:  "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:     &APIKey{ID: 105},
		User:       &User{ID: 205},
		Account:    &Account{ID: 305},
		RouteAudit: RouteAudit{Mode: RouteModeStandby, Attempts: 1},
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      "customer-gpt-alias",
			ChannelMappedModel: "gpt-5.1",
			BillingModelSource: BillingModelSourceChannelMapped,
		},
	})

	require.NoError(t, err)
	require.InDelta(t, expected.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
}

func TestOpenAIRecordUsage_PrimaryBillingRemainsUnchanged(t *testing.T) {
	usage := OpenAIUsage{InputTokens: 90, OutputTokens: 25}
	run := func(audit RouteAudit) float64 {
		repo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{
				RequestID: "openai-primary", Model: "gpt-5.1", BillingModel: "gpt-5.4", Usage: usage, Duration: time.Second,
			},
			APIKey:     &APIKey{ID: 106},
			User:       &User{ID: 206},
			Account:    &Account{ID: 306},
			RouteAudit: audit,
		})
		require.NoError(t, err)
		return repo.lastLog.ActualCost
	}

	require.InDelta(t, run(RouteAudit{}), run(RouteAudit{Mode: RouteModePrimary, Attempts: 1}), 1e-12)
}
