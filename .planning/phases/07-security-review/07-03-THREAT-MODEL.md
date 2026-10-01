# 07-03 Threat Model — nostr-cms + swarm stack (buttercup.exe.xyz)

**Date:** 2026-10-01 · **Scope:** React SPA, swarm relay/blossom/API binary (:3334), nginx edge (:8000/443) · **Method:** static code map + black-box probes on live service

## 1. Trust boundaries

| # | Boundary | Crossing |
|---|---|---|
| T1 | Internet → exe.dev edge → nginx :8000 | All public traffic; nginx routing + deny rules are the only L7 gate |
| T2 | nginx → swarm :3334 | **swarm binds `*:3334` (all interfaces), INPUT policy ACCEPT, no host firewall rules** — nginx bypass possible wherever :3334 is reachable |
| T3 | Browser page JS ↔ key custody | NIP-07 extension (isolated) / NIP-46 bunker (remote) / **in-page nsec signer (H1: plaintext localStorage)** |
| T4 | SPA → relay WS (`/`) | nostr REQ/EVENT; swarm gates writes to "team" members (verified: `blocked: you are not part of the team`) |
| T5 | SPA → Blossom (`/upload`, `/{sha}`, `/list/`, `/mirror`) | BUD-02 24242 signed auth events |
| T6 | SPA → `/api/*` | NIP-98 signed HTTP (scheduler) + `dashboard_session` cookie (admin/dashboard) |
| T7 | SPA → swarm data paths | `nostr.json` admin list is the authz source of truth (public, correct) |
| T8 | Browser localStorage | holds login state (incl. nsec when enabled) + siteConfig (blossom relay lists) |
| T9 | SPA → external blossom servers | blob fetches by hash (now sha256-verified in StaticPage) |
| T10 | Build-time env (`VITE_*`) | public config — pubkey/relay/feature flags; no secrets belong here |

## 2. Entry points (untrusted input)

**HTTP via nginx :8000 → :3334**
- `GET/PUT /upload` — body ≤1024m (`client_max_body_size`), 24242 auth required (401 unauth ✓)
- `GET /{64-hex}[.ext]` — public blob read (by design)
- `DELETE /{sha}` — 24242 owner-auth (403 wrong-key ✓; unauth→502 cosmetic)
- `GET /list/{pubkey}` — public enumeration (BUD-04; info disclosure)
- `POST /mirror` — proxied; unauth POST → 405
- `POST /process-video[-stream]` — auth required (401 unauth ✓); **CPU-heavy ffmpeg — resource-exhaustion surface for any authed caller**
- `/api/scheduler/*` — NIP-98, owner-scoped (401 unauth + wrong-key ✓; absolute-`u` verified post-fix)
- `/api/admin|dashboard/{stats,events,zap-stats,my-stats}` and `{users,environment,user/*}` — **allowlisted through nginx**, cookie-auth → see C1
- `/api/admin/*`, `/api/dashboard/*` others; `/backfill-owners`, `/dashboard` → nginx deny ✓
- `POST /api/admin/login` → **nginx 403 on this deploy, but swarm accepts bare pubkey** (see C1)
- `GET /.well-known/nostr.json`, `/public/*`, `/api/health` → public by design
- Binary inventory also shows: `/api/token`, `/api/dashboard/user/`, `/list_manual`, `/dashboard_view` — probed/unprobed, swarm-side

**WS `/`** — REQ (open reads), EVENT (team-gated writes ✓ verified)

**Client-side inputs** — file uploads (magic-byte validated, MIME normalized), `.txt` nsec import (now env-gated), markdown/HTML rendering (rehype-sanitize + DOMPurify + CSP), nostr event content (React-escaped), localStorage (attacker-writable under XSS), remote `nostr.json`, fetched page blobs (now hash-verified)

## 3. Sensitive assets

