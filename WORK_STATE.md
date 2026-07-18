# Work State

## Status

`complete`

## Current task

All planned tasks are complete. The isolated candidate remains available on port 8084; production port 8090 was not changed.

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

## Active Task

### T-007

- Purpose: prove the complete failover and standby change against isolated infrastructure without changing production on port 8090.
- Exact work: sync the reviewed source to `/root/cyproject/shitoutk`; run backend unit/integration tests, migration checks, frontend verification and a Docker build; create a dedicated database in `sub2api-test-db`; run the candidate as a separate container on port 8084 attached only to the test PostgreSQL/Redis network; inspect health, schema, admin UI and standby configuration workflow; compare the final 8090 container ID, image and start time with the recorded fingerprint.
- Read/write scope: current repository changes, server test database/Redis, one new candidate image/container/database and `WORK_STATE.md`; no production database, Redis or 8090 container mutation.
- Non-goals: no rollout to 8090, no production migration, no Nginx cutover and no unrelated test repair.
- Acceptance: full focused backend/frontend checks pass; migrations 151/152 are applied to the isolated database; 8084 health and admin UI are usable at desktop/mobile widths; 8084 and 8090 are separate containers; 8090 fingerprint and health remain unchanged.
- Focused verification: Go package/integration tests, frontend typecheck/Vitest/build, Docker build, HTTP health/API checks, PostgreSQL migration inspection, browser screenshots and final Docker/port inspection.
- Compatibility: use `sub2api-test-db`, `sub2api-test-redis` and a dedicated database; remove no long-running test infrastructure.
- Completion evidence: backend full tests and PostgreSQL migration/integration tests passed; frontend typecheck, focused Vitest and production build passed; desktop/mobile admin workflow and persisted standby values were verified; 8084 is healthy on `sub2api-test-net`; 8090 retained container `49b39f96db32e69aca3a21bd3a68cdc69e03f575b4f15ba1e0139d46757a4a3e`, image `sub2api-shitou:shitoutk-prod-20260712-005914` and start time `2026-07-11T17:16:40.931017036Z`.
- Known unrelated test state: the full frontend Vitest suite has 15 pre-existing failures in usage/chart/image-billing/page-size tests; all T-006 focused tests pass.

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
- 2026-07-18: Assigned `T-004` to child agent; `assigned -> implementing`.
