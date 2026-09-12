# Phase 4: Upstream Restructuring & Security Fixes - Context

**Gathered:** 2026-09-12
**Status:** Ready for planning
**Source:** `.planning/PR-RESTRUCTURING-REPORT.md` (Rev 3 — verified against `zeabur-dashboard`, not `beeswax`)

## Phase Boundary

Close the live unauthenticated admin-API hole on buttercup.exe.xyz, retire superseded HiveTalk/swarm PR #16, and ship the focused follow-up PRs defined in the restructuring report. buttercup stays fully functional for public use throughout; no redeploy until upstream merges land.

## Implementation Decisions

### Verified state (no assumptions — all checked against code/API)
- **D-01:** PR #16 is *superseded*, not merely rejected — `scheduler.go` is byte-identical on `zeabur-dashboard` (md5 `d8862e57…`), and the dashboard/admin REST API, `NostrLogin.js`, buggy `npubToHex`, and rate limiters all already landed upstream. Close it as superseded.
- **D-02:** Swarm PRs branch from `zeabur-dashboard` @ `a30fb61` (repo default + PR #16 base). NOT `beeswax` (73 commits behind). nostr-cms PRs branch from `upstream/main` @ `dadc676`.
- **D-03:** The reviewer's "no need for REST" was the user's paraphrase, not review text. Do not remove REST paths or `vercel.json` without bitkarrot's explicit say — they deployed that coupling themselves.
- **D-04:** NIP-86 cannot replace `AdminRelayAccess`/`useNostrJsonUsers` (names CRUD + `"_"` owner transfer have no NIP-86 method; swarm's allowlist IS the names map — no separate pubkey allowlist exists). NIP-86 is additive only: allow/ban kinds + IPs.
- **D-05:** Recurring posts need no Go changes — the CMS pre-signs the whole series (459 of 533 prod posts carry `repeat_*` tags). Only scheduler fix needed is `go s.save()` race (`scheduler.go:169`).

### Security (live vuln — verified, no exploit performed)
- **D-06:** `dashboard_session` cookie value IS the pubkey — forgeable without calling `/login`. Handler mounted at BOTH `/api/dashboard/` and `/api/admin/` prefixes. Nostr.json names map = relay write allowlist, so forged admin → arbitrary team membership + relay write access.
- **D-07:** Confirmed live on `swarm.hivetalk.org` too (GET → 405 = handler exists). Disclosure owed to bitkarrot.
- **D-08:** Stopgap choice is a user decision gate (full deny vs surgical GET-only vs accept-risk). `/api/scheduler/*` stays open in all cases (NIP-98 server-side verified, `scheduler.go:555+`).

### Upstream gates (block specific plans until answered)
- **D-09:** bitkarrot decides: community zap stats (delete `useCommunityZapStats` / client-side aggregation / new fixed `zap_stats` PR). Backend endpoint `/dashboard/zap-stats` never landed upstream — the hook 404s today.
- **D-10:** NIP-98 login fix is a *breaking change* to the bundled dashboard UI (`dashboard.html`/`dashboard_view.html`/`NostrLogin.js` must produce signed kind-27235 requests with `u` + `payload` tags). Needs bitkarrot sign-off — it breaks `swarm.hivetalk.org`'s dashboard too.
- **D-11:** NIP-86 enablement is optional new work — ask before building.

### Things that are NOT needed (explicitly out of scope)
- No client-side scheduler rewrite — server-side scheduler already upstream and prod-format-compatible.
- No `vercel.json` removal, no `AdminExplorer` REST-path removal (maintainer built the WS fallback deliberately).
- No NIP-86 replacement of names CRUD — stays on REST.
- No zap_stats.go resubmission unless bitkarrot asks (with config flag, no external `db.SaveEvent`, go-nostr NIP-57 utils, tests).

## Specific Ideas

- `fetchWithNip98` (`useScheduledPosts.ts:26`) is the auth pattern to copy for the dashboard fix — but it signs kind 27235 with `u`+`method` only; the NIP-86 client hook must add the `payload` SHA-256 tag (khatru `HandleNIP86` requires it).
- khatru v0.15.2 NIP-86 is auto-registered behind `corsMiddleware` — browser clients work cross-origin; `u` tag must equal the request URL as the server sees it (nginx already sets `Host`).

## Canonical References

- `.planning/PR-RESTRUCTURING-REPORT.md` — full analysis + pipeline (Rev 3)
- `swarm/` Go source lives on `pr-c-backend` branch at repo root; deployed binary at `swarm/swarm-relay`
- `deploy-latest:nginx-meetup.conf` — repo copy of live nginx config (`/etc/nginx/sites-available/meetup-space`, root-owned)
- khatru NIP-86: `~/go/pkg/mod/github.com/fiatjaf/khatru@v0.15.2/nip86.go`, `handlers.go:46`

## Open Questions Blocking Plans

1. Stopgap choice (A/B/C) — user's call (plan 04-01 has the decision gate).
2. bitkarrot: community-zap disposition; NIP-98 breaking-change ack; NIP-86 interest.
