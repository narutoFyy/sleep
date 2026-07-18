package service

import (
	"context"
	"errors"
	"sort"
	"strings"
)

var ErrPrimaryCircuitOpen = errors.New("primary group circuit is open")

type standbyClientModelContextKey struct{}

// WithStandbyClientModel preserves the client-visible model while the request
// body carries the explicitly mapped standby model.
func WithStandbyClientModel(ctx context.Context, clientModel string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	clientModel = strings.TrimSpace(clientModel)
	if clientModel == "" {
		return ctx
	}
	return context.WithValue(ctx, standbyClientModelContextKey{}, clientModel)
}

func clientModelFromContext(ctx context.Context, fallback string) string {
	if ctx != nil {
		if model, ok := ctx.Value(standbyClientModelContextKey{}).(string); ok && strings.TrimSpace(model) != "" {
			return strings.TrimSpace(model)
		}
	}
	return fallback
}

func enabledPrimaryMembership(account *Account, groupID *int64) bool {
	if account == nil {
		return false
	}
	if groupID == nil {
		return len(account.AccountGroups) == 0 && len(account.GroupIDs) == 0
	}

	for _, membership := range account.AccountGroups {
		if membership.GroupID != *groupID {
			continue
		}
		// Empty role is a legacy in-memory/snapshot membership. Its Enabled
		// zero value predates the field and must remain routable.
		if membership.Role == "" {
			return true
		}
		return membership.Role == AccountGroupRolePrimary && membership.Enabled
	}
	for _, legacyGroupID := range account.GroupIDs {
		if legacyGroupID == *groupID {
			return true
		}
	}
	return false
}

func enabledStandbyMembership(account *Account, groupID int64) (AccountGroup, bool) {
	if account == nil || groupID <= 0 {
		return AccountGroup{}, false
	}
	for _, membership := range account.AccountGroups {
		if membership.GroupID == groupID && membership.Role == AccountGroupRoleStandby && membership.Enabled {
			return membership, true
		}
	}
	return AccountGroup{}, false
}

func filterEnabledPrimaryAccounts(accounts []Account, groupID *int64) []Account {
	if len(accounts) == 0 {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		// Group-scoped repositories and old scheduler snapshots may omit the
		// membership payload after already enforcing the group in their query.
		if groupID != nil && len(accounts[i].AccountGroups) == 0 && len(accounts[i].GroupIDs) == 0 {
			filtered = append(filtered, accounts[i])
			continue
		}
		if enabledPrimaryMembership(&accounts[i], groupID) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}

type primaryHealthPreparation struct {
	excluded          map[int64]struct{}
	probeAccountIDs   map[int64]struct{}
	primaryAccountIDs []int64
	circuitOpen       bool
}

func preparePrimaryHealth(
	ctx context.Context,
	health *PrimaryHealthService,
	groupID *int64,
	requestedModel string,
	accounts []Account,
	excluded map[int64]struct{},
) primaryHealthPreparation {
	prepared := primaryHealthPreparation{excluded: cloneExcludedAccountIDs(excluded)}
	if health == nil || groupID == nil || *groupID <= 0 || strings.TrimSpace(requestedModel) == "" {
		return prepared
	}

	primarySet := make(map[int64]struct{}, len(accounts))
	for i := range accounts {
		if accounts[i].ID <= 0 || !enabledPrimaryMembership(&accounts[i], groupID) {
			continue
		}
		primarySet[accounts[i].ID] = struct{}{}
		prepared.primaryAccountIDs = append(prepared.primaryAccountIDs, accounts[i].ID)
	}
	sort.Slice(prepared.primaryAccountIDs, func(i, j int) bool { return prepared.primaryAccountIDs[i] < prepared.primaryAccountIDs[j] })

	state, err := health.CircuitState(ctx, *groupID, requestedModel)
	if err != nil {
		return prepared // Redis failures fail open.
	}
	if state.Open {
		prepared.circuitOpen = true
		return prepared
	}

	evidence, err := health.FailureEvidence(ctx, *groupID, requestedModel)
	if err != nil {
		return prepared
	}
	if len(evidence) == 0 {
		return prepared
	}

	effectiveExcluded := cloneExcludedAccountIDs(excluded)
	probeIDs := make(map[int64]struct{})
	for _, failure := range evidence {
		if _, currentPrimary := primarySet[failure.AccountID]; !currentPrimary {
			continue
		}
		if _, alreadyExcluded := effectiveExcluded[failure.AccountID]; alreadyExcluded {
			continue
		}
		acquired, leaseErr := health.TryAcquireProbeLease(ctx, *groupID, requestedModel, failure.AccountID)
		if leaseErr != nil {
			return prepared // Discard partial decisions and fail open.
		}
		if acquired {
			probeIDs[failure.AccountID] = struct{}{}
			continue
		}
		if effectiveExcluded == nil {
			effectiveExcluded = make(map[int64]struct{})
		}
		effectiveExcluded[failure.AccountID] = struct{}{}
	}
	prepared.excluded = effectiveExcluded
	prepared.probeAccountIDs = probeIDs
	return prepared
}

func annotatePrimarySelection(selection *AccountSelectionResult, prepared primaryHealthPreparation) {
	if selection == nil || selection.Account == nil {
		return
	}
	selection.PrimaryAccountIDs = append([]int64(nil), prepared.primaryAccountIDs...)
	_, selection.PrimaryProbe = prepared.probeAccountIDs[selection.Account.ID]
}

func (s *GatewayService) SelectAccountWithLoadAwareness(ctx context.Context, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}, metadataUserID string, sub2apiUserID int64) (*AccountSelectionResult, error) {
	if s.primaryHealthService == nil || groupID == nil {
		return s.selectPrimaryAccountWithLoadAwareness(ctx, groupID, sessionHash, requestedModel, excludedIDs, metadataUserID, sub2apiUserID)
	}
	group, err := s.resolveGroupByID(ctx, *groupID)
	if err != nil || group == nil || group.Platform != PlatformAnthropic {
		return s.selectPrimaryAccountWithLoadAwareness(ctx, groupID, sessionHash, requestedModel, excludedIDs, metadataUserID, sub2apiUserID)
	}
	accounts, _, listErr := s.listSchedulableAccounts(ctx, groupID, PlatformAnthropic, false)
	if listErr != nil {
		return nil, listErr
	}
	prepared := preparePrimaryHealth(ctx, s.primaryHealthService, groupID, requestedModel, accounts, excludedIDs)
	if prepared.circuitOpen {
		return nil, ErrPrimaryCircuitOpen
	}
	selection, selectErr := s.selectPrimaryAccountWithLoadAwareness(ctx, groupID, sessionHash, requestedModel, prepared.excluded, metadataUserID, sub2apiUserID)
	annotatePrimarySelection(selection, prepared)
	return selection, selectErr
}

