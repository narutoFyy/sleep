package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// partialMessageStartSSE 模拟 handleStreamingResponse 已写入的首批 SSE 事件。
const partialMessageStartSSE = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-sonnet-4-5\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n" +
	"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"

// Metadata and protocol-safe pings may make gin's writer non-empty, but they do
// not commit an Anthropic account attempt before effective model content.
func TestClaudeAttemptCommitted_MetadataAndPingDoNotBlockFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	sizeBeforeForward := c.Writer.Size()
	require.Equal(t, -1, sizeBeforeForward, "gin writer 初始 Size 应为 -1（未写入任何字节）")

	_, err := c.Writer.Write([]byte(partialMessageStartSSE))
	require.NoError(t, err)

	require.NotEqual(t, sizeBeforeForward, c.Writer.Size(),
		"写入 SSE 内容后 writer size 必须增加，守卫条件应为 true")

	_, err = c.Writer.Write([]byte("event: ping\ndata: {\"type\":\"ping\"}\n\n"))
	require.NoError(t, err)

	tracker := service.NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	require.False(t, claudeAttemptCommitted(c, tracker, true, sizeBeforeForward))
}

func TestClaudeAttemptCommitted_EffectiveContentPreventsRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	tracker := service.NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	require.True(t, tracker.MarkEffectiveContent())
	require.True(t, claudeAttemptCommitted(c, tracker, true, c.Writer.Size()))
}

func TestPreContentRoutingDeadline_CancelsSameAccountRetry(t *testing.T) {
	tracker := service.NewPreContentTracker(context.Background(), 10*time.Millisecond)
	defer tracker.Close()
	state := NewFailoverState(10, false)
	start := time.Now()
	action := state.HandleFailoverError(tracker.Context(), nil, 1, service.PlatformAnthropic, &service.UpstreamFailoverError{
		StatusCode:             http.StatusBadGateway,
		RetryableOnSameAccount: true,
	})
	require.Equal(t, FailoverCanceled, action)
	require.Less(t, time.Since(start), 200*time.Millisecond)
	require.True(t, tracker.DeadlineExceeded())
}

func TestHandlePreContentRoutingDeadline_WritesTimeoutAfterPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	_, err := c.Writer.Write([]byte("event: ping\ndata: {\"type\":\"ping\"}\n\n"))
	require.NoError(t, err)
	tracker := service.NewPreContentTracker(context.Background(), 5*time.Millisecond)
	defer tracker.Close()
	select {
	case <-tracker.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("routing deadline did not fire")
	}

	h := &GatewayHandler{}
	require.True(t, h.handlePreContentRoutingDeadline(c, tracker))
	require.Contains(t, w.Body.String(), `"type":"error"`)
	require.Contains(t, w.Body.String(), `"type":"timeout_error"`)
}

// TestStreamWrittenGuard_GeminiPath_AbortFailoverOnSSEContentWritten 与上述测试相同，
// 验证 Gemini 路径使用 service.PlatformGemini（而非 account.Platform）时行为一致。
func TestStreamWrittenGuard_GeminiPath_AbortFailoverOnSSEContentWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.0-flash:streamGenerateContent", nil)

	sizeBeforeForward := c.Writer.Size()

	_, err := c.Writer.Write([]byte(partialMessageStartSSE))
	require.NoError(t, err)

	require.NotEqual(t, sizeBeforeForward, c.Writer.Size())

	failoverErr := &service.UpstreamFailoverError{
		StatusCode: http.StatusForbidden,
	}

	h := &GatewayHandler{}
	h.handleFailoverExhausted(c, failoverErr, service.PlatformGemini, true)

	body := w.Body.String()

	require.Contains(t, body, "event: message_start")
	require.Contains(t, body, `"type":"error"`)

	firstIdx := strings.Index(body, "event: message_start")
	lastIdx := strings.LastIndex(body, "event: message_start")
	assert.Equal(t, firstIdx, lastIdx, "Gemini 路径不得出现双 message_start")
}

// TestStreamWrittenGuard_NoByteWritten_GuardNotTriggered 验证反向场景：
// 当 Forward 返回 UpstreamFailoverError 时若未向客户端写入任何 SSE 内容，
// 守卫条件（c.Writer.Size() != sizeBeforeForward）为 false，不应中止 failover。
func TestStreamWrittenGuard_NoByteWritten_GuardNotTriggered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	// 模拟 writerSizeBeforeForward：初始为 -1
	sizeBeforeForward := c.Writer.Size()

	// Forward 未写入任何字节直接返回错误（例如 401 发生在连接建立前）
	// c.Writer.Size() 仍为 -1

	// 守卫条件：sizeBeforeForward == c.Writer.Size() → 不触发
	guardTriggered := c.Writer.Size() != sizeBeforeForward
	require.False(t, guardTriggered,
		"未写入任何字节时，守卫条件必须为 false，应允许正常 failover 继续")
}
