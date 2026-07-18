package handler

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestShouldAttemptStandby(t *testing.T) {
	tests := []struct {
		name                   string
		selectionErr           error
		primaryFailureObserved bool
		want                   bool
	}{
		{name: "open circuit", selectionErr: service.ErrPrimaryCircuitOpen, want: true},
		{name: "wrapped open circuit", selectionErr: errors.Join(errors.New("select failed"), service.ErrPrimaryCircuitOpen), want: true},
		{name: "primary exhaustion after upstream failure", selectionErr: service.ErrNoAvailableAccounts, primaryFailureObserved: true, want: true},
		{name: "no eligible primary accounts", selectionErr: service.ErrNoAvailableAccounts, want: true},
		{name: "wrapped no eligible primary accounts", selectionErr: errors.Join(errors.New("select failed"), service.ErrNoAvailableAccounts), want: true},
		{name: "unrelated selection error", selectionErr: errors.New("database unavailable"), want: false},
		{name: "nil selection without upstream failure", selectionErr: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldAttemptStandby(tt.selectionErr, tt.primaryFailureObserved))
		})
	}
}
