package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicAttemptBuffer_MetadataDiscardedBeforeContentFailure(t *testing.T) {
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	buffer := newAnthropicAttemptBuffer(tracker.Context(), 1024)

	out, committed, err := buffer.Accept([]byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\n"))
	require.NoError(t, err)
	require.Empty(t, out)
	require.False(t, committed)

	buffer.Discard()
	require.False(t, buffer.Committed())
	require.False(t, tracker.HasEffectiveContent())
}

func TestAnthropicAttemptBuffer_PingPassesWithoutCommitting(t *testing.T) {
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	buffer := newAnthropicAttemptBuffer(tracker.Context(), 1024)
	ping := []byte("event: ping\ndata: {\"type\":\"ping\"}\n\n")

	out, committed, err := buffer.Accept(ping)
	require.NoError(t, err)
	require.Equal(t, ping, out)
	require.False(t, committed)
	require.False(t, buffer.Committed())

	comment := []byte(": keepalive\n\n")
	out, committed, err = buffer.Accept(comment)
	require.NoError(t, err)
	require.Equal(t, comment, out)
	require.False(t, committed)
}

func TestAnthropicAttemptBuffer_FirstEffectiveEventFlushesMetadataExactlyOnce(t *testing.T) {
	tests := []struct {
		name  string
		event string
	}{
		{name: "text", event: `event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}\n\n`},
		{name: "thinking", event: `event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"hmm"}}\n\n`},
		{name: "tool", event: `event: content_block_start\ndata: {"type":"content_block_start","content_block":{"type":"tool_use","id":"tool_1","name":"search","input":{}}}\n\n`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewPreContentTracker(context.Background(), time.Second)
			defer tracker.Close()
			buffer := newAnthropicAttemptBuffer(tracker.Context(), 4096)
			metadata := []byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
			_, _, err := buffer.Accept(metadata)
			require.NoError(t, err)

			out, committed, err := buffer.Accept([]byte(strings.ReplaceAll(tt.event, `\n`, "\n")))
			require.NoError(t, err)
			require.True(t, committed)
			require.True(t, tracker.HasEffectiveContent())
			require.Equal(t, 1, strings.Count(string(out), "event: message_start"))

			after := []byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\"}\n\n")
			out, committed, err = buffer.Accept(after)
			require.NoError(t, err)
			require.False(t, committed)
			require.Equal(t, after, out)
		})
	}
}

func TestPreContentTracker_DeadlineCancelsSharedContext(t *testing.T) {
	tracker := NewPreContentTracker(context.Background(), 10*time.Millisecond)
	defer tracker.Close()

	select {
	case <-tracker.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("routing context was not canceled")
	}
	require.True(t, tracker.DeadlineExceeded())
	require.True(t, errors.Is(context.Cause(tracker.Context()), ErrPreContentRoutingDeadline))
}

func TestPreContentTracker_EffectiveContentLetsStreamOutliveDeadline(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	tracker := NewPreContentTracker(parent, 20*time.Millisecond)
	defer tracker.Close()
	require.True(t, tracker.MarkEffectiveContent())

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, tracker.Context().Err())

	cancel()
	select {
	case <-tracker.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("client cancellation did not cancel the stream")
	}
}

func TestAnthropicAttemptBuffer_WithoutTrackerPreservesNonStreamingLegacyBehavior(t *testing.T) {
	buffer := newAnthropicAttemptBuffer(context.Background(), 1024)
	metadata := []byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
	out, committed, err := buffer.Accept(metadata)
	require.NoError(t, err)
	require.False(t, committed)
	require.Equal(t, metadata, out)
}

func TestAnthropicStreamFailoverError_SwitchesAccountImmediately(t *testing.T) {
	err := newAnthropicStreamFailoverError("stream_timeout", "timed out before content")
	require.False(t, err.RetryableOnSameAccount)
}

func TestAnthropicPassthroughNonStreaming_MarksEffectiveContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"model":"claude-test","usage":{}}`)),
	}

	_, err := (&GatewayService{}).handleNonStreamingResponseAnthropicAPIKeyPassthrough(
		tracker.Context(),
		resp,
		c,
		&Account{},
	)
	require.NoError(t, err)
	require.True(t, tracker.HasEffectiveContent())
	require.Equal(t, http.StatusOK, recorder.Code)
}
