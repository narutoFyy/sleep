package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/config"
	gocache "github.com/patrickmn/go-cache"
)

const (
	defaultAbnormalOutputRepeatWindow    = 5 * time.Second
	defaultAbnormalOutputLineThreshold   = 6
	defaultAbnormalOutputWordThreshold   = 8
	defaultAbnormalOutputMinWordLength   = 3
	defaultAbnormalOutputContinuationMax = 256 * 1024
)

type abnormalOutputDetection struct {
	Triggered bool
	Reason    string
	Candidate string
	Count     int
}

// anthropicAbnormalOutputDetector works on text content rather than SSE frame
// boundaries. Upstreams are free to split one word across several frames.
type anthropicAbnormalOutputDetector struct {
	window        time.Duration
	lineThreshold int
	wordThreshold int
	minWordLength int

	lastObservedAt time.Time
	lineCarry      string
	wordCarry      string
	previousLine   string
	lineCount      int
	previousWord   string
	wordCount      int
	unitsSeen      int
	inCodeFence    bool
}

func newAnthropicAbnormalOutputDetector() *anthropicAbnormalOutputDetector {
	return &anthropicAbnormalOutputDetector{
		window:        defaultAbnormalOutputRepeatWindow,
		lineThreshold: defaultAbnormalOutputLineThreshold,
		wordThreshold: defaultAbnormalOutputWordThreshold,
		minWordLength: defaultAbnormalOutputMinWordLength,
	}
}

func newAnthropicAbnormalOutputDetectorForConfig(cfg *config.Config) *anthropicAbnormalOutputDetector {
	d := newAnthropicAbnormalOutputDetector()
	if cfg == nil {
		return d
	}
	settings := cfg.Gateway
	if settings.AbnormalOutputRepeatWindowSeconds > 0 {
		d.window = time.Duration(settings.AbnormalOutputRepeatWindowSeconds) * time.Second
	}
	if settings.AbnormalOutputLineRepeatThreshold > 0 {
		d.lineThreshold = settings.AbnormalOutputLineRepeatThreshold
	}
	if settings.AbnormalOutputWordRepeatThreshold > 0 {
		d.wordThreshold = settings.AbnormalOutputWordRepeatThreshold
	}
	if settings.AbnormalOutputMinWordCharacters > 0 {
		d.minWordLength = settings.AbnormalOutputMinWordCharacters
	}
	return d
}

func (d *anthropicAbnormalOutputDetector) ObserveText(text string) abnormalOutputDetection {
	if d == nil || text == "" {
		return abnormalOutputDetection{}
	}
	now := time.Now()
	if !d.lastObservedAt.IsZero() && now.Sub(d.lastObservedAt) > d.window {
		d.previousLine, d.previousWord = "", ""
		d.lineCount, d.wordCount = 0, 0
	}
	d.lastObservedAt = now

	for _, r := range text {
		d.lineCarry += string(r)
		if r == '\n' {
			line := strings.TrimSpace(d.lineCarry[:len(d.lineCarry)-1])
			d.lineCarry = ""
			if detected := d.observeLine(line); detected.Triggered {
				return detected
			}
		}

		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			d.wordCarry += string(r)
			continue
		}
		if detected := d.observeWord(d.wordCarry); detected.Triggered {
			return detected
		}
		d.wordCarry = ""
	}
	return abnormalOutputDetection{}
}

func (d *anthropicAbnormalOutputDetector) Flush() abnormalOutputDetection {
	if d == nil {
		return abnormalOutputDetection{}
	}
	if detected := d.observeWord(d.wordCarry); detected.Triggered {
		d.wordCarry = ""
		return detected
	}
	d.wordCarry = ""
	if strings.TrimSpace(d.lineCarry) != "" {
		return d.observeLine(strings.TrimSpace(d.lineCarry))
	}
	d.lineCarry = ""
	return abnormalOutputDetection{}
}

func (d *anthropicAbnormalOutputDetector) UnitsSeen() int {
	if d == nil {
		return 0
	}
	return d.unitsSeen
}

