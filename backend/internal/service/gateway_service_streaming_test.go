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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type upstreamContextTestKey string

func TestGatewayService_StreamingReusesScannerBufferAndStillParsesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}

	svc := &GatewayService{
		cfg:              cfg,
		rateLimitService: &RateLimitService{},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"model\":\"upstream-model\",\"usage\":{\"input_tokens\":3}}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n"))
		_, _ = pw.Write([]byte("data: [DONE]\n\n"))
	}()

	result, err := svc.handleStreamingResponse(tracker.Context(), resp, c, &Account{ID: 1}, time.Now(), "public-model", "upstream-model", false)
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 3, result.usage.InputTokens)
	require.Equal(t, 7, result.usage.OutputTokens)
	require.Contains(t, rec.Body.String(), `"model":"public-model"`)
	require.Equal(t, 1, strings.Count(rec.Body.String(), `"type":"message_start"`))
}

type streamReadError struct{ err error }

func (r streamReadError) Read([]byte) (int, error) { return 0, r.err }

func TestGatewayService_PreContentMetadataDiscardedOnReadFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	payload := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(strings.NewReader(payload), streamReadError{err: io.ErrUnexpectedEOF}))}

	_, err := svc.handleStreamingResponse(tracker.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Empty(t, rec.Body.String())
	require.False(t, tracker.HasEffectiveContent())
}

func TestGatewayService_PingBeforeReadFailureStillReturnsFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	payload := "event: ping\ndata: {\"type\":\"ping\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(strings.NewReader(payload), streamReadError{err: io.ErrUnexpectedEOF}))}

	_, err := svc.handleStreamingResponse(tracker.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Contains(t, rec.Body.String(), "event: ping")
	require.False(t, tracker.HasEffectiveContent())
}

func TestGatewayService_ReadFailureAfterEffectiveContentDoesNotFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	payload := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(strings.NewReader(payload), streamReadError{err: io.ErrUnexpectedEOF}))}

	_, err := svc.handleStreamingResponse(tracker.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	var failoverErr *UpstreamFailoverError
	require.Error(t, err)
	require.False(t, errors.As(err, &failoverErr))
	require.True(t, tracker.HasEffectiveContent())
	require.Contains(t, rec.Body.String(), "hello")
	require.Contains(t, rec.Body.String(), "event: error")
}

func TestGatewayService_CleanEmptyEOFFailsOverBeforeContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}

	_, err := svc.handleStreamingResponse(tracker.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Empty(t, rec.Body.String())
}

func TestGatewayService_PreContentIdleTimeoutFailsOverWithoutSSEError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 1}}, rateLimitService: &RateLimitService{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	tracker := NewPreContentTracker(context.Background(), 3*time.Second)
	defer tracker.Close()
	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}

	_, err := svc.handleStreamingResponse(tracker.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	_ = pr.Close()
	_ = pw.Close()
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Empty(t, rec.Body.String())
}

func TestGatewayService_AnthropicPassthroughUsesPreContentBuffer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	tracker := NewPreContentTracker(context.Background(), time.Second)
	defer tracker.Close()
	payload := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":2}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload))}

	result, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(tracker.Context(), resp, c, &Account{ID: 1}, time.Now(), "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, tracker.HasEffectiveContent())
	require.Contains(t, rec.Body.String(), "hello")
	require.Equal(t, 1, strings.Count(rec.Body.String(), "event: message_start"))
}

func TestDetachUpstreamContextIgnoresClientCancel(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), upstreamContextTestKey("test-key"), "test-value"))
	upstreamCtx, release := detachUpstreamContext(parent)
	defer release()

	cancel()

	require.NoError(t, upstreamCtx.Err())
	require.Equal(t, "test-value", upstreamCtx.Value(upstreamContextTestKey("test-key")))
}

func TestDetachStreamUpstreamContext_PreservesTrackedClientCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	tracker := NewPreContentTracker(parent, time.Second)
	defer tracker.Close()
	require.True(t, tracker.MarkEffectiveContent())
	upstreamCtx, release := detachStreamUpstreamContext(tracker.Context(), true)
	defer release()

	cancel()
	select {
	case <-upstreamCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("tracked stream ignored client cancellation")
	}
}