- **Master/publisher private keys** — sign everything: site config, pages, media auth, scheduler. In extension custody today (verified) — H1 keeps raw-nsec path off in prod.
- **`dashboard_session` cookie** — issued with zero proof (C1).
- **Scheduled-post signed events** — stored pre-publish server-side (content leak until publish).
- **Media blobs** — public-read by design; write/delete auth'd.
- **Zap analytics + relay stats** — admin-data disclosure via C1.
- **nostr.json admin list** — integrity held by swarm/relay (anyone can read; writes protected by team gate).

## 4. Highest-leverage components if compromised

1. **swarm `/api/admin/login` + session check** — one weak endpoint → admin read surface (C1).
2. **The browser login dialog / nostrify localStorage persistence** — key theft = total control (H1, mitigated by env gate).
3. **Publisher-authored content path** (kind-34128 → DOMPurify) — compromised publisher → stored sanitized-HTML injection.
4. **Blossom relay-list config (localStorage)** — steers uploads/fetches to attacker servers (M2 hardened; T8).
5. **Relay "team" write gate** — if bypassed, arbitrary event injection (verified sound today).

## 5. Ranked attack surfaces

| Rank | Surface | Status |
|---|---|---|
| **C1 — CRITICAL** | **swarm dashboard session: `POST /api/admin/login` issues `dashboard_session=<pubkey>` for any admin-listed pubkey — no signature/proof.** Cookie is self-declarable → forged `Cookie: dashboard_session=<admin>` yields `200` on `/api/admin/users` (verified), and full `/login`+session→stats/zap-stats chain works against bare :3334 (verified: users/stats/environment/zap-stats all 200 after proof-free login). nginx denies `/login` on this deploy but forged-cookie access to `users` works *through* nginx. On deployments without nginx in front (oneshot VPS default?), whole admin API is open. swarm binds `*:3334` — verify whether port is publicly reachable; bind to 127.0.0.1 regardless. **Fix belongs in swarm upstream: require NIP-98 proof at /login + random opaque session tokens + HttpOnly.** | **Verified live** |
| A1 | nsec-in-localStorage (H1) — env-gated off in prod build today; upstream needs ncryptsec or policy | mitigated buttercup |
| A2 | nginx: **no `limit_req`/`limit_conn` anywhere** — all public endpoints unthrottled | open |
| A3 | `/process-video*` — auth'd but CPU-heavy; no rate limit → authed DoS/成本 vector | open |
| A4 | `client_max_body_size 1024m` on upload/mirror/process-video — 1GB per request × N concurrent = disk/memory pressure | open |
| B1 | Unverified write surface behind cookie-auth endpoints (only GETs probed — `/api/admin/user/*` POST/PUT behavior unknown) | needs probe (non-prod) |
| B2 | NIP-98 `expiration` enforcement by swarm — untested | unverified |
| B3 | `/mirror` real-method auth — POST 405'd; untested with correct method | unverified |
| B4 | StaticPage publisher-author XSS — bounded by sanitize (hash-verify shipped today) | mitigated |
| B5 | blossom relay list from localStorage → attacker-configured servers (XSS-dependent) | partial (M2 throw) |

## Auth/AuthZ flow map (end-to-end)

**Path A — NIP-98 (scheduler):** client signs kind-27235 `{u,method[,payload]}` per request (`useScheduledPosts.ts:26`) → Authorization `Nostr <b64>` → swarm verifies sig+URL+method+ownership (verified: unauth 401, wrong-key 401, relative-u rejected pre-fix).
Points where checks exist: list `/scheduler/list` (:117,:168), schedule `:196`, delete `:233,:313,:346`, retry `:287`. **No client-side bypass exists — all sensitive ops are server-auth'd.** ✓

**Path B — Blossom 24242:** client signs kind-24242 `{t,x,expiration(15min)}` → Authorization bearer → swarm enforces: upload 401 unauth, delete 403 wrong-key. Call sites: `mediaProcessing.ts` uploadMediaFile/streamUpload, `useFollowBackup.ts:1047-1051` (delete). ✓

