package service

import (
	"context"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AccountGroupRole string

const (
	AccountGroupRolePrimary AccountGroupRole = "primary"
	AccountGroupRoleStandby AccountGroupRole = "standby"
)

type AccountGroup struct {
	AccountID    int64
	GroupID      int64
	Priority     int
	Role         AccountGroupRole
	Enabled      bool
	ModelMapping map[string]string
	CreatedAt    time.Time

	Account *Account
	Group   *Group
}

// AccountGroupInput is the admin-facing representation used when replacing an
// account's group memberships. Nil optional fields preserve legacy defaults.
type AccountGroupInput struct {
	GroupID      int64
	Priority     *int
	Role         string
	Enabled      *bool
	ModelMapping map[string]string
}

// AccountGroupMembershipBinder is implemented by repositories that can persist
// the extended membership fields. AccountRepository intentionally keeps its
// legacy BindGroups method so existing callers and test doubles remain valid.
type AccountGroupMembershipBinder interface {
	BindAccountGroups(ctx context.Context, accountID int64, memberships []AccountGroup) error
}

func NormalizeAccountGroupInputs(inputs []AccountGroupInput) ([]AccountGroup, error) {
	memberships := make([]AccountGroup, 0, len(inputs))
	seen := make(map[int64]struct{}, len(inputs))
	for i, input := range inputs {
		if input.GroupID <= 0 {
			return nil, infraerrors.BadRequest("INVALID_ACCOUNT_GROUP", "group_id must be greater than 0")
		}
		if _, ok := seen[input.GroupID]; ok {
			return nil, infraerrors.BadRequest("INVALID_ACCOUNT_GROUP", fmt.Sprintf("duplicate group_id: %d", input.GroupID))
		}
		seen[input.GroupID] = struct{}{}

		role := AccountGroupRole(input.Role)
		if role == "" {
			role = AccountGroupRolePrimary
		}
		if role != AccountGroupRolePrimary && role != AccountGroupRoleStandby {
			return nil, infraerrors.BadRequest("INVALID_ACCOUNT_GROUP_ROLE", "role must be primary or standby")
		}

		priority := i + 1
		if input.Priority != nil {
			priority = *input.Priority
		}
		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}

		memberships = append(memberships, AccountGroup{
			GroupID:      input.GroupID,
			Priority:     priority,
			Role:         role,
			Enabled:      enabled,
			ModelMapping: cloneStringMap(input.ModelMapping),
		})
	}
	return memberships, nil
}

func LegacyAccountGroupMemberships(groupIDs []int64) []AccountGroup {
	memberships := make([]AccountGroup, 0, len(groupIDs))
	for i, groupID := range groupIDs {
		memberships = append(memberships, AccountGroup{
			GroupID:      groupID,
			Priority:     i + 1,
			Role:         AccountGroupRolePrimary,
			Enabled:      true,
			ModelMapping: map[string]string{},
		})
	}
	return memberships
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
