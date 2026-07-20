# Abnormal Output Detection and Same-Model Failover

## Status

Implementation complete on the isolated 8084 test deployment. This document records the approved behavior and does not authorize production deployment.

## Incident

Account `8033` in group `满血claude` returned a valid HTTP 200/SSE response for user `76`, but the generated text repeated `course` until the provider output ceiling. Two requests ran for roughly 8.5 minutes and ended at exactly 64,000 output tokens. The gateway made one route attempt and did not duplicate the stream.

The failure is semantic, not an HTTP or SSE protocol failure.

## Goal

When a same-model regular account starts producing an objectively repeated text loop, stop the loop quickly and automatically continue the same user request on another eligible account. The user must not press Enter again or resubmit the request.

The client should remain on the same request and normally remain in its generating state. A short pause while the gateway cancels the bad upstream and starts the replacement is acceptable.

## Confirmed User-Visible Behavior

### Before any model text is committed

- Hold a small initial text buffer while the detector observes the first output.
- If the loop threshold is reached, discard the bad attempt completely.
- Retry the original request on another regular account supporting the same requested model and group.
- The client sees only the replacement account's answer.

### After normal text has already been committed

- Stop forwarding the repeated tail as soon as the detector confirms the anomaly.
- Cancel the bad upstream request and exclude that account from the current route.
- Keep the existing downstream SSE connection open.
- Send the original conversation and the already-visible normal answer to a replacement account using Anthropic assistant prefill semantics.
- Suppress the replacement stream's second message envelope and forward only compatible text deltas into the existing logical assistant response.
- The user may see a short pause and a minor wording transition, but does not need to resubmit.

This is continuation, not byte-for-byte reconstruction. Bytes already sent to a client cannot be retracted.

## Non-Goals

- Do not wait for 64,000 tokens or any output ceiling before detecting a loop.
- Do not classify an answer as abnormal based only on length, low diversity, usefulness or correctness.
- Do not inspect or persist full prompts or full responses for detection.
- Do not change the requested model name or group pricing.
- Do not use a standby account before eligible regular accounts are exhausted.
- Do not cross provider families when selecting a replacement or standby account.
- Do not splice a second response during an active tool call, structured JSON input stream or signed thinking block.
- Do not impose a new total timeout on legitimate long-running model output. The existing pre-content routing deadline remains separate.

## Detection Rules

The detector operates on assembled Anthropic visible `text_delta` content, not raw SSE frame count.

Initial V1 thresholds:

- The same normalized non-empty line appears 6 consecutive times within 5 seconds.
- Or the same normalized word of at least 3 alphanumeric characters appears 8 consecutive times within 5 seconds.
- Whitespace, line endings and case may be normalized for comparison.
- Punctuation-only units, empty deltas, ping frames, usage metadata, message metadata, tool events, thinking/reasoning deltas and code blocks are ignored.
- Before the first effective content is committed, the gateway quarantines at most 10 SSE text units, 8 KiB or 1.5 seconds of output. If the candidate breaks, the held units are released; if it reaches the confirmed threshold, the held attempt is discarded. After content is committed, detection runs on the live stream; the already-visible tail cannot be retracted, so the confirmed offending delta is stopped and continuation begins immediately.
- The detector must report a reason code, observed repeat count and whether client-visible content was already committed.

The detector must not use a generic low-diversity score as an independent trigger. Output length and the 64K incident ceiling are audit fields only.

## Stream Buffering and Failover State Machine

The existing `anthropicAttemptBuffer` and `PreContentTracker` remain the source of truth for pre-content commitment and routing deadlines. The new logic adds abnormal-output state around the existing Anthropic stream loop:

1. `observing`: parse protocol frames and observe visible text while holding the bounded initial buffer.
2. `committed`: release normal text and record the visible answer in a bounded continuation buffer.
3. `suspected`: hold the suspected repeated tail for a short bounded period.
4. `abnormal_precommit`: cancel upstream, discard all held content and return the existing pre-content failover error with reason `abnormal_output`.
5. `abnormal_postcommit`: cancel upstream, discard the repeated tail, select a replacement account and create a continuation request.
6. `continuing`: suppress the replacement stream envelope and forward only compatible content deltas into the original logical response.
7. `completed` or `failed`: emit one final logical stream termination or the existing stream error when no safe replacement remains.

The initial quarantine stays bounded at 10 text units or 8 KiB, with no more than 1.5 seconds of initial hold. Late detection does not buffer an entire response; only the already-visible text needed for the bounded continuation prefill is retained.

