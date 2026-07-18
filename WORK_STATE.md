# Work State

## Status

`complete`

## Current task

All planned tasks are complete. The repaired candidate remains available on port 8084; production port 8090 was not changed.

## Rules

- Execute one task at a time and verify it before starting the next task.
- Preserve the production service on port 8090 throughout this work.
- Deploy the changed build only on port 8084.
- Keep exactly one pre-change source archive for this work.
- Do not infer standby model mappings; administrators must configure them explicitly.
- Do not allow standby routing across provider families.

## State Machine

`pending -> ready -> assigned -> implementing -> self_check -> main_verify -> done`

`main_verify -> needs_fix -> assigned`

## Tasks

| ID | Task | Status | Depends on | Allowed write scope | Verification |
| --- | --- | --- | --- | --- | --- |
| P-001 | Backup and TODO | done | none | `WORK_STATE.md`, `docs/FAILOVER_STANDBY_TODO.md`; server backup directory | Archive integrity, checksum, document review |
| P-002 | Remove the tg-message deployment | done | P-001 | tg-message container/image, `/etc/nginx/conf.d/tg-message-8086.conf`, `/usr/share/nginx/html/tg-message-8086` | Ports 8086/8087 closed; Nginx config valid; unrelated services healthy |
| T-001 | Standby membership data model and migration | done | P-002 | account-group schema, generated Ent code, account-group service/API, migrations, focused tests | Schema generation and backend tests |
| T-002 | Account health and group circuit breaker | done | T-001 | scheduler/health services, cache/state helpers, focused tests | Unit and integration tests for all-primary failure and recovery |
| T-003 | Transparent pre-content request failover | done | T-002 | gateway failover/stream handlers and focused tests | Streaming/non-streaming failover tests and 100-second deadline tests |
| T-004 | Group standby scheduler and model mapping | done | T-003 | account selection, compatibility checks, sticky routing, focused tests | Provider isolation, mapping, capacity and recovery tests |
| T-005 | Billing, audit and observability | done | T-004 | usage records, logs/metrics and focused tests | Original-model pricing and route audit tests |
| T-006 | Admin standby configuration UI | done | T-001, T-004 | group/account admin API and `GroupsView.vue` related frontend modules | Typecheck, frontend tests and browser workflow |
| T-007 | Full verification and port-8084 deployment | done | T-005, T-006 | tests/configuration and a separate 8084 deployment | Full test sweep, health checks, port/process inspection |
| T-008 | OpenAI live failover repair and retest | done | T-007 | OpenAI Chat Completions transport handling, standby entry predicate, focused tests and isolated 8084 deployment | Unit tests plus real `cs` to `kun` transport-failure exercise |

## Active Task

None. All planned tasks passed main verification.

## Latest Completion Evidence

### T-008

- Purpose: fix two gaps found by real-key verification: TCP/DNS/TLS failures committed a 502 before account switching, and a new request did not enter standby when all primary accounts were globally unavailable.
- Exact work: route both Chat Completions upstream transport paths through the existing OpenAI transport-failover helper; permit standby selection for `ErrNoAvailableAccounts`; add focused regressions; enable pool mode with zero same-account retries for the two 8084 relay accounts; rebuild and re-run real transport failure against `cs` with `kun` as standby.
- Non-goals: no 8090 deployment or production data change.
- Completion evidence: both `cs` and `kun` have `pool_mode=true` and `pool_mode_retry_count=0`; real TCP refusal on `cs` failed over to `kun` without committing the intermediate 502; a subsequent request while `cs` was unavailable entered standby directly; after restoring `cs`, its real upstream 502 also failed over to `kun`; full `go test ./...` passed on the server with Go 1.26.4.

## File Access Requests

