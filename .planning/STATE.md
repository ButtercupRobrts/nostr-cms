# STATE.md — Nostr-CMS Upstream Project

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-01)

**Core value:** Buttercup.exe.xyz remains fully functional and unchanged while its improvements are contributed to bitkarrot/nostr-cms as reviewable, secure, and incrementally mergeable PRs.
**Current focus:** Phase 4 — Upstream Restructuring & Security Fixes

## Current Position

- **Phase**: 4 (Upstream Restructuring & Security Fixes)
- **Step**: 04-01/02/07 done (stopgap live, #16 closed, GHSA filed; swarm PRs #18 + #19 open); 04-04 partial (nostr-cms PR #81 open; community-zap gated)
- **Next action**: Await bitkarrot replies (DM sent — advisory ack, NIP-98 sign-off, zap-stats disposition, NIP-86); NIP-46 follow-up is a separate future PR

## Reality Check (2026-09-12)

- PRs #78/#79 MERGED to bitkarrot/nostr-cms; PR #16 (swarm) SUPERSEDED — most content already on `zeabur-dashboard` via other merges
- **Live vuln**: `/api/dashboard` + `/api/admin` session is forgeable (cookie = bare pubkey); confirmed on buttercup AND swarm.hivetalk.org — Phase 0 stopgap pending user decision
- Corrected analysis in `.planning/PR-RESTRUCTURING-REPORT.md` (Rev 3) — supersedes all earlier assumptions

## Active Decisions

1. **Swarm PRs base**: `zeabur-dashboard` (NOT beeswax — 73 behind). nostr-cms PRs base: `upstream/main` @ `dadc676`
2. **NIP-86 is additive only** — cannot do names CRUD; AdminRelayAccess/useNostrJsonUsers stay REST
3. **Recurring posts need no Go work** — CMS pre-signs series (459 repeat-tagged posts in prod)
4. **Deployment safety**: buttercup untouched except optional Phase 0 nginx stopgap (user's call); redeploy only after upstream merges, with explicit delta acceptance

## Blockers

- User decision: stopgap option A/B/C (04-01)
- bitkarrot answers: community-zap disposition, NIP-98 breaking-change ack, NIP-86 interest (gates 04-03/04/05)

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

### Sanitization (completed 2026-09-02)
- 4 source files sanitized: CreateEventDialog.tsx, AdminMedia.tsx, MediaSelectorDialog.tsx, AdminEvents.tsx
- Replaced hardcoded `buttercup.exe.xyz` relay URL with env-driven `getDefaultRelayUrl()`
- Removed domain-specific relay checks
- No secrets, API keys, or personal data in any PR
- npub in NoteContent.test.tsx verified as test fixture (not real user)

## Metrics

- Phases completed: 3/4 (Phase 4 review pending on new PRs)
- PRs created: 3 (fresh submission)
- PRs closed: 7 (#71-#77, superseded)
- tsc --noEmit: 0 errors (PR A and PR B)
- go build: 0 errors (PR C)
- Tests: 27 passed, 6 test files (PR B)

## Fresh PRs (2026-09-02)

| PR | Repo | Title | Files | Lines | Status |
|----|------|-------|-------|-------|--------|
| #78 | bitkarrot/nostr-cms | Core utilities, bugfixes, and config updates | 19 | +1,385/-368 | Open |
| #20 | ButtercupRobrts/nostr-cms | Blossom media, calendar, zaplytics, repost, mentions, settings, admin features | 98 | +19,779/-3,088 | Open (stacked on #78) |
| #2 | bitkarrot/swarm | Scheduler, zap analytics dashboard, and dashboard templates | 21 | +7,547/-386 | Open |

**Merge order**: #78 (PR A) → #20 (PR B, retarget to main after A merges) → #2 (PR C, independent)

## Old PRs (superseded 2026-09-02)

| PR | Branch | Title | Status |
|----|--------|-------|--------|
| #71 | pr-01-foundation | Foundation utilities and admin roles cleanup | Closed |
| #72 | pr-02-blossom-media | Blossom media utilities and client-side processing | Closed |
| #73 | pr-03-relay-explorer | Relay explorer admin page and activity dashboard | Closed |
| #74 | pr-04-calendar-events | Calendar, events, share-as-note, and repost scheduling | Closed |
| #75 | pr-06-zaplytics | Community zap analytics (Zaplytics) dashboard | Closed |
| #76 | pr-07-settings-content | Settings modularization, admin content, and public pages | Closed |
| #77 | pr-08-follow-backup | Follow list backup, recovery, and relay archive | Closed |

## Last Known Good State

- VM on `main` branch at 2ed6676 (upstream base)
- Deployed dist in nostr-cms/dist/ (Aug 7, 2026) — untouched
- Working branches: `pr-a-frontend` (PR A), `pr-b-features` (PR B), `pr-c-backend` (PR C)
- Planning artifacts in .planning/ including .planning/phases/03-fresh-prs/03-01-PLAN.md

## Feature Completeness Audit (2026-09-02)

Compared source of truth (60c8059:nostr-cms/src/) against PR A + PR B:
- 97 files changed/new in source of truth vs upstream
- 93 files in PR B diff (vs PR A) — all 97 source-of-truth changes accounted for
- 4 files were accidentally reverted to upstream during earlier sanitization work:
  - AdminEvents.tsx, AdminMedia.tsx, CreateEventDialog.tsx, MediaSelectorDialog.tsx
  - **FIXED**: Restored from 60c8059 with buttercup.exe.xyz references sanitized
- 26 files had content drift (refactoring, improved types, sanitization) — all improvements, no feature loss
- 11 extra files in PRs not in source of truth (EngagementStats, NostrEventEmbed, FollowBackup, etc.) — refactored extracts and new features, kept
- streamUpload function restored to mediaProcessing.ts (was removed during refactoring)
- AdminMedia.tsx adapted to use StrippedImage return type from improved stripImageMetadata
- AdminEvents.tsx: added missing sig field for RepostTarget
- AppProvider.tsx migration logic fixed: primary→publisher, secondary→user (was no-op)

**Verification**: tsc --noEmit = 0 errors, vitest = 27/27 passed, no buttercup.exe.xyz references

---
*Last updated: 2026-09-02 after feature completeness audit and restoration*