func (d *anthropicAbnormalOutputDetector) observeLine(line string) abnormalOutputDetection {
	normalized := normalizeAbnormalOutputUnit(line)
	if strings.HasPrefix(normalized, "```") {
		d.inCodeFence = !d.inCodeFence
		d.previousLine, d.lineCount = "", 0
		return abnormalOutputDetection{}
	}
	if d.inCodeFence || !hasAbnormalOutputAlphaNumeric(normalized) {
		return abnormalOutputDetection{}
	}
	d.unitsSeen++
	if normalized == d.previousLine {
		d.lineCount++
	} else {
		d.previousLine, d.lineCount = normalized, 1
	}
	if d.lineCount >= d.lineThreshold {
		return abnormalOutputDetection{Triggered: true, Reason: "repeated_line", Candidate: normalized, Count: d.lineCount}
	}
	return abnormalOutputDetection{}
}

func (d *anthropicAbnormalOutputDetector) observeWord(word string) abnormalOutputDetection {
	if d.inCodeFence || len([]rune(word)) < d.minWordLength {
		return abnormalOutputDetection{}
	}
	d.unitsSeen++
	word = strings.ToLower(word)
	if word == d.previousWord {
		d.wordCount++
	} else {
		d.previousWord, d.wordCount = word, 1
	}
	if d.wordCount >= d.wordThreshold {
		return abnormalOutputDetection{Triggered: true, Reason: "repeated_word", Candidate: word, Count: d.wordCount}
	}
	return abnormalOutputDetection{}
}

func normalizeAbnormalOutputUnit(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func hasAbnormalOutputAlphaNumeric(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

type abnormalOutputContextKey struct{}

type anthropicContinuationState struct {
	VisibleText    string
	TextBlockIndex int
	OriginalUsage  ClaudeUsage
}

// WithAnthropicContinuation marks a request as the second leg of one logical
// downstream stream. The text is already visible to the client and is used to
// prefill the replacement account.
func WithAnthropicContinuation(ctx context.Context, visibleText string, textBlockIndex int, originalUsage ClaudeUsage) context.Context {
	return context.WithValue(ctx, abnormalOutputContextKey{}, anthropicContinuationState{
		VisibleText:    strings.TrimSpace(visibleText),
		TextBlockIndex: textBlockIndex,
		OriginalUsage:  originalUsage,
	})
}

func AnthropicContinuationFromContext(ctx context.Context) (anthropicContinuationState, bool) {
	if ctx == nil {
		return anthropicContinuationState{}, false
	}
	state, ok := ctx.Value(abnormalOutputContextKey{}).(anthropicContinuationState)
	return state, ok && state.VisibleText != ""
}

// BuildAnthropicContinuationBody uses Anthropic assistant prefill semantics so
// the replacement account continues from the text already shown to the user.
func BuildAnthropicContinuationBody(body []byte, visibleText string) ([]byte, error) {
	visibleText = strings.TrimSpace(visibleText)
	if visibleText == "" {
		return nil, errors.New("continuation text is empty")
	}
	if len([]byte(visibleText)) > defaultAbnormalOutputContinuationMax {
		data := []byte(visibleText)[len([]byte(visibleText))-defaultAbnormalOutputContinuationMax:]
		for len(data) > 0 && data[0]&0xc0 == 0x80 {
			data = data[1:]
		}
		visibleText = string(data)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var messages []json.RawMessage
	if raw := payload["messages"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &messages); err != nil {
			return nil, err
		}
	}
	if len(messages) == 0 {
		return nil, errors.New("continuation request has no messages")
	}
	prefill, err := json.Marshal(map[string]any{
		"role":    "assistant",
		"content": visibleText,
	})
	if err != nil {
		return nil, err
	}
	messages = append(messages, prefill)
	updatedMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}
	payload["messages"] = updatedMessages
	return json.Marshal(payload)
}

type AbnormalOutputFailoverError struct {
	UpstreamFailoverError
	Committed      bool
	Reason         string
	Candidate      string
	RepeatCount    int
	VisibleText    string
	TextBlockIndex int
	OriginalUsage  ClaudeUsage
}

func (e *AbnormalOutputFailoverError) Error() string {
	return "abnormal upstream output (failover)"
}

func (e *AbnormalOutputFailoverError) Unwrap() error {
	return &e.UpstreamFailoverError
}

func newAbnormalOutputFailoverError(committed bool, detection abnormalOutputDetection, visibleText string, textBlockIndex int, usage ClaudeUsage) *AbnormalOutputFailoverError {
	return &AbnormalOutputFailoverError{
		UpstreamFailoverError: UpstreamFailoverError{StatusCode: 502, FailoverReason: "abnormal_output"},
		Committed:             committed,
		Reason:                detection.Reason,
		Candidate:             detection.Candidate,
		RepeatCount:           detection.Count,
		VisibleText:           visibleText,
		TextBlockIndex:        textBlockIndex,
		OriginalUsage:         usage,
	}
}

func appendVisibleAnthropicText(dst *bytes.Buffer, text string) {
	if dst == nil || text == "" || dst.Len() >= defaultAbnormalOutputContinuationMax {
		return
	}
	remaining := defaultAbnormalOutputContinuationMax - dst.Len()
	data := []byte(text)
	if len(data) > remaining {
		data = data[:remaining]
		for len(data) > 0 && data[len(data)-1]&0xc0 == 0x80 {
			data = data[:len(data)-1]
		}
	}
	_, _ = dst.Write(data)
}

func extractAnthropicTextDelta(data string) string {
	var event struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return ""
	}
	if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
		return event.Delta.Text
	}
	return ""
}

