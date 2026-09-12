# 04-01 Summary: Security stopgap + upstream coordination

**Date:** 2026-09-12
**Status:** Partially complete (stopgap applied, PR closed; bitkarrot gate answers pending)

## Decision
- Stopgap: **Option A (full deny)** — user chose after confirming CMS login/post/schedule/upload/transcode all unaffected (verified: nostrify login, WS relay, `/api/scheduler/*` NIP-98, Blossom `/upload`, `/process-video` — none route through the denied prefixes)

## Applied (verified live on 127.0.0.1:8000)
- `deny all` on `/api/dashboard/`, `/api/admin/`, `/dashboard`, `/backfill-owners` in `/etc/nginx/sites-available/meetup-space` (root-owned; sudo edit; `nginx -t` clean; reloaded)
- `/backfill-owners` added beyond original plan — it was the unauthenticated full-DB-scan endpoint, publicly proxied
- Probes: POST `/api/admin/users` w/ forged cookie → 403; GET w/ forged cookie → 403; POST `/api/dashboard/login` + `/api/admin/login` → 403; `/dashboard` → 403; `/backfill-owners` → 403
- Still working: `/` → 200; `/.well-known/nostr.json` → 200 (NIP-05 intact); `/api/scheduler/list` → 401 (alive, correctly rejecting unsigned); `/api/health` → 200

## Consequences (as designed)
- Dark until NIP-98 fix ships + redeploy: `/dashboard` page, My Activity card, community zap tab, Relay Access write UI
- Still working: entire public site, CMS login, posting, scheduling UI + 272 pending posts, media upload/transcode, event explorer (WS fallback)
- Member admin via ops channel: edit `swarm/public/.well-known/nostr.json` → restart swarm-relay

## Upstream coordination
- PR #16 on HiveTalk/swarm **CLOSED** as superseded; comment initially included public exploit detail — **edited** to a vague "dashboard-auth hardening" + pointer to the private advisory
- **Private vulnerability report filed**: GHSA-mf34-999w-2wm2 on HiveTalk/swarm (full technical details, mitigation, NIP-98 fix offer). Provenance correction surfaced during verification: the dashboard/admin API is bitkarrot's own code (upstream since Jan 2026, owner-only login); our branch's `isKnownTeamPubkey` broadened it to any team member. Both are forgeable.
- **Pending:** bitkarrot gate answers — (1) community-zap disposition, (2) NIP-98 breaking-change ack, (3) NIP-86 interest. Recommended: a direct Nostr DM heads-up pointing at the advisory (advisories get missed) — user's channel.

## Not done
- bitkarrot question message (user's channel — draft is in 04-01-PLAN.md task 4)
- Lift stopgap only when a build with real auth deploys (04-06 gate)
