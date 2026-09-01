# STATE.md — Nostr-CMS Upstream Project

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-01)

**Core value:** Buttercup.exe.xyz remains fully functional and unchanged while its improvements are contributed to bitkarrot/nostr-cms as reviewable, secure, and incrementally mergeable PRs.
**Current focus:** Phase 1 — Source Preparation

## Current Position

- **Phase**: 1 (Source Preparation)
- **Step**: Planning complete, ready to execute
- **Next action**: Execute Phase 1 — verify source of truth, create clean working branch, close broken PRs

## Active Decisions

1. **Source of truth**: `github/main` (ButtercupRobrts/NostrCMSbtcp) at commit 60c8059 — verified as the complete pre-split working branch with all deployed features, compiles cleanly
2. **PR strategy**: Stack-on-fork — each PR targets the previous PR's branch in ButtercupRobrts/nostr-cms fork
3. **Supersede broken PRs**: The 19 existing PRs (#32-#50) will be closed and replaced with a clean re-split
4. **Deployment safety**: VM main stays at 2ed6676, deployed dist (Aug 7) is immutable

## Blockers

None currently.

## Key Findings

### Source of Truth (verified 2026-09-01)
- `github/main` at 60c8059 (Aug 2, 2026) is the original working repo
- 191 commits, 366 files changed, 34,510 insertions since upstream base (2ed6676)
- nostr-cms app code is in `nostr-cms/` subdirectory
- 255 files in nostr-cms/src/, 49,747 insertions
- **Compiles cleanly** (tsc --noEmit = 0 errors)
- Contains ALL features deployed on buttercup.exe.xyz (verified via dist bundle analysis)
- Has complete AdminEvents.tsx with both RepostDialog AND ShareAsNoteDialog (code the 19 PRs lost)

### Deployed buttercup.exe.xyz (verified 2026-09-01)
- Static files in nostr-cms/dist/ (built Aug 7, 2026)
- Contains: blossom media, zaplytics, share-as-note, relay explorer, repost, calendar, NIP-53 live rooms, follow backup, mention resolution, activity card
- Served directly by exe.dev HTTPS proxy (no running server process)
- Vite build outputs to dist/ (not nostr-cms/dist/), so rebuilds cannot overwrite deployment

### Broken PRs (from restacking report)
- 19 PRs (#32-#50) on fork, all from base 2ed6676
- 15 of 19 fail tsc --noEmit
- 4 have class A lost code (empty JSX conditionals in AdminEvents.tsx and AdminBlog.tsx)
- Class C: adminRoles migration duplicated in #37 and #48
- Class D: genuine defects (hideCloseButton, centerMedia/truncateEmbeds, autoHarvest24h, FALLBACK_DISCOVERY_RELAYS)

## Metrics

- Phases completed: 0/4
- Requirements covered: 0/24
- PRs designed: 0
- PRs reviewed: 0

## Last Known Good State

- VM on `main` branch at 2ed6676 (upstream base)
- Deployed dist in nostr-cms/dist/ (Aug 7, 2026) — untouched
- GSD Core v1.12.0 installed in .claude/
- Planning artifacts created in .planning/

---
*Last updated: 2026-09-01 after project initialization*