func extractAnthropicTextDeltaFromBlock(block []byte) string {
	for _, line := range strings.Split(string(block), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			return extractAnthropicTextDelta(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return ""
}

func quarantinedBlockBytes(blocks [][]byte) int {
	total := 0
	for _, block := range blocks {
		total += len(block)
	}
	return total
}

func detectAnthropicNonStreamingAbnormalOutput(body []byte, cfg *config.Config) abnormalOutputDetection {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return abnormalOutputDetection{}
	}
	detector := newAnthropicAbnormalOutputDetectorForConfig(cfg)
	for _, block := range response.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		if detected := detector.ObserveText(block.Text + "\n"); detected.Triggered {
			return detected
		}
	}
	return detector.Flush()
}

func abnormalOutputIsolationKey(userID, groupID int64, model string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(model), " "))
	sum := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%d:%d:%x", userID, groupID, sum[:8])
}

func (s *GatewayService) RecordAbnormalOutputIsolation(userID int64, groupID *int64, model string, accountID int64) {
	if s == nil || userID <= 0 || accountID <= 0 || strings.TrimSpace(model) == "" {
		return
	}
	s.abnormalOutputIsolationMu.Lock()
	defer s.abnormalOutputIsolationMu.Unlock()
	if s.abnormalOutputIsolationCache == nil {
		s.abnormalOutputIsolationCache = gocache.New(10*time.Minute, time.Minute)
	}
	ttl := 10 * time.Minute
	if s.cfg != nil && s.cfg.Gateway.AbnormalOutputAccountIsolationSeconds > 0 {
		ttl = time.Duration(s.cfg.Gateway.AbnormalOutputAccountIsolationSeconds) * time.Second
	}
	key := abnormalOutputIsolationKey(userID, derefGroupID(groupID), model)
	var isolated map[int64]struct{}
	if value, found := s.abnormalOutputIsolationCache.Get(key); found {
		isolated, _ = value.(map[int64]struct{})
	}
	copySet := make(map[int64]struct{}, len(isolated)+1)
	for id := range isolated {
		copySet[id] = struct{}{}
	}
	copySet[accountID] = struct{}{}
	s.abnormalOutputIsolationCache.Set(key, copySet, ttl)
}

func (s *GatewayService) AbnormalOutputIsolatedAccounts(userID int64, groupID *int64, model string) map[int64]struct{} {
	result := make(map[int64]struct{})
	if s == nil || userID <= 0 || strings.TrimSpace(model) == "" {
		return result
	}
	s.abnormalOutputIsolationMu.Lock()
	defer s.abnormalOutputIsolationMu.Unlock()
	if s.abnormalOutputIsolationCache == nil {
		return result
	}
	value, found := s.abnormalOutputIsolationCache.Get(abnormalOutputIsolationKey(userID, derefGroupID(groupID), model))
	if !found {
		return result
	}
	isolated, _ := value.(map[int64]struct{})
	for id := range isolated {
		result[id] = struct{}{}
	}
	return result
}
