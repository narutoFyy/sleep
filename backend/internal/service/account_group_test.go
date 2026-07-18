package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAccountGroupInputsDefaults(t *testing.T) {
	memberships, err := NormalizeAccountGroupInputs([]AccountGroupInput{
		{GroupID: 10},
		{GroupID: 20, Role: string(AccountGroupRoleStandby), ModelMapping: map[string]string{"claude-opus-*": "claude-sonnet-4-6"}},
	})
	require.NoError(t, err)
	require.Equal(t, []AccountGroup{
		{
			GroupID:      10,
			Priority:     1,
			Role:         AccountGroupRolePrimary,
			Enabled:      true,
			ModelMapping: map[string]string{},
		},
		{
			GroupID:      20,
			Priority:     2,
			Role:         AccountGroupRoleStandby,
			Enabled:      true,
			ModelMapping: map[string]string{"claude-opus-*": "claude-sonnet-4-6"},
		},
	}, memberships)
}

func TestNormalizeAccountGroupInputsExplicitValues(t *testing.T) {
	priority := 30
	enabled := false
	mapping := map[string]string{"gpt-5": "gpt-5-mini"}

	memberships, err := NormalizeAccountGroupInputs([]AccountGroupInput{{
		GroupID:      7,
		Priority:     &priority,
		Role:         string(AccountGroupRoleStandby),
		Enabled:      &enabled,
		ModelMapping: mapping,
	}})
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	require.Equal(t, 30, memberships[0].Priority)
	require.False(t, memberships[0].Enabled)
	require.Equal(t, AccountGroupRoleStandby, memberships[0].Role)
	require.Equal(t, mapping, memberships[0].ModelMapping)

	mapping["gpt-5"] = "changed"
	require.Equal(t, "gpt-5-mini", memberships[0].ModelMapping["gpt-5"])
}

func TestNormalizeAccountGroupInputsRejectsInvalidRole(t *testing.T) {
	_, err := NormalizeAccountGroupInputs([]AccountGroupInput{{GroupID: 1, Role: "backup"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "role must be primary or standby")
}

func TestNormalizeAccountGroupInputsRejectsDuplicateGroup(t *testing.T) {
	_, err := NormalizeAccountGroupInputs([]AccountGroupInput{{GroupID: 1}, {GroupID: 1}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate group_id")
}
