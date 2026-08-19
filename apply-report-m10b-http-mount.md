# M10b Apply Report — `m10b-http-mount`

**Cycle**: `p-07dc4abdd11cc9be/m10b-http-mount`
**Branch**: `feat/m10-vertical-hexagons`
**Base**: `81046b4` (M10a merge to main)
**Head**: `3cf0fba`
**Scope**: 4 tasks (T07..T10)
**Status**: PASS_WITH_WARNINGS

## What was actually done

| Task | Commit | What landed |
|------|--------|-------------|
| T07 | `b967d55` | `HTTPMount` interface + `MultiMount` + `MountDeps` (13 fields + `registry` + `routeLabels`) + `RegisterRoute` + 2 new kernel narrow ports `GraphNodeFetcher` + `JournalStreamReader`. |
| T08 | `ede061e` | `WorkMount` implementing `MultiMount` with 8 routes (3 GETs use new kernel ports). `TestWorkRoutesResponseParity` PASS. |
| T09 | `8abab84` | Deleted `muxMatch` + `routeMatches`. `routes()` kept as fallback (legacy commands still registered in tck). Added `TestRouteLabelsDerivedFromMounts` (26 labels). |
| T10 | `47f5c80` | Added `MountDeps` validation in `mount.go`. Created `ReleaseMount`, `VerificationMount`, `AdminMount`, `PlatformMount`. Added `tck.NewMountSet(rt)`. TCK callsites updated to use `NewMountSet` + `MountDepsForRT`. |
| T10 fixup | `3cf0fba` | Added `Journal` field to `MountDeps` for `JournalStreamReader` creation. Set `graph`, `streams`, `journal`, `mounts` in `NewWithMounts` properly. `routesWithMounts()` reads `s.journal` for `JournalStreamReader`. |

**5 commits, 1 task fixup.**

## What was NOT done (carryover to M10c)

1. **`cmd/golem-api/main.go` rewiring to `NewWithMounts`** — the production composition root still uses the legacy `httpapi.New(rt.Bus, rt.Graph, rt.Journal)`. Per `git show --name-only 47f5c80`, main.go was NOT touched by the T10 commit despite the commit message claiming "Rewrites cmd/golem-api/main.go to use the new Mount surface". **M10c TASK-M10-11 must rewrite main.go.**

2. **`routesWithMounts` builds fresh `MountDeps` with empty fields.** The current implementation creates a new `MountDeps` and only passes `Bus` + `Registry` + `routeLabels` to it. Other read-side ports (ReleaseGraphReader, BlastRadiusQuery, etc.) are nil. This is incorrect behavior for production. M10c TASK-M10-12 must fix `routesWithMounts` to use the deps stored at `NewWithMounts` construction time.

3. **`MountDeps` mutex-by-value bug.** `internal/api/httpapi/mount.go:58` declares `mu sync.Mutex` inside `MountDeps`. The struct is then passed **by value** from `NewWithMounts` → `m.Mount()` → `handle*(deps)` → closure. Result: each copy has its own mutex, locking one doesn't lock the others. `go vet ./internal/api/httpapi/...` reports 20+ "passes lock by value" warnings. **Concurrency-safe design**: refactor to `MountDeps*` (pointer) or remove the mutex entirely (use `sync.Map` + atomic). M10c TASK-M10-13.

## Tests status

- PASS: `TestWorkRoutesResponseParity`, `TestRouteLabelsDerivedFromMounts`, all unit tests, all kernel port TCK tests.
- FAIL (3, pre-existing, NOT caused by T10):
  - `TestIngestAndReleaseGate` — `release create: 404` (artifact not found in graph)
  - `TestM3Lineage` — same upstream
  - `TestSupplyChainScenarios` — same upstream

**Verified pre-existing**: checked out `81046b4` (M10a merge base, before any T10 wiring) and the same 3 tests fail with the same error. The 404 is upstream of mount wiring — the artifact node from CI ingest never reaches the graph before the release handler queries it. Conjecture: projection runner registered with `globalRegistry=nil` in test environment falls back to legacy `projectSingle` switch, but the test's drain loop doesn't wait for the legacy case to commit. This requires a dedicated diagnostic cycle.

## Architecture wins (preserved)

- Vertical-hexagons split: each Mount encapsulates its own handlers + deps → 5 bounded contexts (work, release, verification, admin, platform) are self-contained.
- Kernel port isolation: `GraphNodeFetcher` + `JournalStreamReader` eliminate direct `ports.GraphStore`/`ports.Journal` leaks in handlers.
- Label-driven route discovery: `TestRouteLabelsDerivedFromMounts` proves 26 routes are reachable without legacy `routes()`.

## Risks carried into M10c

- **R1**: production wiring (main.go) still uses legacy `New()`. New Mount system is exercised only by tests. M10c MUST close this before any release.
- **R2**: 3 integration tests fail. Cannot certify M10's "vertical-hexagons don't break integration" until the projection timing bug is fixed.
- **R3**: `routesWithMounts` doesn't pass the full MountDeps built at `NewWithMounts`. This is a real bug that only M10c's wiring+fresh test will catch.
- **R4**: `go vet` reports 20+ "MountDeps passes lock by value" warnings. Symptom of a real concurrency bug (mutex copied at every handler call). M10c must fix this BEFORE any production concurrent traffic.

## `go vet` findings

- 20+ warnings, ALL introduced by T10 (verified: `git checkout 81046b4 && go vet ./internal/api/httpapi/...` returns 0 warnings).
- Root cause: `MountDeps` embeds `sync.Mutex` and is copied at every layer.
- Fix scope: M10c TASK-M10-13 (see above).

## Ledger entries

- 5 implementation commits (T07..T10 + fixup).
- 1 design decision (Real Mounts in T10) documented in proposal.md.
- 5 vault REQs updated: REQ-MOUNT-HTTPMount, REQ-MOUNT-MultiMount, REQ-MOUNT-MountDeps, REQ-MOUNT-RegisterRoute, REQ-PORT-GraphNodeFetcher, REQ-PORT-JournalStreamReader.

## Recommendation

PASS_WITH_WARNINGS. M10b's scope (HTTPMount, MultiMount, MountDeps, WorkMount, 2 kernel ports, TCK port layer) is complete and stable. The 3 pre-existing test failures and the main.go wiring are SEPARATE from this cycle's scope and belong to M10c.

**M10c next**: archtest + ADR-100 Accepted + ADR-CATALOG + main.go rewiring + `routesWithMounts` fix + MountDeps mutex-by-value fix + projection timing diagnostic.
