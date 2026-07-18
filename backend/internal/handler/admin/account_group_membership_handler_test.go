package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountHandlerCreateRoundTripsMembershipInput(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupAccountMixedChannelRouter(adminSvc)
	enabled := false
	priority := 9
	body, err := json.Marshal(map[string]any{
		"name":        "standby-account",
		"platform":    "anthropic",
		"type":        "apikey",
		"credentials": map[string]any{"api_key": "secret"},
		"account_groups": []AccountGroupMembershipRequest{{
			GroupID:      44,
			Priority:     &priority,
			Role:         string(service.AccountGroupRoleStandby),
			Enabled:      &enabled,
			ModelMapping: map[string]string{"claude-opus-*": "claude-sonnet-4-6"},
		}},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Nil(t, adminSvc.createdAccounts[0].GroupIDs)
	require.Equal(t, []service.AccountGroupInput{{
		GroupID:      44,
		Priority:     &priority,
		Role:         string(service.AccountGroupRoleStandby),
		Enabled:      &enabled,
		ModelMapping: map[string]string{"claude-opus-*": "claude-sonnet-4-6"},
	}}, adminSvc.createdAccounts[0].AccountGroups)
}

func TestAccountHandlerUpdateAcceptsEmptyMembershipList(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupAccountMixedChannelRouter(adminSvc)
	body := []byte(`{"account_groups":[]}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/300", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, adminSvc.updatedAccounts, 1)
	require.NotNil(t, adminSvc.updatedAccounts[0].AccountGroups)
	require.Empty(t, *adminSvc.updatedAccounts[0].AccountGroups)
	require.Nil(t, adminSvc.updatedAccounts[0].GroupIDs)
}

func TestAccountHandlerRejectsInvalidMembershipRole(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupAccountMixedChannelRouter(adminSvc)
	body := []byte(`{
		"name":"bad-role",
		"platform":"anthropic",
		"type":"apikey",
		"credentials":{"api_key":"secret"},
		"account_groups":[{"group_id":44,"role":"backup"}]
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, adminSvc.createdAccounts)
}
