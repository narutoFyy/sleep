# Abnormal Output Failover Changelog

Date: 2026-07-20

## Scope

Added semantic repeated-output detection for Anthropic Messages streams and automatic same-model regular-account failover. Production port 8090 was not changed; validation ran on port 8084.

## Behavior

- Detects the same normalized line repeated 6 times or the same alphanumeric word repeated 8 times within 5 seconds.
- Ignores metadata, ping, tool/structured output, thinking/reasoning output, punctuation-only units and fenced code.
- Holds the first 10 text units, up to 8 KiB or 1.5 seconds, so an early loop can be discarded before it reaches the client.
- After normal text is visible, stops at confirmation, closes the old text block, and continues the same downstream SSE request on another regular account using bounded Anthropic assistant prefill.
- Suppresses the replacement stream envelope and preserves one logical message lifecycle. The client does not press Enter again.
- Isolates the abnormal account for the affected user, group and model for 10 minutes. This does not globally disable the account.
- Keeps the requested model name and original group pricing. The final usage record contains the replacement output and route audit failure kind `abnormal_output`.

## Verification

- Focused service and handler tests passed.
- Full backend `go test ./... -count=1` passed under Go 1.26.4.
- Isolated 8084 E2E passed for early repetition, late continuation, valid SSE block/message lifecycle, one usage record, route audit and user-scoped isolation.
- Final test image: `sub2api-shitou:abnormal-output-8084-final-20260720`.
- 8084 container: `sub2api-abnormal-8084`, healthy.
- Production container `sub2api` on 8090 remained unchanged and healthy.

## Rollback

The single pre-change source archive is:

`/root/cyproject/backups/shitoutk-before-abnormal-output-20260720.tar.gz`

SHA-256:

`72e68a0a2ba9dc176820a6610019f7a516aabd40ba44584f096a3115e2bf9a75`
