package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type standbyRoutingHealthStore struct {
	circuit    PrimaryCircuitState
	evidence   []PrimaryFailureEvidence
	leaseByID  map[int64]bool
	readErr    error
	leaseErr   error
	probeCalls []int64
}

func (s *standbyRoutingHealthStore) RecordFailure(context.Context, PrimaryHealthScope, PrimaryFailureEvidence, []int64, time.Duration) error {
	return nil
}

func (s *standbyRoutingHealthStore) GetCircuitState(context.Context, PrimaryHealthScope) (PrimaryCircuitState, error) {
	return s.circuit, s.readErr
}

func (s *standbyRoutingHealthStore) ListFailureEvidence(context.Context, PrimaryHealthScope) ([]PrimaryFailureEvidence, error) {
	return append([]PrimaryFailureEvidence(nil), s.evidence...), s.readErr
}

func (s *standbyRoutingHealthStore) TryAcquireProbeLease(_ context.Context, _ PrimaryHealthScope, accountID int64, _ string, _ time.Duration) (bool, error) {
	s.probeCalls = append(s.probeCalls, accountID)
	return s.leaseByID[accountID], s.leaseErr
}

func (s *standbyRoutingHealthStore) RecordProbeSuccess(context.Context, PrimaryHealthScope, int64, int) (PrimaryProbeResult, error) {
	return PrimaryProbeResult{}, nil
}

type standbyRoutingAccountRepo struct {
	AccountRepository
	accounts []Account
}

type standbyRoutingGroupRepo struct {
	GroupRepository
	group *Group
}

func (r *standbyRoutingGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	if r.group != nil && r.group.ID == id {
		return r.group, nil
	}
	return nil, errors.New("group not found")
}

func (r *standbyRoutingAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, errors.New("account not found")
}

func (r *standbyRoutingAccountRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

type standbyRoutingGatewayCache struct {
	setCalls int
}

func (c *standbyRoutingGatewayCache) GetSessionAccountID(context.Context, int64, string) (int64, error) {
	return 0, nil
}

func (c *standbyRoutingGatewayCache) SetSessionAccountID(context.Context, int64, string, int64, time.Duration) error {
	c.setCalls++
	return nil
}

func (c *standbyRoutingGatewayCache) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}

func (c *standbyRoutingGatewayCache) DeleteSessionAccountID(context.Context, int64, string) error {
	return nil
}

func TestEnabledPrimaryMembership_LegacyAndExplicitStates(t *testing.T) {
	groupID := int64(42)
	tests := []struct {
		name    string
		account Account
		want    bool
	}{
		{
			name: "legacy blank role ignores enabled zero value",
			account: Account{AccountGroups: []AccountGroup{{
				GroupID: groupID,
			}}},
			want: true,
		},
		{
			name: "legacy group ids remain primary",
			account: Account{
				GroupIDs: []int64{groupID},
			},
			want: true,
		},
		{
			name: "explicit enabled primary",
			account: Account{AccountGroups: []AccountGroup{{
				GroupID: groupID,
				Role:    AccountGroupRolePrimary,
				Enabled: true,
			}}},
			want: true,
		},
		{
			name: "explicit disabled primary",
			account: Account{AccountGroups: []AccountGroup{{
				GroupID: groupID,
				Role:    AccountGroupRolePrimary,
				Enabled: false,
			}}},
			want: false,
		},
		{
			name: "standby is not primary",
			account: Account{AccountGroups: []AccountGroup{{
				GroupID: groupID,
				Role:    AccountGroupRoleStandby,
				Enabled: true,
			}}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, enabledPrimaryMembership(&tt.account, &groupID))
		})
	}
}

func TestFilterEnabledPrimaryAccounts_GroupScopedLegacyPayload(t *testing.T) {
	groupID := int64(42)
	accounts := []Account{
		{ID: 1},
		{ID: 2, AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRoleStandby, Enabled: true}}},
		{ID: 3, AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRolePrimary, Enabled: false}}},
		{ID: 4, AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRolePrimary, Enabled: true}}},
	}

	filtered := filterEnabledPrimaryAccounts(accounts, &groupID)
	require.Equal(t, []int64{1, 4}, []int64{filtered[0].ID, filtered[1].ID})
}

func TestPreparePrimaryHealth_CircuitProbeAndFailOpen(t *testing.T) {
	groupID := int64(42)
	accounts := []Account{
		{ID: 1, AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRolePrimary, Enabled: true}}},
		{ID: 2, AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRolePrimary, Enabled: true}}},
		{ID: 3, AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRolePrimary, Enabled: false}}},
	}

	store := &standbyRoutingHealthStore{circuit: PrimaryCircuitState{Open: true}}
	prepared := preparePrimaryHealth(context.Background(), NewPrimaryHealthService(store), &groupID, "claude-sonnet-4", accounts, nil)
	require.True(t, prepared.circuitOpen)
	require.Equal(t, []int64{1, 2}, prepared.primaryAccountIDs)

	store.circuit = PrimaryCircuitState{}
	store.evidence = []PrimaryFailureEvidence{{AccountID: 1}, {AccountID: 2}, {AccountID: 3}}
	store.leaseByID = map[int64]bool{1: true, 2: false}
	prepared = preparePrimaryHealth(context.Background(), NewPrimaryHealthService(store), &groupID, "claude-sonnet-4", accounts, nil)
	require.False(t, prepared.circuitOpen)
	require.Contains(t, prepared.probeAccountIDs, int64(1))
	require.Contains(t, prepared.excluded, int64(2))
	require.NotContains(t, prepared.excluded, int64(1))
	require.NotContains(t, prepared.excluded, int64(3))

	store.readErr = errors.New("redis unavailable")
	prepared = preparePrimaryHealth(context.Background(), NewPrimaryHealthService(store), &groupID, "claude-sonnet-4", accounts, map[int64]struct{}{9: {}})
	require.False(t, prepared.circuitOpen)
	require.Equal(t, map[int64]struct{}{9: {}}, prepared.excluded)
	require.Empty(t, prepared.probeAccountIDs)
}

