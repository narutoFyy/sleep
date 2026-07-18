package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountGroupFromServiceIncludesMembershipFields(t *testing.T) {
	got := AccountGroupFromService(&service.AccountGroup{
		AccountID:    12,
		GroupID:      44,
		Priority:     3,
		Role:         service.AccountGroupRoleStandby,
		Enabled:      false,
		ModelMapping: map[string]string{"claude-opus-*": "claude-sonnet-4-6"},
	})

	require.Equal(t, "standby", got.Role)
	require.False(t, got.Enabled)
	require.Equal(t, map[string]string{"claude-opus-*": "claude-sonnet-4-6"}, got.ModelMapping)
}