func (s *OpenAIGatewayService) selectAccountWithScheduler(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requiredImageCapability OpenAIImagesCapability,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if s.primaryHealthService == nil || groupID == nil || requiredImageCapability != "" ||
		(requiredTransport != OpenAIUpstreamTransportAny && requiredTransport != OpenAIUpstreamTransportHTTPSSE) {
		return s.selectPrimaryAccountWithScheduler(ctx, groupID, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, requiredCapability, requiredImageCapability, requireCompact)
	}
	accounts, err := s.listSchedulableAccounts(ctx, groupID)
	if err != nil {
		return nil, OpenAIAccountScheduleDecision{}, err
	}
	prepared := preparePrimaryHealth(ctx, s.primaryHealthService, groupID, requestedModel, accounts, excludedIDs)
	if prepared.circuitOpen {
		return nil, OpenAIAccountScheduleDecision{}, ErrPrimaryCircuitOpen
	}
	selection, decision, selectErr := s.selectPrimaryAccountWithScheduler(ctx, groupID, previousResponseID, sessionHash, requestedModel, prepared.excluded, requiredTransport, requiredCapability, requiredImageCapability, requireCompact)
	annotatePrimarySelection(selection, prepared)
	return selection, decision, selectErr
}

type standbyCandidate struct {
	account       *Account
	membership    AccountGroup
	upstreamModel string
	mappingRule   string
}

func resolveStandbyMappingRule(mapping map[string]string, requestedModel string) (mappedModel, rule string, matched bool) {
	if target, ok := mapping[requestedModel]; ok {
		return target, requestedModel + " -> " + target, true
	}
	type matchedRule struct {
		pattern string
		target  string
	}
	matches := make([]matchedRule, 0)
	for pattern, target := range mapping {
		if matchWildcard(pattern, requestedModel) {
			matches = append(matches, matchedRule{pattern: pattern, target: target})
		}
	}
	if len(matches) == 0 {
		return requestedModel, "", false
	}
	sort.Slice(matches, func(i, j int) bool {
		if len(matches[i].pattern) != len(matches[j].pattern) {
			return len(matches[i].pattern) > len(matches[j].pattern)
		}
		return matches[i].pattern < matches[j].pattern
	})
	selected := matches[0]
	return selected.target, selected.pattern + " -> " + selected.target, true
}