func TestListStandbyCandidates_RequiresRoleProviderAndExplicitMapping(t *testing.T) {
	groupID := int64(42)
	repo := &standbyRoutingAccountRepo{accounts: []Account{
		{
			ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Priority: 10,
			AccountGroups: []AccountGroup{{GroupID: groupID, Priority: 2, Role: AccountGroupRoleStandby, Enabled: true, ModelMapping: map[string]string{"claude-sonnet-4": "claude-sonnet-4-20250514"}}},
		},
		{
			ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Priority: 20,
			AccountGroups: []AccountGroup{{GroupID: groupID, Priority: 1, Role: AccountGroupRoleStandby, Enabled: true, ModelMapping: map[string]string{"claude-*": "claude-fallback"}}},
		},
		{
			ID: 3, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRoleStandby, Enabled: true, ModelMapping: map[string]string{"claude-*": "gpt-5"}}},
		},
		{
			ID: 4, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRoleStandby, Enabled: false, ModelMapping: map[string]string{"claude-*": "disabled"}}},
		},
		{
			ID: 5, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRolePrimary, Enabled: true, ModelMapping: map[string]string{"claude-*": "not-standby"}}},
		},
		{
			ID: 6, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: groupID, Role: AccountGroupRoleStandby, Enabled: true}},
		},
	}}

	candidates, err := listStandbyCandidates(context.Background(), repo, groupID, PlatformAnthropic, "claude-sonnet-4")
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, int64(2), candidates[0].account.ID)
	require.Equal(t, "claude-fallback", candidates[0].upstreamModel)
	require.Equal(t, int64(1), candidates[1].account.ID)
	require.Equal(t, "claude-sonnet-4-20250514", candidates[1].upstreamModel)
}

func TestOpenAIStandbySelector_ReturnsMappedModelWithoutStickyWrite(t *testing.T) {
	groupID := int64(42)
	repo := &standbyRoutingAccountRepo{accounts: []Account{{
		ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 5,
		AccountGroups: []AccountGroup{{
			GroupID: groupID, Priority: 1, Role: AccountGroupRoleStandby, Enabled: true,
			ModelMapping: map[string]string{"gpt-5*": "gpt-5.4"},
		}},
	}}}
	cache := &standbyRoutingGatewayCache{}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: cache}

	selection, decision, err := svc.SelectStandbyAccountForCapability(
		context.Background(),
		&groupID,
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportHTTPSSE,
		OpenAIEndpointCapabilityChatCompletions,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.True(t, selection.Standby)
	require.True(t, selection.Acquired)
	require.Equal(t, int64(11), selection.Account.ID)
	require.Equal(t, "gpt-5.4", selection.UpstreamModel)
	require.Equal(t, "gpt-5* -> gpt-5.4", selection.MappingRule)
	require.Equal(t, "standby", decision.Layer)
	require.Zero(t, cache.setCalls)
	selection.ReleaseFunc()
}

func TestAnthropicStandbySelector_ReturnsMappedModelWithoutStickyWrite(t *testing.T) {
	groupID := int64(42)
	repo := &standbyRoutingAccountRepo{accounts: []Account{{
		ID: 21, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 5,
		AccountGroups: []AccountGroup{{
			GroupID: groupID, Priority: 1, Role: AccountGroupRoleStandby, Enabled: true,
			ModelMapping: map[string]string{"claude-sonnet-*": "claude-sonnet-4-20250514"},
		}},
	}}}
	cache := &standbyRoutingGatewayCache{}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo:   &standbyRoutingGroupRepo{group: &Group{ID: groupID, Platform: PlatformAnthropic}},
		cache:       cache,
	}

	selection, err := svc.SelectStandbyAccount(context.Background(), &groupID, "claude-sonnet-4", nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.True(t, selection.Standby)
	require.True(t, selection.Acquired)
	require.Equal(t, int64(21), selection.Account.ID)
	require.Equal(t, "claude-sonnet-4-20250514", selection.UpstreamModel)
	require.Equal(t, "claude-sonnet-* -> claude-sonnet-4-20250514", selection.MappingRule)
	require.Zero(t, cache.setCalls)
	selection.ReleaseFunc()
}

func TestStandbyClientModelContext_PreservesOriginalModel(t *testing.T) {
	ctx := WithStandbyClientModel(context.Background(), "claude-sonnet-4")
	require.Equal(t, "claude-sonnet-4", clientModelFromContext(ctx, "claude-fallback"))
	require.Equal(t, "claude-fallback", clientModelFromContext(context.Background(), "claude-fallback"))
}