**Path C — dashboard cookie (THE BROKEN ONE):** `AdminExplorer.tsx:159-169` `ensureAdminSession` POSTs `{pubkey}` to `/login` → cookie → `credentials:'include'` fetches (`AdminExplorer.tsx:176-179`, `AdminRelayAccess.tsx:184-216`, `useNostrJsonUsers.ts:48-50`). **Zero proof-of-possession in the exchange.** Client code assumes server challenges properly; swarm doesn't. Post-converge, upstream replaced zap-stats/my-stats reads with relay queries — the cookie path remains in AdminExplorer/RelayAccess/nostr-json-users code.

**Path D — relay WS:** REQ open; EVENT gated by team membership (verified blocked for outsider). Correct.

**Path E — client gates:** `useAdminAuth` (`useRemoteNostrJson.ts:28`) reads remote `nostr.json` → `AdminLayout.tsx:51` hides UI. **UI-only, correctly so** — every privileged op re-checks server-side except the broken Path C. `StaticPage.tsx:53-60` filters page authors to master/publisher ✓.

## Broken-access-control gaps (file:line)

| Gap | Where | Status |
|---|---|---|
| Admin session issued without proof | swarm `/api/admin/login` (server-side); client trust at `AdminExplorer.tsx:159-169`, `AdminRelayAccess.tsx:184-216`, `useNostrJsonUsers.ts:48` | **CRITICAL — verified** |
| `/api/admin/users` accepts forged cookie, no session | swarm endpoint via nginx allowlist | **verified live** |
| Unprobed write endpoints under cookie path | `/api/admin/user/*` POST/PUT (nginx-allowlisted), `/api/token` | untested — probe on staging |

## 6. DDoS assessment

**Yes — materially exposed.** No `limit_req`/`limit_conn`/`client_max_body_size` caps beyond the 1GB bodies; no per-IP throttling at nginx. Specifics:

- **WS relay**: unbounded REQ/EVENT flood from any IP; legit client code (`useZapAnalytics`) already paginates hard — an attacker needs no client, just a socket. Swarm may have internal caps (binary, unverified).
- **`/upload`**: 1GB bodies × concurrency → disk/CPU exhaustion (auth required, so actor = any key that can produce a 24242 — i.e., **anyone**).
- **`/process-video*`**: auth'd ffmpeg jobs — expensive-per-request; N concurrent jobs starve CPU.
- **`/list/`, blob GETs, `/api/*`**: cheap but unlimited → volumetric amplification via exe.dev bandwidth.
- **Mitigations (nginx-side, ours to apply):**
  - `limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s` + `limit_req` on `/api/`, `/list/`, blob reads (burstable)
  - `limit_conn` on `/` WS upgrade (e.g., 20/IP) + `proxy_read_timeout` sane
  - Drop `client_max_body_size` to real need (~200m?) on upload/process paths
  - `limit_rate` on blob GETs if bandwidth abuse appears
  - log `429`s; consider fail2ban on repeat offenders
- **swarm-side (upstream)**: per-conn REQ rate cap, max active ffmpeg jobs, session-store TTL — file as upstream issues
- **Platform**: exe.dev edge may absorb L3/L4 volumetrics; our layer is L7 request floods.

## Recommended next actions

1. **Report C1 to HiveTalk/swarm maintainer immediately** — it's a swarm fix (proof-of-possession at login, opaque session tokens); include the :3334 binding note. On buttercup, nginx already masks /login but forged-cookie `users` access remains until swarm fixes it — optionally tighten the nginx allowlist to drop `/api/admin/users|environment|user/*` (the upstream WS-rewrite doesn't need them).
2. Bind swarm to 127.0.0.1 in its config/systemd — removes the direct-bypass boundary T2.
3. nginx rate-limit block (A2–A4) — config-only, ~15 lines.
4. Probe B1 write surface on a non-prod swarm before assuming read-only blast radius.
