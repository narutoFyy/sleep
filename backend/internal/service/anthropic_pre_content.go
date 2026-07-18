package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultPreContentRoutingDeadline  = 100 * time.Second
	maxAnthropicPreContentBufferBytes = 1 << 20 // Framing before content is normally tiny; cap it at 1 MiB.
)

var (
	ErrPreContentRoutingDeadline      = errors.New("pre-content routing deadline exceeded")
	errAnthropicPreContentStreamError = errors.New("upstream sent an error before effective content")
)

type preContentTrackerContextKey struct{}

// PreContentTracker owns the single routing deadline shared by all account
// attempts. Once effective content starts, only the original client context can
// cancel the stream.
type PreContentTracker struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	timer  *time.Timer

	mu            sync.Mutex
	effective     bool
	deadlineFired bool
}

func NewPreContentTracker(parent context.Context, deadline time.Duration) *PreContentTracker {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancelCause(parent)
	tracker := &PreContentTracker{ctx: ctx, cancel: cancel}
	tracker.timer = time.AfterFunc(deadline, func() {
		tracker.mu.Lock()
		if tracker.effective || tracker.deadlineFired || tracker.ctx.Err() != nil {
			tracker.mu.Unlock()
			return
		}
		tracker.deadlineFired = true
		tracker.mu.Unlock()
		tracker.cancel(ErrPreContentRoutingDeadline)
	})
	return tracker
}

func (t *PreContentTracker) Context() context.Context {
	if t == nil {
		return context.Background()
	}
	return context.WithValue(t.ctx, preContentTrackerContextKey{}, t)
}

// MarkEffectiveContent permanently disarms the routing timer. It returns false
// if the deadline or client cancellation already won the race.
func (t *PreContentTracker) MarkEffectiveContent() bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.deadlineFired || t.ctx.Err() != nil {
		return false
	}
	if !t.effective {
		t.effective = true
		if t.timer != nil {
			t.timer.Stop()
		}
	}
	return true
}

func (t *PreContentTracker) HasEffectiveContent() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.effective
}

func (t *PreContentTracker) DeadlineExceeded() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.deadlineFired
}

func (t *PreContentTracker) Close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.timer != nil {
		t.timer.Stop()
	}
	t.mu.Unlock()
	t.cancel(context.Canceled)
}

func PreContentTrackerFromContext(ctx context.Context) *PreContentTracker {
	if ctx == nil {
		return nil
	}
	tracker, _ := ctx.Value(preContentTrackerContextKey{}).(*PreContentTracker)
	return tracker
}

func IsPreContentRoutingDeadline(ctx context.Context) bool {
	return ctx != nil && errors.Is(context.Cause(ctx), ErrPreContentRoutingDeadline)
}

type anthropicAttemptBuffer struct {
	tracker   *PreContentTracker
	limit     int
	buffer    bytes.Buffer
	committed bool
}

func newAnthropicAttemptBuffer(ctx context.Context, configuredMax int) *anthropicAttemptBuffer {
	limit := maxAnthropicPreContentBufferBytes
	if configuredMax > 0 && configuredMax < limit {
		limit = configuredMax
	}
	tracker := PreContentTrackerFromContext(ctx)
	return &anthropicAttemptBuffer{
		tracker:   tracker,
		limit:     limit,
		committed: tracker == nil, // Non-Messages callers retain legacy immediate streaming.
	}
}

func (b *anthropicAttemptBuffer) Committed() bool {
	return b != nil && b.committed
}

func (b *anthropicAttemptBuffer) Discard() {
	if b != nil && !b.committed {
		b.buffer.Reset()
	}
}

// Accept returns bytes that may be written now and whether this event committed
// the attempt. Ping events bypass buffering without committing the attempt.
func (b *anthropicAttemptBuffer) Accept(block []byte) ([]byte, bool, error) {
	if b == nil || b.committed {
		return block, false, nil
	}
	classification := classifyAnthropicSSEBlock(block)
	if classification.ping {
		return block, false, nil
	}
	if classification.upstreamError {
		b.Discard()
		return nil, false, errAnthropicPreContentStreamError
	}

	overflow := b.buffer.Len()+len(block) > b.limit
	if !classification.effective && !overflow {
		_, _ = b.buffer.Write(block)
		return nil, false, nil
	}
	if b.tracker != nil && !b.tracker.MarkEffectiveContent() {
		b.Discard()
		if b.tracker.DeadlineExceeded() {
			return nil, false, ErrPreContentRoutingDeadline
		}
		return nil, false, context.Cause(b.tracker.ctx)
	}

	out := make([]byte, 0, b.buffer.Len()+len(block))
	out = append(out, b.buffer.Bytes()...)
	out = append(out, block...)
	b.buffer.Reset()
	b.committed = true
	return out, true, nil
}

type anthropicSSEClassification struct {
	ping          bool
	effective     bool
	upstreamError bool
}

func classifyAnthropicSSEBlock(block []byte) anthropicSSEClassification {
	eventName := ""
	data := ""
	commentOnly := false
	for _, line := range strings.Split(string(block), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ":") {
			commentOnly = true
			continue
		}
		if strings.HasPrefix(trimmed, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			continue
		}
		if data == "" && strings.HasPrefix(trimmed, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		}
	}

	if eventName == "ping" || eventName == "keepalive" || (commentOnly && eventName == "" && data == "") {
		return anthropicSSEClassification{ping: true}
	}
	if eventName == "error" {
		return anthropicSSEClassification{upstreamError: true}
	}
	if data == "" || data == "[DONE]" {
		return anthropicSSEClassification{}
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		// Unknown non-empty payloads are conservatively client-visible content.
		return anthropicSSEClassification{effective: true}
	}
	eventType, _ := event["type"].(string)
	if eventName == "" {
		eventName = eventType
	}
	if eventName == "ping" || eventName == "keepalive" || eventType == "ping" || eventType == "keepalive" {
		return anthropicSSEClassification{ping: true}
	}
	if eventName == "error" || eventType == "error" {
		return anthropicSSEClassification{upstreamError: true}
	}

	semanticType := eventType
	if semanticType == "" {
		semanticType = eventName
	}
	switch semanticType {
	case "content_block_start":
		block, _ := event["content_block"].(map[string]any)
		blockType, _ := block["type"].(string)
		if blockType == "tool_use" || blockType == "server_tool_use" {
			return anthropicSSEClassification{effective: true}
		}
		if nonEmptyString(block, "text", "thinking", "reasoning") {
			return anthropicSSEClassification{effective: true}
		}
	case "content_block_delta":
		delta, _ := event["delta"].(map[string]any)
		deltaType, _ := delta["type"].(string)
		if deltaType == "input_json_delta" && nonEmptyString(delta, "partial_json") {
			return anthropicSSEClassification{effective: true}
		}
		if nonEmptyString(delta, "text", "thinking", "reasoning") {
			return anthropicSSEClassification{effective: true}
		}
	}
	return anthropicSSEClassification{}
}

func nonEmptyString(object map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func newAnthropicStreamFailoverError(reason, message string) *UpstreamFailoverError {
	body, _ := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]string{
			"type":    reason,
			"message": message,
		},
	})
	return &UpstreamFailoverError{
		StatusCode:   http.StatusBadGateway,
		ResponseBody: body,
	}
}