func listStandbyCandidates(ctx context.Context, repo AccountRepository, groupID int64, provider, requestedModel string) ([]standbyCandidate, error) {
	accounts, err := repo.ListSchedulableByGroupIDAndPlatform(ctx, groupID, provider)
	if err != nil {
		return nil, err
	}
	candidates := make([]standbyCandidate, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		membership, ok := enabledStandbyMembership(account, groupID)
		if !ok || account.Platform != provider || !account.IsSchedulable() {
			continue
		}
		mappedModel, mappingRule, matched := resolveStandbyMappingRule(membership.ModelMapping, requestedModel)
		mappedModel = strings.TrimSpace(mappedModel)
		if !matched || mappedModel == "" {
			continue
		}
		candidates = append(candidates, standbyCandidate{account: account, membership: membership, upstreamModel: mappedModel, mappingRule: mappingRule})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].membership.Priority != candidates[j].membership.Priority {
			return candidates[i].membership.Priority < candidates[j].membership.Priority
		}
		if candidates[i].account.Priority != candidates[j].account.Priority {
			return candidates[i].account.Priority < candidates[j].account.Priority
		}
		left, right := candidates[i].account.LastUsedAt, candidates[j].account.LastUsedAt
		if left == nil || right == nil {
			return left == nil && right != nil
		}
		return left.Before(*right)
	})
	return candidates, nil
}

func (s *GatewayService) SelectStandbyAccount(ctx context.Context, groupID *int64, requestedModel string, excludedIDs map[int64]struct{}) (*AccountSelectionResult, error) {
	if groupID == nil || *groupID <= 0 {
		return nil, ErrNoAvailableAccounts
	}
	group, err := s.resolveGroupByID(ctx, *groupID)
	if err != nil {
		return nil, err
	}
	if group == nil || group.Platform != PlatformAnthropic {
		return nil, ErrNoAvailableAccounts
	}
	candidates, err := listStandbyCandidates(ctx, s.accountRepo, *groupID, group.Platform, requestedModel)
	if err != nil {
		return nil, err
	}
	var waitCandidate *standbyCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if _, excluded := excludedIDs[candidate.account.ID]; excluded {
			continue
		}
		if waitCandidate == nil {
			waitCandidate = candidate
		}
		acquired, acquireErr := s.tryAcquireAccountSlot(ctx, candidate.account.ID, candidate.account.Concurrency)
		if acquireErr != nil {
			return nil, acquireErr
		}
		if acquired == nil || !acquired.Acquired {
			continue
		}
		selection, hydrateErr := s.newSelectionResult(ctx, candidate.account, true, acquired.ReleaseFunc, nil)
		if hydrateErr != nil {
			acquired.ReleaseFunc()
			continue
		}
		latestMembership, valid := enabledStandbyMembership(selection.Account, *groupID)
		latestMappedModel, latestMappingRule, latestMatched := resolveStandbyMappingRule(latestMembership.ModelMapping, requestedModel)
		latestMappedModel = strings.TrimSpace(latestMappedModel)
		if !valid || !latestMatched || latestMappedModel == "" || selection.Account.Platform != group.Platform || !selection.Account.IsSchedulable() {
			selection.ReleaseFunc()
			continue
		}
		selection.Standby = true
		selection.UpstreamModel = latestMappedModel
		selection.MappingRule = latestMappingRule
		return selection, nil
	}
	if waitCandidate != nil {
		cfg := s.schedulingConfig()
		return &AccountSelectionResult{
			Account:       waitCandidate.account,
			Standby:       true,
			UpstreamModel: waitCandidate.upstreamModel,
			MappingRule:   waitCandidate.mappingRule,
			WaitPlan: &AccountWaitPlan{
				AccountID:      waitCandidate.account.ID,
				MaxConcurrency: waitCandidate.account.Concurrency,
				Timeout:        cfg.FallbackWaitTimeout,
				MaxWaiting:     cfg.FallbackMaxWaiting,
			},
		}, nil
	}
	return nil, ErrNoAvailableAccounts
}