## Continuation Request

For a post-commit switch, the gateway must retain enough already-visible normal text to give the replacement account a stable breakpoint:

- Keep the latest normal answer text up to 256 KiB, preserving UTF-8 boundaries.
- Never include confirmed repeated tail content in the continuation context.
- Preserve the original conversation, requested model identity and relevant request options.
- Add an internal continuation instruction that the visible answer already contains the retained text and the replacement must continue from its end without repeating it.
- If the request is in a tool, structured JSON or signed thinking phase, do not synthesize a text continuation. End or retry through the protocol-safe existing path.

The replacement's `message_start`, initial content block and terminal events must not be passed through as a second logical response. The gateway owns the downstream envelope and must emit one consistent message lifecycle.

## Routing Order

- Exclude the abnormal account immediately for the current request.
- Clear the current user/model sticky binding so the failed account is not selected again.
- Try other eligible regular accounts in the same group and same provider/model family.
- If all eligible regular accounts fail, enter the existing group standby mapping.
- Standby mapping remains group-owned, explicitly configured and provider-family restricted.
- If a replacement account also triggers the detector, apply the same exclusion and continue through remaining eligible routes subject to the existing bounded failover policy.

## Account Isolation

- Record `abnormal_output` as a distinct health/failover reason.
- Isolate the abnormal account for the affected user/model/session scope for 10 minutes initially.
- Do not globally disable an account after one user's semantic anomaly.
- A later aggregate health policy may broaden isolation only after repeated evidence from multiple users; that is outside V1.

## Billing and Audit

- Create one user-facing usage record and one charge for the request.
- Preserve the original requested model and original group price.
- Do not charge the user for discarded repeated output or internal retry duplication.
- Record attempted account IDs, detector reason, threshold, detection time, committed-content state, discarded-tail size, continuation account, final route, standby usage and total request outcome.
- Do not store the full prompt or response. A short hash of the repeated unit may be recorded for diagnosis.

## Configuration

The first implementation uses centrally controlled safe defaults and a feature flag:

```text
abnormal_output_detection_enabled = true
abnormal_output_repeat_window_seconds = 5
abnormal_output_line_repeat_threshold = 6
abnormal_output_word_repeat_threshold = 8
abnormal_output_min_word_characters = 3
abnormal_output_precommit_buffer_bytes = 8192
abnormal_output_precommit_max_text_units = 10
abnormal_output_precommit_max_hold_ms = 1500
abnormal_output_account_isolation_seconds = 600
```

The feature flag must restore the existing forwarding behavior when disabled. Per-account overrides are deferred until false-positive measurements exist.

## Implementation Scope

Expected code areas:

- `backend/internal/service/anthropic_pre_content.go`: bounded text quarantine and detector-facing commitment state.
- `backend/internal/service/gateway_service.go`: Anthropic SSE event assembly, detection, cancellation, continuation request creation, and single-envelope stream forwarding.
- `backend/internal/handler/gateway_handler.go`: route retry/error classification, exclusion and sticky clearing for `abnormal_output`, and final usage handoff.
- Focused service/handler tests for detector, stream lifecycle, routing, continuation and billing audit.

The implementation must reuse the existing primary failover and standby scheduler instead of creating a parallel account-selection mechanism.

## Verification

- A mock HTTP 200 Anthropic stream repeating `course` is detected well before 64,000 tokens.
- Early repetition is discarded and the next regular same-model account returns the only visible answer.
- Late repetition stops at the confirmed threshold, the client connection remains open, and the replacement account continues without a second message envelope.
- The client never needs to press Enter or issue a second request.
- A second failed regular account is excluded and the configured group standby is used only after regular exhaustion.
- Normal prose, repeated list items, code blocks, tool events, thinking events and empty protocol frames do not trigger V1 detection.
- The downstream receives one logical `message_start` and one logical terminal sequence.
- Only one user usage/charge record is created with the original model and price.
- Sticky routing does not select the abnormal account again during isolation.
- Feature disablement restores existing stream behavior.
- Focused tests, relevant backend tests, image build and isolated 8084 health/routing checks pass.
- Port 8090 remains on its pre-change image and is not restarted.

## Rollback

Disable `abnormal_output_detection_enabled` first to restore forwarding behavior. If a code rollback is required, use the single verified pre-change archive:

`/root/cyproject/backups/shitoutk-before-abnormal-output-20260720.tar.gz`

Production replacement is not part of this implementation run; 8084 validation must pass before any separate production approval.