- 2026-07-18: Main-agent path audit corrected the planned nonexistent `backend/internal/service/openai_chat_completions.go` path and granted T-004 write access to the actual handler/service OpenAI chat completion files listed above.
- 2026-07-18: Child requested write access to `backend/cmd/server/wire_gen.go`; granted after confirming the checked-in generated server constructor is required for runtime `PrimaryHealthService` injection into both gateway services.
- 2026-07-18: Child checkpoint requested six existing test files for mechanical constructor compatibility; granted within T-004 focused-test scope. T-004 implementation is staged as service routing core verification followed by HTTP handler integration.
- 2026-07-18: Child produced the T-004 service-core patch but did not return from self-check after repeated bounded waits and an explicit finalize request; child session closed while running. Main agent took ownership of formatting, server-side Go verification, and any narrow repair without discarding the patch.
- 2026-07-18: Main agent formatted the T-004 service core with Go 1.26.4 and passed standby/health focused service tests plus compile-only checks for `internal/handler` and `cmd/server`. Service-core slice accepted; HTTP handler integration remains active under T-004.
- 2026-07-18: Child requested `openai_gateway_chat_completions_raw.go`; granted only to read the preserved client model from context so raw API-key Chat Completions does not expose the mapped standby model.
- 2026-07-18: Main agent found and repaired group-scoped legacy Account payload compatibility, then passed focused regressions, full `internal/handler`, full `internal/service`, and compile-only `cmd/server`; `T-004 implementing -> self_check -> main_verify -> done`.
- 2026-07-18: `T-005 pending -> ready -> assigned`.
- 2026-07-18: Assigned `T-005` to child agent; `assigned -> implementing`.
- 2026-07-18: T-005 checkpoint accepted a two-slice implementation: first typed in-memory route audit plus original-model standby pricing and focused compile/tests; then additive usage-log persistence/migration after main verification.
- 2026-07-18: Main agent formatted and verified the T-005 in-memory/billing slice with focused unit-tag tests, full `internal/handler`, full `internal/service`, and `cmd/server` compile. Persistence slice activated.
- 2026-07-18: T-005 persistence passed Ent/DTO/service/server checks, full repository tests, real migration idempotency and standby usage round-trip in PostgreSQL; `T-005 implementing -> self_check -> main_verify -> done`.
- 2026-07-18: `T-006 pending -> ready -> assigned`.
- 2026-07-18: Assigned `T-006` to child agent; `assigned -> implementing`.
- 2026-07-18: Child returned a partial T-006 helper/API patch; main agent completed the group-owned editor, account-creation defaults, translations and focused tests.
- 2026-07-18: T-006 passed `vue-tsc`, focused Vitest (8 tests), changed-file ESLint and production Vite build. Full Vitest exposed 15 unrelated existing failures in usage/chart/image-billing/page-size tests; no failing file overlaps T-006.
- 2026-07-18: `T-006 implementing -> self_check -> main_verify -> done`.
- 2026-07-18: `T-007 pending -> ready -> implementing`.

## Transition Log

- 2026-07-18: Plan accepted in `delegated-state`, linear topology.
- 2026-07-18: `P-001 pending -> implementing`.
- 2026-07-18: Created and verified `/root/cyproject/backups/shitoutk-before-standby-20260718.tar.gz`.
- 2026-07-18: Added and reviewed `docs/FAILOVER_STANDBY_TODO.md`; `P-001 implementing -> main_verify -> done`.
- 2026-07-18: `P-002 pending -> implementing`.
- 2026-07-18: Removed the tg-message container/image, static site and Nginx server block; verified 8086/8087 closed and unchanged 8090 container health; `P-002 implementing -> main_verify -> done`.
- 2026-07-18: `T-001 pending -> ready -> assigned`.
- 2026-07-18: Assigned `T-001` to child agent; `assigned -> implementing`.
- 2026-07-18: Child self-check completed with an environment limitation (Go unavailable locally); `implementing -> self_check -> main_verify` for server-side generation and tests.
- 2026-07-18: Generated Ent code with Go 1.26.4, fixed nested request validation, and passed focused service/repository/admin/DTO tests; `T-001 main_verify -> done`.
- 2026-07-18: `T-002 pending -> ready -> assigned`.
- 2026-07-18: Assigned `T-002` to child agent; `assigned -> implementing`.
- 2026-07-18: Added Redis-backed per-group/model/account health evidence, 30-second circuit, 60-second probe leases and two-success recovery; passed unit-tagged and full service/repository tests; `T-002 implementing -> self_check -> main_verify -> done`.
- 2026-07-18: `T-003 pending -> ready -> assigned`.
- 2026-07-18: Assigned `T-003` to child agent; `assigned -> implementing`.
- 2026-07-18: Synced the final formatted `gateway_service.go` from the server and confirmed focused plus full handler/service tests passed; `T-003 implementing -> self_check -> main_verify -> done`.
- 2026-07-18: `T-004 pending -> ready -> assigned`.
- 2026-07-18: T-007 candidate image built and deployed as `sub2api-standby-8084` on isolated PostgreSQL/Redis infrastructure; migrations 151/152 applied.
- 2026-07-18: Verified standby configuration save/reload and mobile layout, including persisted `standby`, enabled, priority `3` and explicit Claude model mapping.
- 2026-07-18: Rechecked 8084/8090 health and container/network separation; 8086/8087 remain closed and the single source backup remains intact.
- 2026-07-18: `T-007 implementing -> self_check -> main_verify -> done`; work status set to `complete`.
- 2026-07-18: Live 8084 testing found OpenAI Chat Completions transport errors returned 502 before failover and globally unschedulable primaries did not re-enter standby; `T-008 implementing`.
- 2026-07-18: Enabled pool mode with zero same-account retries for 8084 accounts `cs` and `kun`; account URLs and keys were left unchanged.
- 2026-07-18: Verified real-key standby routing for injected connection refusal, already-unavailable primary entry, and a real restored-primary upstream 502; all client requests completed through `kun` with standby audit records.
- 2026-07-18: Full backend `go test ./...` passed; `T-008 implementing -> self_check -> main_verify -> done`; work status set to `complete`.
- 2026-07-18: Assigned `T-004` to child agent; `assigned -> implementing`.
