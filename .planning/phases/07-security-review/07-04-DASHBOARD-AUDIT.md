# 07-04 — Threat model & authz audit: HiveTalk/dashboard (repo review)

**Date:** 2026-10-01 · **Method:** static review of cloned main, no live probing (not our deployment). Second pass on the swarm+nostr-cms stack where new evidence appears.

## 1. Trust boundaries (dashboard)

| # | Boundary | Notes |
|---|---|---|
| D1 | Browser → Astro API routes (`src/pages/api/**`, ~60 endpoints) | Own session (`hivetalk_session`) — **properly built**: challenge nonce + signed kind-27235 + `verifyEvent` + opaque `mintSession` token, domain tag, 5-min freshness (`src/pages/api/auth/login.ts`, `src/lib/session.ts:80-126`) |
| D2 | API routes → upstream relay host (`PUBLIC_HIVETALK_RELAY_HOST`) | Bearer token pass-through; actual authz lives upstream (kick/mute/moderate/stage proxies) |
| D3 | Browser → key custody | same nostrify localStorage issue (shared Finding 3) + `NostrCreateAccountDialog` raw `nostr-backup.txt` export |
| D4 | LiveKit room auth | guests = ephemeral per-room keys (`src/lib/livekit/ephemeralKeys.ts`, localStorage — disposable by design); authed = session/signed event (`tokenRequest.ts`) |
| D5 | Supabase edge functions (`supabase/edge/*`) | proxy layer; separate trust domain |
| D6 | localStorage | nsec + ephemeral keys + relay lists |

## 2. Entry points (untrusted input)

- ~60 Astro API routes: auth, billing, livekit (moderation/lobby/recordings), nostr-cache, events, scheduling, uploads
- Body JSON on all POSTs; Bearer header on livekit proxies; `signed_event` JSON on token/lobby paths
- nostr event content → rendered (see XSS finding)
- File upload: `.txt` nsec import (account creation/backup round-trip)
- Env: `PUBLIC_HIVETALK_RELAY_HOST`, session secret(s) for `mintSession` signing (`src/lib/session.ts:36` — `secret()`)

## 3. Sensitive assets

- **Billing flows** — `api/billing/{subscribe,payment-status,subscription,void-invoice}` — all session-gated via `pubkeyFromRequest` ✓ (plans.ts is public plan-listing, correct)
- **LiveKit moderation power** — kick/mute/promote/demote/lobby/lock/recordings — proxied upstream with Bearer token
- **Session signing secret** — env; compromise → universal session forgery (proper design, single point)
- **Bearer tokens to relay host** — see finding D-F1 (logged!)
- **nsec material** — shared with nostr-cms (Finding 3)

## 4. Highest-leverage components

1. `src/lib/session.ts` — session minting/verification (verified sound: signed token, exp check, constant-time-ish compare path)
2. `api/auth/login.ts` — challenge handshake (verified sound)
3. `api/livekit/*` proxy layer — trust boundary to relay; token handling (D-F1)
4. `src/lib/dashLogic/profileTabLoaders.ts` — unescaped innerHTML of remote data (D-F2)

## 5. Findings (dashboard-specific)

**D-F1 — Bearer tokens written to server logs** — `src/pages/api/livekit/kick-user.ts:14-15`
```
console.log('kick-user - token:', token)
console.log('kick-user - requestData:', requestData)
```
A valid relay moderation token in plaintext logs = token exfiltration via log access/rotation/shipping. `requestData` also dumps room/identity data. Fix: remove both lines (or log only `token.slice(0,6)+'…'`). **Same class may exist in sibling proxies** — `grep console.log` across `api/livekit/*` worth a pass by maintainer.

**D-F2 — Unescaped innerHTML of nostr event fields (XSS)** — `src/lib/dashLogic/profileTabLoaders.ts` `renderEvents` (~line 100-115) and `renderRooms` (~line 79): `title`, `summary`, `image` from event tags interpolate straight into `innerHTML`. A crafted event `title`/`image` (e.g., `" onerror=`) executes in viewers' dashboards. The codebase HAS `escapeHtml`/`sanitizeAvatarUrl` (`ModeratorManager.ts:30,42`) — this file just doesn't use them. My-Events/My-Rooms views carry the risk.

**Shared findings restated** (in maintainer report): nsec→localStorage (nostrify) — same exposure; `nostr-backup.txt` raw-key export; primal.net hardcode in `useUploadFile.ts:15`.

## 6. What verified SOUND (dashboard)

- Session system: opaque signed tokens, TTL, domain-bound challenge, nonce consumed single-use — model is correct
- Billing routes: every money endpoint checks `pubkeyFromRequest` before acting
- Moderator proxies: require Bearer, forward upstream — upstream enforces (consistent with "server checks" model)
- Guest ephemeral keys: per-room, disposable, cleared on exit — safe by design
- `get-token`: guests can't bypass — ephemeral path signs kind-27235; rate_limit error path exists (`get-token.ts:98`)

## 7. DDoS posture (dashboard)

- Own API surface: no rate-limit middleware observed in routes (a `rate_limited` error string exists in get-token — app-level throttle present there only)
- Upstream proxies amplify cost: every proxied call hits their relay backend
- Supabase edge functions: platform-level limits apply
- Recommend same nginx/CDN per-IP zones pattern as buttercup if they self-host the Astro node

## Priority order for maintainer

1. Remove token logging (D-F1) — one-line, real leak
2. Escape renderEvents/renderRooms interpolations (D-F2) — XSS
3. Same nsec-at-rest posture decision as nostr-cms (ncryptsec or gate)
