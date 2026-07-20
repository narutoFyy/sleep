package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAnthropicAbnormalOutputDetectorRepeatedWordAcrossFrames(t *testing.T) {
	d := newAnthropicAbnormalOutputDetector()
	for i := 0; i < 7; i++ {
		require.False(t, d.ObserveText("course ").Triggered)
	}
	got := d.ObserveText("course ")
	require.True(t, got.Triggered)
	require.Equal(t, "repeated_word", got.Reason)
	require.Equal(t, 8, got.Count)
}

func TestAnthropicAbnormalOutputDetectorRepeatedLineAndChunkBoundary(t *testing.T) {
	d := newAnthropicAbnormalOutputDetector()
	for i := 0; i < 5; i++ {
		require.False(t, d.ObserveText("same answer").Triggered)
		require.False(t, d.ObserveText("\n").Triggered)
	}
	require.False(t, d.ObserveText("same answer").Triggered)
	got := d.ObserveText("\n")
	require.True(t, got.Triggered)
	require.Equal(t, "repeated_line", got.Reason)
	require.Equal(t, 6, got.Count)
}

func TestAnthropicAbnormalOutputDetectorIgnoresCodeFence(t *testing.T) {
	d := newAnthropicAbnormalOutputDetector()
	d.ObserveText("```\n")
	for i := 0; i < 12; i++ {
		require.False(t, d.ObserveText("course ").Triggered)
	}
	d.ObserveText("```\n")
	require.False(t, d.ObserveText("normal ").Triggered)
}

func TestBuildAnthropicContinuationBodyAppendsAssistantPrefill(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-8","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	got, err := BuildAnthropicContinuationBody(body, "partial answer")
	require.NoError(t, err)

	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(got, &payload))
	require.Len(t, payload.Messages, 2)
	require.Equal(t, "assistant", payload.Messages[1].Role)
	require.Equal(t, "partial answer", payload.Messages[1].Content)
}

func TestDetectAnthropicNonStreamingAbnormalOutput(t *testing.T) {
	body := []byte(`{"content":[{"type":"text","text":"course course course course course course course course "}]}`)
	got := detectAnthropicNonStreamingAbnormalOutput(body, nil)
	require.True(t, got.Triggered)
	require.Equal(t, "repeated_word", got.Reason)
}

func TestGatewayServiceAbnormalOutputIsolationIsUserScoped(t *testing.T) {
	svc := &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{AbnormalOutputAccountIsolationSeconds: 600}}}
	groupID := int64(3)
	svc.RecordAbnormalOutputIsolation(76, &groupID, "claude-opus-4-8", 8033)
	require.Contains(t, svc.AbnormalOutputIsolatedAccounts(76, &groupID, "claude-opus-4-8"), int64(8033))
	require.Empty(t, svc.AbnormalOutputIsolatedAccounts(77, &groupID, "claude-opus-4-8"))
	require.Empty(t, svc.AbnormalOutputIsolatedAccounts(76, &groupID, "claude-sonnet-4-8"))
}