func (s *OpenAIGatewayService) SelectStandbyAccountForCapability(
	ctx context.Context,
	groupID *int64,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	decision := OpenAIAccountScheduleDecision{Layer: "standby"}
	if groupID == nil || *groupID <= 0 {
		return nil, decision, ErrNoAvailableAccounts
	}
	candidates, err := listStandbyCandidates(ctx, s.accountRepo, *groupID, PlatformOpenAI, requestedModel)
	if err != nil {
		return nil, decision, err
	}
	var waitCandidate *standbyCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if _, excluded := excludedIDs[candidate.account.ID]; excluded {
			continue
		}
		account := candidate.account
		if !accountSupportsOpenAICapabilities(account, requiredCapability, "") ||
			!s.isOpenAIAccountTransportCompatible(account, requiredTransport) ||
			s.isOpenAIAccountRuntimeBlocked(account) || (requireCompact && !account.AllowsOpenAICompact()) {
			continue
		}
		if waitCandidate == nil {
			waitCandidate = candidate
		}
		acquired, acquireErr := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
		if acquireErr != nil {
			return nil, decision, acquireErr
		}
		if acquired == nil || !acquired.Acquired {
			continue
		}
		latest, latestErr := s.accountRepo.GetByID(ctx, account.ID)
		if latestErr != nil || latest == nil || !latest.IsSchedulable() || latest.Platform != PlatformOpenAI ||
			!accountSupportsOpenAICapabilities(latest, requiredCapability, "") ||
			!s.isOpenAIAccountTransportCompatible(latest, requiredTransport) ||
			s.isOpenAIAccountRuntimeBlocked(latest) || (requireCompact && !latest.AllowsOpenAICompact()) {
			acquired.ReleaseFunc()
			continue
		}
		latestMembership, valid := enabledStandbyMembership(latest, *groupID)
		latestMappedModel, latestMappingRule, latestMatched := resolveStandbyMappingRule(latestMembership.ModelMapping, requestedModel)
		latestMappedModel = strings.TrimSpace(latestMappedModel)
		if !valid || !latestMatched || latestMappedModel == "" {
			acquired.ReleaseFunc()
			continue
		}
		selection := &AccountSelectionResult{
			Account:       latest,
			Acquired:      true,
			ReleaseFunc:   acquired.ReleaseFunc,
			Standby:       true,
			UpstreamModel: latestMappedModel,
			MappingRule:   latestMappingRule,
		}
		decision.SelectedAccountID = latest.ID
		decision.SelectedAccountType = latest.Type
		decision.CandidateCount = len(candidates)
		return selection, decision, nil
	}
	if waitCandidate != nil {
		cfg := s.schedulingConfig()
		selection := &AccountSelectionResult{
			Account:       waitCandidate.account,
			Standby:       true,
			UpstreamModel: waitCandidate.upstreamModel,
			MappingRule:   waitCandidate.mappingRule,
			WaitPlan: &AccountWaitPlan{
				AccountID:      waitCandidate.account.ID,
				MaxConcurrency: waitCandidate.account.Concurrency,
				Timeout:        cfg.FallbackWaitTimeout,
				MaxWaiting:     cfg.FallbackMaxWaiting,
			},
		}
		decision.SelectedAccountID = waitCandidate.account.ID
		decision.SelectedAccountType = waitCandidate.account.Type
		decision.CandidateCount = len(candidates)
		return selection, decision, nil
	}
	return nil, decision, ErrNoAvailableAccounts
}

func recordPrimaryFailure(ctx context.Context, health *PrimaryHealthService, groupID *int64, requestedModel string, selection *AccountSelectionResult, failoverErr *UpstreamFailoverError) {
	if health == nil || groupID == nil || selection == nil || selection.Account == nil || selection.Standby || failoverErr == nil {
		return
	}
	_, _ = health.RecordFailure(ctx, *groupID, requestedModel, selection.PrimaryAccountIDs, PrimaryFailureEvidence{
		AccountID:  selection.Account.ID,
		Reason:     failoverErr.Error(),
		StatusCode: failoverErr.StatusCode,
	})
}

func recordPrimaryProbeSuccess(ctx context.Context, health *PrimaryHealthService, groupID *int64, requestedModel string, selection *AccountSelectionResult) {
	if health == nil || groupID == nil || selection == nil || selection.Account == nil || selection.Standby || !selection.PrimaryProbe {
		return
	}
	_, _ = health.RecordProbeSuccess(ctx, *groupID, requestedModel, selection.Account.ID)
}

func (s *GatewayService) RecordPrimaryFailure(ctx context.Context, groupID *int64, requestedModel string, selection *AccountSelectionResult, failoverErr *UpstreamFailoverError) {
	recordPrimaryFailure(ctx, s.primaryHealthService, groupID, requestedModel, selection, failoverErr)
}

func (s *GatewayService) RecordPrimaryProbeSuccess(ctx context.Context, groupID *int64, requestedModel string, selection *AccountSelectionResult) {
	recordPrimaryProbeSuccess(ctx, s.primaryHealthService, groupID, requestedModel, selection)
}

func (s *OpenAIGatewayService) RecordPrimaryFailure(ctx context.Context, groupID *int64, requestedModel string, selection *AccountSelectionResult, failoverErr *UpstreamFailoverError) {
	recordPrimaryFailure(ctx, s.primaryHealthService, groupID, requestedModel, selection, failoverErr)
}

func (s *OpenAIGatewayService) RecordPrimaryProbeSuccess(ctx context.Context, groupID *int64, requestedModel string, selection *AccountSelectionResult) {
	recordPrimaryProbeSuccess(ctx, s.primaryHealthService, groupID, requestedModel, selection)
}
