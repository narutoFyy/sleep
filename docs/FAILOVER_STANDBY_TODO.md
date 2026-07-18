# Transparent Failover and Group Standby TODO

## Deployment boundaries

- [x] Back up the complete pre-change `/root/cyproject/shitoutk` source once.
- [x] Verify the backup archive and record its SHA-256 checksum.
- [x] Remove the confirmed tg-message deployment from ports 8086/8087, including its container, image, static files and Nginx configuration.
- [x] Leave the current production deployment on port 8090 unchanged.
- [x] Deploy the completed candidate only on port 8084.
- [x] Verify that 8084 and 8090 are separate containers and that 8090 remains healthy.

Backup created for this work:

```text
/root/cyproject/backups/shitoutk-before-standby-20260718.tar.gz
SHA-256: caf4a89cb91f37894f5d6f3f911bc681296664eb63fccd44725296a6bd5f99f8
```

## Data model and admin configuration

- [x] Extend an account-to-group membership with a `primary` or `standby` role.
- [x] Add per-group enabled state, standby priority and explicit request-model-to-upstream-model mappings.
- [x] Keep URL, key, proxy, concurrency and RPM settings on the reusable Account entity.
- [x] Allow one standby account to be reused by multiple groups.
- [x] Default every existing membership to `primary` without changing current routing.
- [x] Add a standby-account area to group administration for selecting or creating accounts and editing mappings.
- [x] Add connection and mapping-rule tests in the admin workflow.
- [x] Reject mappings across provider families: Claude to Claude only and GPT to GPT only.

## Primary-pool failover

- [x] Exclude a failed account for the remainder of the current request.
- [x] Retry another primary account while no effective model content has reached the client.
- [x] Treat SSE metadata and keepalive frames as pre-content so they do not block failover.
- [x] Preserve the SSE connection while switching accounts and hide intermediate upstream errors.
- [x] Cancel failed upstream requests and release concurrency reservations.
- [x] Apply one 100-second routing deadline from request receipt until the first effective model content.
- [x] Never retry transparently after effective model content has been sent.

## Health, circuit breaking and recovery

- [x] Record a real upstream failure result for each enabled primary account independently.
- [x] Open the group circuit only when every enabled primary account has its own failure evidence.
- [x] Do not open the circuit because of local concurrency saturation, queueing or queue timeout.
- [x] Keep the group circuit open for 30 seconds, then allow limited half-open primary probes.
- [x] Require two consecutive successful probes before restoring normal primary routing.
- [x] Probe system-automatically-out accounts every 60 seconds.
- [x] Never automatically restore an account disabled manually by an administrator.

## Standby routing

- [x] Enter standby immediately when the current request has exhausted all eligible primary accounts.
- [x] Bypass known-bad primaries while the group circuit is open.
- [x] Select standby accounts by enabled state, priority, provider compatibility and explicit model mapping.
- [x] Skip incompatible tool, Thinking or image rules and continue to the next configured standby rule.
- [x] Keep standby accounts out of normal balancing and long-lived sticky sessions.
- [x] Return new requests to the primary pool after recovery.
- [x] Enforce aggregate standby limits of 50 concurrent requests and 100 RPM, with no daily budget.

## Client contract, billing and audit

- [x] Preserve the originally requested model name in client-visible responses.
- [x] Charge against the original group/model price even when the actual standby model differs.
- [x] Record the actual account, actual model, mapping rule, standby level, attempts and failure reasons for administrators.
- [x] Ensure one client request creates one user-facing charge even when multiple upstream attempts incur internal cost.

## Verification

- [x] Cover streaming and non-streaming primary failover.
- [x] Cover connection errors, retryable status codes, first-token timeout and pre-content stream interruption.
- [x] Cover single-account failure without a group circuit, all-primary failure, 30-second half-open behavior and two-success recovery.
- [x] Cover standby provider isolation, explicit mappings, capability mismatch, limits, billing and audit records.
- [x] Run backend unit/integration tests, Ent generation checks, frontend typecheck/tests and production build.
- [x] Exercise the admin workflow and gateway behavior against the isolated 8084 deployment.
- [x] Confirm port 8090 has not restarted or changed image/configuration.

The focused frontend standby tests, typecheck and production build pass. The full
frontend Vitest suite still has 15 pre-existing failures in unrelated usage,
chart, image-billing and persisted-page-size tests.
