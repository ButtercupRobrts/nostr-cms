# PR Restructuring Strategy Report

> **Revision 3 (2026-09-12):** Two rounds of corrections.
> Rev 2: wrong-branch fix (audited `beeswax` instead of `zeabur-dashboard`),
> reviewer-quote fix, NIP-86 names gap, redeploy delta.
> Rev 3: Phase 0 security stopgap corrected (cookie is forgeable — deny both
> admin prefixes, not just `/login`), kind number fixed (27235 not 24242),
> `fetchWithNip98` can't be reused for NIP-86 (missing `payload` tag), PR 1
> split (trivial fixes vs NIP-98 login — latter is a breaking change), NIP-86
> `allowpubkey` reasoning corrected, `useCommunityZapStats` fetch count fixed
> (2 not 4), same-repo PR noted, NIP-98 login breaking change flagged.
> See §6.7 for the full correction log.

## Context

HiveTalk/swarm PR #16 (branch `pr-c-backend`, +7,547/-386 lines across 21 files) was reviewed by `bitkarrot` with "changes requested." The reviewer's actual feedback (verified by reading the full 9,600-char review + both follow-up comments): **the PR bundles 8+ unrelated features, most should be rejected for per-item defects** (external DB ingestion, missing ffmpeg, hand-rolled bech32, nsec handling in NostrLogin.js, DB-scanning endpoints, secret presence exposure). The dashboard verdict evolved to "No. Separate feature" — because it scans the DB per request and leaks env-var presence — not because REST is philosophically wrong.

The reviewer **never said** "no need for REST APIs in most cases where we can pull from websockets." That was the user's paraphrase of an offline conversation, not text in the review. The review is about bundling and per-item defects, not about REST vs. WebSocket philosophy. The maintainer's own actions confirm this: they merged the REST admin API (`/api/dashboard/*`, `/api/admin/*`) upstream and wired `vercel.json` to proxy `/api/` to `swarm.hivetalk.org`.

This report analyzes the upstream architecture of both `swarm` and `nostr-cms`, maps our PR's features against what already landed upstream, presents 3 strategies for moving forward, and defines a deployment-safe pipeline that keeps buttercup.exe.xyz untouched until upstream merges.

---

## 0. Verified State of the World (No Assumptions)

### 0.1 PR Status (verified via GitHub API, 2026-09-12)

| PR | Repo | State | Notes |
|---|---|---|---|
| #78 | bitkarrot/nostr-cms | **MERGED** 2026-09-10 | Core utilities, bugfixes, config updates (`pr-a-frontend` branch) |
| #79 | bitkarrot/nostr-cms | **MERGED** 2026-09-10 | Blossom media, calendar, zaplytics, repost, mentions, settings, admin features (`pr-b-features` branch) |
| #16 | HiveTalk/swarm | **OPEN** | Scheduler, zap analytics, dashboard (`pr-c-backend` branch) — changes requested. **Superseded**: most of its content already merged to `zeabur-dashboard` via other PRs. |

### 0.2 The Wrong-Branch Correction

**Original error:** The report's §1.1 "upstream architecture" and §0.3 "what's on upstream" described `beeswax` @ `aaba6f6`. But PR #16's base was `zeabur-dashboard` — the repo's default branch — and `beeswax` is 0 ahead / 73 behind it.

**Corrected base:** All swarm analysis now uses `zeabur-dashboard` @ `a30fb61` (fetched as `refs/swarm-zeabur`).

### 0.3 What Already Landed on `zeabur-dashboard` (verified via `git cat-file -e` and `git show`)

The following from PR #16 are **already merged** to `zeabur-dashboard` (the actual upstream base):

| Feature | On `zeabur-dashboard`? | Evidence |
|---|---|---|
| `scheduler.go` (635 lines) | **YES** — byte-for-byte identical to `pr-c-backend` | Same md5sum: `d8862e57fb23f84572ccb35746a8ac65` |
| `setupDashboardHandlers` + `/api/dashboard/*` + `/api/admin/*` | **YES** — 7 endpoints: `/login`, `/ui`, `/logout`, `/users`, `/user/`, `/environment`, `/convert` | `main.go:1425-1698` |
| `dashboard.html` (343 lines) | **YES** | `public/dashboard.html` |
| `templates/dashboard_view.html` (593 lines) | **YES** | `templates/dashboard_view.html` |
| `NostrLogin.js` (904 lines) | **YES** | `public/js/NostrLogin.js` |
| `npubToHex`/`hexToNpub` with wrong bech32 alphabet | **YES** — at `main.go:1875` and `main.go:1969` | `const alphabet = "023456789acdefghjklmnpqrstuvwxyz"` (wrong; correct is `qpzry9x8gf2tvdw0s3jn54khce6mua7l`) |
| Configurable rate limiters | **YES** — 15 references to `rateLimiter`/`newRateLimiter` | `main.go` |
| `fetchNostrData` fix | **YES** | merged |
| `getEnvironmentVars` (masks secrets, exposes presence) | **YES** | `main.go` — masks `password`/`secret`/`key`/`database_url` values, but exposes their presence/absence |
| Scheduler endpoints (`/api/scheduler/schedule`, `/list`, `/delete`) | **YES** | `main.go:263-265` |

The following from PR #16 did **NOT** land on `zeabur-dashboard`:

| Feature | On `zeabur-dashboard`? | Notes |
|---|---|---|
| `zap_stats.go` (1,421 lines) | **NO** | External relay ingestion, `db.SaveEvent` of external events |
| `/stats`, `/events`, `/my-stats`, `/zap-stats` endpoints | **NO** — 0 references | These were the DB-scanning endpoints the reviewer rejected |
| Video transcoding (`/process-video`, `/process-video-stream`) | **NO** — 0 references to `ffmpeg`/`process-video` | Requires ffmpeg not in Dockerfile |
| `/backfill-owners` | **NO** — 0 references | One-time migration endpoint |
| Blob owner/mime sidecar maps | **NO** — 0 references to `readOwnerMap`/`setBlobOwner`/`readMimeMap`/`setBlobMime` | |

**Consequence:** Strategy B's PR 1 (scheduler) and PR 2 (small fixes) are **already upstream** — there's nothing to split. PR #16 should be closed because it's **superseded**, not merely rejected. Any new PRs must branch from `zeabur-dashboard`, not `beeswax`.

### 0.4 Upstream nostr-cms Post-Merge Activity (verified via git log)

After our PRs #78/#79 merged, `bitkarrot` made 7 follow-up commits on `upstream/main`:

| Commit | What it does |
|---|---|
| `dadc676` | Restore missing Feed, Relay Access, Profile to admin sidebar |
| `c7f6ef7` | Fix blank homepage when `homepage_section_order` is empty array |
| `0b17c1c` | Bump react-router (#80) |
| `27ec30d` | Restore hero background and use white hero text |
| `28ca642` | **Make relay explorer work without unavailable stats API** — added WebSocket `nostr.query()` fallback to `AdminExplorer.tsx` when REST `/events`/`/stats` returns 404/401 |
| `48ea74b` | **Proxy swarm admin API through Vercel** — `vercel.json` now rewrites `/api/:path*` → `https://swarm.hivetalk.org/api/:path*` |
| `de639e4` | Remove unsafe-eval from CSP, add rehype-sanitize |

**Key finding:** `bitkarrot` already patched `AdminExplorer.tsx` to have a WebSocket fallback when the REST API is unavailable (commit `28ca642`). The upstream `AdminExplorer.tsx` now tries REST first, falls back to `nostr.query()` — but still has 6 `fetchAdminApi` calls. The REST path is still the primary one. This suggests the maintainer wants REST-when-available, not REST removal.

**Key finding:** `upstream/main` now has `vercel.json` proxying `/api/` to `swarm.hivetalk.org` — meaning the upstream deployment expects the swarm REST API to exist. This is a deployment coupling between nostr-cms and swarm that the maintainer created. Do not remove `vercel.json` without confirming with `bitkarrot`.

### 0.5 What's ON `upstream/main` Now (verified via `git cat-file -e`)

| File | On upstream/main? | Data source |
|---|---|---|
| `useZapAnalytics.ts` | YES | Nostr-native (WS `kind 9735` subscriptions, 6 `nostr.query` calls). **Single-pubkey**: filter uses `#p: [pubkey]`, not multi-pubkey. |
| `useCommunityZapStats.ts` | YES | REST (2 `fetch` calls: `/dashboard/login` + `/dashboard/zap-stats`). Returns `MemberStats[]` — an all-members leaderboard. **Calls `/dashboard/zap-stats` which does NOT exist on `zeabur-dashboard`** — 404s on the upstream deployment. |
| `AdminExplorer.tsx` | YES | REST primary + WS fallback (6 `fetchAdminApi`, 3 `nostr.query`). Auth via `useAdminApi` (calls REST `/login`) + `useAdminAuth` (reads `nostr.json`). |
| `MyActivityCard.tsx` | YES | REST (`/dashboard/my-stats`, `/dashboard/login`). **Calls `/my-stats` which does NOT exist on `zeabur-dashboard`** — 404s on the upstream deployment. |
| `AdminZaplytics.tsx` | YES | Both (imports `useZapAnalytics` AND `useCommunityZapStats`, with tabs to switch) |
| `AdminRelayAccess.tsx` | YES | REST (10 `fetchAdminApi`/`fetch` calls) — pre-existing, not added by our PR. Manages `nostr.json` names map: username↔pubkey CRUD + `"_"` owner transfer via `PUT /user/{pubkey} {name:"_"}`. |
| `useNostrJsonUsers.ts` | YES | REST (8 `fetch`/`adminApi` calls) — pre-existing, not added by our PR. Calls swarm `/api/admin/users`. |
| `useScheduledPosts.ts` | YES | REST with **NIP-98 auth** (`fetchWithNip98`, `Authorization: Nostr {token}`, **kind 27235** — not 24242, which is the Blossom auth kind). This is the correct pattern. Note: `fetchWithNip98` sends `u` + `method` tags but **no `payload` hash tag** — khatru's `HandleNIP86` requires a `payload` tag with the SHA-256 hash of the request body, so the `useNip86` hook cannot reuse `fetchWithNip98` as-is. |
| `useAdminAuth` | YES | Exported from `useRemoteNostrJson.ts`. Reads `nostr.json` via HTTP. Returns `{isAdmin, isMaster, isLoading, masterPubkey, allowedPubkeys}` (5 fields). Does NOT call the REST admin API. |

### 0.6 buttercup.exe.xyz Deployment (verified via nginx config, process list, filesystem)

**Frontend:**
- nginx serves from `/home/exedev/meetup-space/nostr-cms/dist` (root directive in `/etc/nginx/sites-enabled/meetup-space`)
- `nostr-cms/dist/index.html` was built 2026-09-02 18:05 (after `deploy-latest` commit at 10:30)
- The deployed bundle references `/dashboard/login`, `/dashboard/my-stats`, `/dashboard/zap-stats` (verified by grepping `nostr-cms/dist/assets/*.js`)
- nginx proxies `/api/` → `127.0.0.1:3334/api/`, `/dashboard` → `127.0.0.1:3334/dashboard`, `/process-video` → `127.0.0.1:3334/process-video`, etc.
- **The deployed frontend uses the REST API.** It is coupled to the running swarm binary.

**Relay:**
- `swarm-relay` binary running (PID 211396, built 2026-09-01 21:26, 35MB)
- Runtime data: `swarm/db/scheduled_posts.json` (1MB, 533 entries: 272 pending, 261 published, 459 with `repeat_total`/`repeat_index`/`repeat_interval` tags — updated 2026-09-12 12:17), `swarm/db/zap-stats.json` (254KB, updated 2026-09-12 12:12)
- Port 3334, `RELAY_NAME="Buttercup"`, `TEAM_DOMAIN="buttercup.exe.xyz"`
- **The running relay binary has the REST API, scheduler, zap analytics, video transcoding.** It was built from our full feature set (`pr-c-backend`), which has more than what landed on `zeabur-dashboard`.

**Branch structure:**
- `deploy-latest` = production tracking (2 commits ahead of `origin/main` = `github/main`)
- `origin/main` (exe.dev internal) = `github/main` (NostrCMSbtcp) = canonical deployed code
- `deploy-latest` has `nostr-cms/` and `swarm/` as subdirectories (monorepo structure)
- `pr-b-features` (current working tree) has `src/` at root level (flattened for upstreaming)
- `nostr-cms/dist` is gitignored (built artifact, not tracked in git)

**Build tools available:** Go 1.26.4, Node 22.22.3, npm 10.9.8

### 0.7 NIP-86 Library Support and Limitations (verified)

**Go (khatru):** khatru v0.15.2 has **built-in NIP-86 support**. The `RelayManagementAPI` struct exposes `AllowPubKey`, `BanPubKey`, `ListAllowedPubKeys`, `ListBannedPubKeys`, `AllowKind`, `DisallowKind`, `ListAllowedKinds`, `BlockIP`, `UnblockIP`, `ListBlockedIPs`, `ChangeRelayName`, `ChangeRelayDescription`, `ChangeRelayIcon`, etc. The handler `HandleNIP86` is auto-registered in `ServeHTTP` when `Content-Type: application/nostr+json+rpc` — no manual route registration needed. Auth via NIP-98 (`khatru.GetAuthed(ctx)`). The upstream swarm does NOT currently use it (no `ManagementAPI` assignments in `main.go`).

**NIP-86 CANNOT replace `AdminRelayAccess` / `useNostrJsonUsers`.** These manage the `nostr.json` `names` map — username↔pubkey CRUD plus the `"_"` owner transfer (`PUT /user/{pubkey} {name:"_"}`). NIP-86 `listallowedpubkeys` returns `[]PubKeyReason` — no names, no rename, no ownership transfer. NIP-86 can complement (allow/ban kinds, IPs) but the names CRUD needs the custom endpoint that already exists upstream. **Dropping the plan to replace `AdminRelayAccess` with NIP-86.**

**NIP-86 `AllowPubKey`/`BanPubKey` — why not implement them either.** Swarm's membership IS the `nostr.json` `names` map (`data.Names`). The event-reject path checks `data.Names` for team membership (`main.go:199-206`). `allowpubkey(pk)` doesn't need names CRUD — but it needs a separate pubkey allowlist consulted by the event-reject path, which swarm doesn't have. Implementing `AllowPubKey` would require either (a) creating a separate allowlist data structure, or (b) writing nameless entries to `nostr.json` (which breaks the names map semantics). Neither is a clean fit. NIP-86 PR should focus on `AllowKind`/`DisallowKind`/`ListAllowedKinds`/`BlockIP`/`UnblockIP`/`ListBlockedIPs` — those map cleanly to swarm's existing config.

**JS (nostrify):** nostrify does NOT have a built-in NIP-86 client. A NIP-86 client is a simple `fetch` with `Content-Type: application/nostr+json+rpc` + NIP-98 `Authorization` header. It can be implemented as a small custom hook (~50 lines) using `nostr-tools` for NIP-98 event signing.

### 0.8 Security Issue — Live in Production (verified)

**The `dashboard_session` cookie is forgeable — the `/login` endpoint is a formality, not a gate.**

`requireAdminSession` (`main.go:1441-1453`) just compares `cookie.Value` to `adminPubkey` — the cookie value IS the pubkey. There is no nonce, no signature, nothing server-side to forge against. An attacker can skip `/login` entirely and send `Cookie: dashboard_session=f288a224a61b7361aa9dc41a90aba8a2dff4544db0bc386728e638b21da1792c` straight to `POST /api/admin/users` and rewrite NIP-05 names. The `/login` endpoint is cosmetic.

On `zeabur-dashboard` (upstream): the expected cookie value is the `"_"` pubkey (the relay owner). That pubkey is **public** — it's in `nostr.json` at `/.well-known/nostr.json`. So anyone can forge the cookie.

On our `pr-c-backend` (buttercup.exe.xyz): accepts **any** team pubkey listed in `nostr.json` (via `isKnownTeamPubkey`). All team pubkeys are public. Same vulnerability — any of them can be forged.

The handlers are mounted at **two prefixes**: `registerAdminAPI` (`main.go:1436-1438`) registers every handler at both `/api/dashboard<path>` and `/api/admin<path>`. Blocking only `/api/dashboard/login` leaves `/api/admin/login` open — but that's moot since the cookie is forgeable regardless of the login endpoint.

This is exposed at `buttercup.exe.xyz/api/` right now, and almost certainly at `swarm.hivetalk.org`.

**The scheduler endpoints are safe** — they use NIP-98 with server-side signature verification. `checkAuth` (`scheduler.go:554-604`) verifies kind 27235, `u` tag, and schnorr signature. The same NIP-98 pattern should gate the dashboard. But this is a breaking change: `dashboard.html` (`dashboard.html:187-193`) calls `/api/dashboard/login` with just `{pubkey}`, and `dashboard_view.html` calls `/api/dashboard/users` and `/api/dashboard/environment` with the session cookie — no signed requests. Adding NIP-98 breaks the bundled dashboard UI (`dashboard.html`, `dashboard_view.html`, `NostrLogin.js`) until they're updated to produce signed requests. This is a breaking change to `swarm.hivetalk.org`'s deployment, not just ours — ask `bitkarrot` before shipping.

**Phase 0 nginx stopgap (corrected):** Deny `/api/dashboard/` and `/api/admin/` entirely at the nginx level. This takes the admin UI dark on buttercup until a real NIP-98 fix ships — right call given anyone on the internet can currently rewrite `nostr.json` (NIP-05 names, team membership) on production. `/api/scheduler/*` stays open (NIP-98-gated server-side, verified at `scheduler.go:555+`). Vanilla nginx cannot verify schnorr signatures (no njs/Lua), so a proxy-level signature check is not realistic — the stopgap is denial, not verification.

The report's earlier "secret exposure" framing was also off — `getEnvironmentVars` masks `password`/`secret`/`key`/`database_url` values (verified: `***MASKED***`). The real problem is the session model (forgeable cookie, no signature verification), not the env-var exposure.

### 0.9 Scheduler and Recurrence (verified)

The scheduler's intended scope was always **one-shot**, not recurring. `bitkarrot` retracted the recurring critique in a follow-up comment: "The scheduler's intended scope was always one-shot scheduling, not recurring. That critique was wrong."

Recurring already works **client-side**: `swarm/db/scheduled_posts.json` holds 533 entries (272 pending, 261 published), 459 with `repeat_total`/`repeat_index`/`repeat_interval` tags. The CMS pre-signs the whole series — each repeat instance is a separate signed event with its own scheduled time. No Go changes needed for recurrence.

The one remaining technical nit: `ListPending()` calls `go s.save()` asynchronously (`scheduler.go:169`), which can race with the subsequent `s.store.Update(post)` in `publishPost`. Fix: make saving synchronous or use a write queue.

---

## 1. Upstream Architecture Analysis

### 1.1 Swarm (HiveTalk/swarm, branch `zeabur-dashboard` @ `a30fb61`)

**What it is:** A khatru-based Nostr relay + Blossom (NIP-B7) media server + dashboard/admin API. Written in Go. The `zeabur-dashboard` branch is the repo default and the PR #16 base. It is 73 commits ahead of `beeswax` (which is an older/parallel branch).

**Data layer:**
- `DBBackend` interface with three implementations: PostgreSQL, BadgerDB, LMDB
- All Nostr event storage/query goes through khatru's `relay.StoreEvent` / `relay.QueryEvents` / `relay.DeleteEvent` hooks
- No separate analytics tables, no sidecar JSON files, no custom schemas (the scheduler uses a JSON file, not the DB)

**Read path:**
- **Nostr data reads go through WebSocket REQ/EVENT** via khatru's built-in relay handler
- The relay has **no REST endpoints for reading Nostr events** — no `/api/events`, no `/api/stats` (those were in PR #16 but rejected/not merged)
- HTTP endpoints on `zeabur-dashboard`:
  - `/public/` — static file serving
  - `/dashboard` — dashboard HTML page
  - `/api/dashboard/*` + `/api/admin/*` — admin API: `/login`, `/ui`, `/logout`, `/users`, `/user/`, `/environment`, `/convert` (7 endpoints)
  - `/api/scheduler/*` — scheduler: `/schedule`, `/list`, `/delete`
  - `/api/health` — health check
  - `/convert` — pubkey conversion page
  - `/list/{pubkey}` — Blossom blob list
  - `/mirror` — Blossom blob mirror
  - Blossom endpoints (`PUT /upload`, `GET /{sha256}`, etc.) via `khatru/blossom`
  - `/.well-known/nostr.json` — NIP-05 mapping

**Access control:**
- Based on `nostr.json` `names` map (team members)
- `ALLOWED_KINDS` / `PUBLIC_ALLOWED_KINDS` env vars
- `TRUSTED_CLIENT_NAME` / `TRUSTED_CLIENT_KINDS` for client-specific kind exceptions
- Rate limiting: per-pubkey, per-IP, per-connection, per-query (configurable)
- Dashboard login: POST pubkey → session cookie (no signature verification — see §0.8)

**NIPs implemented:** NIP-01 (relay), NIP-05 (verification), NIP-09 (deletion), NIP-11 (info doc), NIP-19 (bech32 — hand-rolled, **wrong alphabet** at `main.go:1875` and `main.go:1969`), NIP-42 (khatru auth), NIP-65 (relay list, via khatru), Blossom/NIP-B7 (media), NIP-98 (scheduler endpoints only). **NIP-86 is available in khatru but NOT enabled.**

**What is NOT there:** `zap_stats.go`, video transcoding, `/backfill-owners`, blob owner/mime sidecar maps, `/stats`/`/events`/`/my-stats`/`/zap-stats` endpoints. These were in PR #16 but did not land.

### 1.2 Nostr-CMS (bitkarrot/nostr-cms, `upstream/main` @ `dadc676`)

**What it is:** A client-side React 18 + TypeScript SPA (Vite build). A Nostr client that reads from relays via WebSocket — not a relay itself, not a backend.

**Data access pattern:**
- **All public content is read from Nostr relays via WebSocket REQ subscriptions**, not from REST APIs:
  - Blog posts: `kind 30023`
  - Events: `kind 31922/31923/30313`
  - Feed notes: `kind 1` with NIP-65 fan-out
  - Comments: `kind 1111`
  - RSVPs: `kind 31925`
  - Zap receipts (per-post): `kind 9735` with NIP-65 fan-out
  - Profiles: `kind 0` with NIP-65 fan-out
  - Site config: `kind 30078`

**Zap analytics — two paths, not equivalent:**
- `useZapAnalytics.ts` (Nostr-native): subscribes to `kind 9735` via WS with `#p: [pubkey]` — **single-pubkey**. Aggregates client-side. Progressive loading with caching. Good for per-user/per-post stats.
- `useCommunityZapStats.ts` (REST): calls `/dashboard/zap-stats` which returns `MemberStats[]` — an **all-members leaderboard** with per-member earnings, zaps, unique zappers, top content. This is a different shape of data. Client-side equivalence means looping N members × NIP-65 fan-out — feasible, but it's a real cost, not "already exists." Also, `/dashboard/zap-stats` does NOT exist on `zeabur-dashboard` — it 404s on the upstream deployment.

**Existing REST dependencies (pre-our-PR, still on `upstream/main`):**
- `AdminRelayAccess.tsx` — calls swarm REST `/users`, `/user/{pubkey}` for `nostr.json` names CRUD + owner transfer. **NIP-86 cannot replace this** (no names field in NIP-86).
- `useNostrJsonUsers.ts` — calls swarm REST `/api/admin/users` for `nostr.json` user list. **NIP-86 cannot replace this** (same reason).
- These are the **pre-existing** REST dependencies for relay administration. They use a custom REST API because NIP-86 doesn't support names CRUD. They should stay on REST.

**Scheduler:** `useScheduledPosts.ts` uses `fetchWithNip98` — NIP-98 `Authorization` header with kind 27235 signed event. This is the correct Nostr-native auth pattern for HTTP endpoints.

**Library:** `@nostrify/nostrify` + `@nostrify/react` for the relay pool, `nostr-tools` for nip19/nip57. nostrify does NOT have NIP-86 client support.

---

## 2. The Core Problem (Corrected)

The original report framed the problem as "REST vs. WebSocket philosophy." That framing was wrong. The reviewer's actual feedback was about **bundling and per-item defects**, not REST philosophy. The maintainer merged the REST admin API upstream and wired `vercel.json` to it.

The real problems are:

1. **PR #16 is superseded.** Most of its content (scheduler, dashboard, NostrLogin.js, npubToHex, rate limiters) already landed on `zeabur-dashboard` via other PRs. PR #16 should be closed as superseded.
2. **`useCommunityZapStats` and `MyActivityCard` call endpoints that don't exist upstream.** `/dashboard/zap-stats` and `/dashboard/my-stats` were in PR #16 but did NOT land on `zeabur-dashboard`. They 404 on the upstream deployment (`swarm.hivetalk.org`). They work on `buttercup.exe.xyz` only because our running binary has `zap_stats.go`.
3. **Security: `/api/dashboard/login` has no signature verification.** Anyone can POST a public pubkey and get an admin session. This is live in production. The scheduler already uses NIP-98 — the dashboard should too.
4. **Wrong bech32 alphabet in `npubToHex`/`hexToNpub`** — already merged to `zeabur-dashboard`. Should use `go-nostr/nip19`.
5. **`go s.save()` race condition** — already merged to `zeabur-dashboard`. Should be fixed.
6. **`getEnvironmentVars` exposes secret presence** — already merged. Should remove secrets from the list entirely (not just mask values).
7. **`zap_stats.go` (external relay ingestion)** — did NOT land. If wanted, it's a new PR with the reviewer's concerns addressed (config flag, no `db.SaveEvent` of external events, use `go-nostr` NIP-57 utils, tests).

---

## 3. Nostr-Native Alternatives (Verified Against NIPs)

| Feature | Nostr-native alternative | NIP/standard | Library support | Caveats |
|---|---|---|---|---|
| Zap analytics (server-side) | Client subscribes to `kind 9735` on relay(s), aggregates locally. `useZapAnalytics.ts` already does this for single-pubkey. | NIP-57 | nostrify (WS), nostr-tools (NIP-57) | **Not a drop-in for community leaderboard.** `useZapAnalytics` is single-pubkey (`#p: [pubkey]`). Community stats require looping N members × NIP-65 fan-out. Feasible but a real cost. |
| Dashboard my-stats | Client queries own events (`kinds: [1, 30023, ...]`, `authors: [pubkey]`) via WS, counts locally | NIP-01 | nostrify `nostr.query()` | Straightforward. |
| Event explorer | Client sends `REQ` with arbitrary filters via WS. Upstream already added this as a fallback (commit `28ca642`). | NIP-01 | nostrify `nostr.query()` | Maintainer built the WS fallback, suggesting they want REST-when-available. **Ask before removing the REST path.** |
| User/relay management | NIP-86 for allow/ban kinds, IPs. **Custom REST for `nostr.json` names CRUD** (NIP-86 has no names field). | NIP-86, NIP-98 | khatru built-in (server). JS: custom ~50-line hook. | **NIP-86 cannot replace `AdminRelayAccess`.** Names CRUD stays on REST. |
| Scheduled posts | Client holds signed event, publishes at scheduled time. Recurring: CMS pre-signs the whole series (already works — 459 posts with repeat tags). | Client-side timer | nostrify (WS publish), nostr-tools (signing) | No server needed for recurrence. Server-side scheduler is custodial (holds signed events). |
| npub↔hex conversion | `nostr-tools` `nip19.encode/decode` — pure client-side, no server round-trip | NIP-19 | nostr-tools | Replace wrong-alphabet server-side `npubToHex` with `go-nostr/nip19`. |
| Dashboard login auth | NIP-98 signed event (kind 27235) — same pattern as `useScheduledPosts.ts`. Must include `payload` hash tag for khatru NIP-86 compatibility. | NIP-98 | nostr-tools (signing) | **Security fix needed now.** Breaking change: requires dashboard UI updates. |
| Video transcoding | Server-side ffmpeg is inherently a server job. Keep as REST endpoint with NIP-98 auth. | NIP-71, NIP-B7 | khatru/blossom (server) | Separate PR, ffmpeg in Dockerfile. |
| NIP-05 management | NIP-05 requires HTTP `/.well-known/nostr.json`. User management (names CRUD) uses the custom REST API. | NIP-05 | HTTP (no library needed) | Stays on REST. |

**There is no standard NIP for scheduled post publishing.** NIP-71 is Video Events. NIP-40 (Expiration) is for deletion. NIP-37 (Draft Wraps) is for storing unsigned drafts. The Nostr-native approach is client-side scheduling (the client holds the signed event and publishes it at the right time). Recurring already works this way — the CMS pre-signs the whole series.

---

## 4. Three Strategies (Corrected)

### Strategy A: Full Nostr-Native (Aggressive Refactor)

**What:** Remove all REST endpoints from swarm except Blossom + NIP-86. Move all analytics, dashboard, and content management to client-side nostr-cms using WebSocket subscriptions.

**Why it's NOT recommended:** This fights the maintainer's deployed architecture. `bitkarrot` merged the REST admin API upstream and wired `vercel.json` to it. They built the WS fallback in `AdminExplorer.tsx` (commit `28ca642`), suggesting they want REST-when-available, not REST removal. NIP-86 cannot replace `AdminRelayAccess` (no names CRUD). `useZapAnalytics` is single-pubkey, not a drop-in for the community leaderboard. This strategy requires confirming with `bitkarrot` before removing `vercel.json` or the REST paths.

### Strategy B: Split into Focused PRs (Corrected)

**What:** Accept the reviewer's breakdown. But recognize that most of PR #16 already landed on `zeabur-dashboard`. The remaining work is: (1) close PR #16 as superseded, (2) fix bugs in what already landed, (3) add NIP-86 as new work, (4) fix the nostr-cms components that call non-existent endpoints.

**Swarm PRs (against `zeabur-dashboard`):**
1. **PR 1: Trivial fixes** — fix bech32 alphabet (use `go-nostr/nip19`), fix `go s.save()` race condition, remove secrets from `getEnvironmentVars` entirely (not just mask). ~50-80 lines. *(NIP-98 dashboard login is deliberately split into its own PR — see §6.2 pipeline 1c — since it's a breaking change to the bundled dashboard UI.)*
2. **PR 2: NIP-86 relay management** — enable khatru's built-in NIP-86 `ManagementAPI` handlers for allow/ban kinds, IPs, list allowed/banned pubkeys. ~100-150 lines. NIP-98 auth. **Does NOT replace `AdminRelayAccess`** — complements it.
3. **PR 3: Video transcoding** (optional, separate) — `/process-video` endpoints with NIP-98 auth, ffmpeg in Dockerfile, in a `video.go` file. Separate concern.
4. **Reject/abandon:** `zap_stats.go` (external relay ingestion — if wanted, new PR with reviewer's concerns addressed), `/backfill-owners`, blob sidecar maps.

**nostr-cms PRs (new follow-up PRs against current `upstream/main`):**
5. **PR 4: Fix community zap stats** — `useCommunityZapStats` calls `/dashboard/zap-stats` which 404s on upstream. Either: (a) delete it and revert `AdminZaplytics.tsx` to use only `useZapAnalytics.ts` (lose community leaderboard), or (b) build client-side community aggregation consciously (loop N members × NIP-65 fan-out). **Ask `bitkarrot` which they prefer.**
6. **PR 5: Fix MyActivityCard** — calls `/dashboard/my-stats` which 404s on upstream. Rewrite to use client-side event queries via `nostr.query()` with `authors: [pubkey]`.
7. **PR 6: NIP-86 client support** (optional) — add a `useNip86` hook (~50 lines) for allow/ban kinds, IPs. **Does NOT replace `AdminRelayAccess` or `useNostrJsonUsers`** — those stay on REST for names CRUD.

**Why it's optimal:** Directly follows the reviewer's explicit guidance. Fixes real bugs in what already landed. Doesn't fight the maintainer's deployed architecture. NIP-86 is additive (new capability), not replacement (removing working code). The security fix (NIP-98 login) ships first.

**Trade-offs:** The community zap leaderboard may be lost (if `bitkarrot` doesn't want server-side analytics) or require real client-side work (if they do). `AdminRelayAccess` and `useNostrJsonUsers` stay on REST — but that's correct, because NIP-86 can't do names CRUD.

### Strategy C: Hybrid (Keep Server-Side Analytics, Fix Architecture)

**What:** Keep server-side zap analytics but fix the architectural violations. Don't ingest external events into the local relay DB. Use NIP-98 for dashboard auth. Use NIP-86 for kind/IP management.

**Why it's partially moot:** `zap_stats.go` never landed on `zeabur-dashboard`. "Keep server-side analytics" can't preserve what was never merged — it's a new PR to `zeabur-dashboard`. If `bitkarrot` wants server-side zap analytics, this is a new PR with the reviewer's concerns addressed (config flag, no `db.SaveEvent` of external events, use `go-nostr` NIP-57 utils, tests, cached results with TTL). If they don't, it's abandoned.

---

## 5. Recommendation

**I recommend Strategy B (Split into Focused PRs, Corrected).**

Reasons:

1. **It reflects reality.** Most of PR #16 already landed on `zeabur-dashboard`. PR #16 should be closed as superseded. The remaining work is bugfixes, NIP-86, and fixing nostr-cms components that call non-existent endpoints.

2. **It follows the reviewer's actual feedback.** The review was about bundling and per-item defects, not REST philosophy. The fixes address the specific defects: wrong bech32, `go s.save()` race, secret exposure, no-signature login, external relay ingestion.

3. **It doesn't fight the maintainer's deployed architecture.** `bitkarrot` merged the REST admin API and wired `vercel.json` to it. They built the WS fallback in `AdminExplorer.tsx`. Strategy B keeps REST where the maintainer wants it, adds NIP-86 as new capability, and fixes bugs.

4. **NIP-86 is additive, not replacement.** NIP-86 complements the existing REST admin API (allow/ban kinds, IPs) but cannot replace `AdminRelayAccess` (no names CRUD). Strategy B adds NIP-86 without removing working code.

5. **The security fix ships early.** The `/api/dashboard/login` no-signature vulnerability is live in production. Strategy B ships it as its own focused PR (pipeline 1c — NIP-98, same pattern `useScheduledPosts.ts` already uses), right after the trivial-fixes PR.

6. **The community zap stats decision is deferred to `bitkarrot`.** `useCommunityZapStats` calls an endpoint that 404s on upstream. Either delete it (lose the leaderboard) or build client-side aggregation (real work). Ask the maintainer before acting.

---

## 6. Deployment-Safe Pipeline

### 6.1 Principles

1. **buttercup.exe.xyz is never touched by PR work.** It runs from `deploy-latest` + `nostr-cms/dist` (built 2026-09-02) + the running `swarm-relay` binary (built 2026-09-01). Git branch work does not affect it. The deployed frontend bundle contains REST API calls, and the running relay binary serves those endpoints. This coupling remains intact until we explicitly redeploy.

2. **New PRs branch from the correct upstream base.** Swarm PRs branch from `zeabur-dashboard` (the repo default and PR #16 base). nostr-cms PRs branch from the current `upstream/main` (which contains our merged PRs #78/#79 + bitkarrot's follow-up fixes).

3. **Close PR #16 first.** It's superseded — most of its content already landed on `zeabur-dashboard` via other PRs.

4. **Ask `bitkarrot` before removing `vercel.json` or the `AdminExplorer` REST path.** They built the WS fallback and wired the proxy. They may want REST-when-available.

5. **Redeploy only when ready.** After upstream PRs merge, pull into a fresh branch, build, test, then deploy. Manual step, not automatic.

### 6.2 Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 0: SECURITY STOPGAP (urgent, independent)                 │
│                                                                 │
│  nginx: deny /api/dashboard/ and /api/admin/ entirely.          │
│  This takes the admin UI dark on buttercup until a real         │
│  NIP-98 fix ships. Right call: anyone on the internet can      │
│  currently forge the dashboard_session cookie (the cookie       │
│  value IS the public "_" pubkey) and rewrite nostr.json.       │
│  /api/scheduler/* stays open (NIP-98-gated server-side).        │
│  Vanilla nginx can't verify schnorr signatures — stopgap is     │
│  denial, not verification.                                      │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 1: SWARM PRs (against zeabur-dashboard)                  │
│  Note: pr-c-backend lives on HiveTalk/swarm (same-repo PR).     │
│  New branches can go on ButtercupRobrts/swarm (fork) or         │
│  HiveTalk/swarm (same repo) — either works.                     │
│                                                                 │
│  1a. Close PR #16 — superseded, not just rejected               │
│                                                                 │
│  1b. Branch pr-trivial-fixes from zeabur-dashboard              │
│      Fix bech32: replace npubToHex/hexToNpub with go-nostr/nip19│
│      Fix go s.save() race: make synchronous or use write queue  │
│      Fix getEnvironmentVars: remove secrets from list entirely  │
│      (These 3 are unrelated to each other but trivial and       │
│       low-risk — acceptable to bundle. ~50-80 lines.)          │
│      Push, open PR                                              │
│                                                                 │
│  1c. Branch pr-nip98-login from zeabur-dashboard                │
│      Add NIP-98 signature verification to /api/dashboard/login │
│      and all /api/dashboard/* + /api/admin/* handlers.          │
│      Update dashboard.html, dashboard_view.html, NostrLogin.js  │
│        to produce signed requests (kind 27235, u tag, payload   │
│        hash tag — khatru HandleNIP86 requires payload tag).     │
│      ASK BITKARROT FIRST: this breaks swarm.hivetalk.org's     │
│        bundled dashboard until its UI is updated.              │
│      This is a larger PR (~200-300 lines across Go + JS).       │
│      Push, open PR                                              │
│                                                                 │
│  1d. Branch pr-nip86 from zeabur-dashboard                     │
│      Enable khatru ManagementAPI handlers:                     │
│        AllowKind, DisallowKind, ListAllowedKinds,              │
│        BlockIP, UnblockIP, ListBlockedIPs                       │
│        (NOT AllowPubKey/BanPubKey — swarm's allowlist IS the   │
│         nostr.json names map; allowpubkey needs a separate     │
│         pubkey allowlist that swarm doesn't have)               │
│      RejectAPICall: only relay owner (from nostr.json "_")      │
│      Push, open PR                                              │
│                                                                 │
│  1e. (Optional) Branch pr-video from zeabur-dashboard            │
│      Cherry-pick /process-video endpoints from pr-c-backend     │
│      Add ffmpeg to Dockerfile, put in video.go                  │
│      NIP-98 auth                                                │
│      Push, open PR                                              │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 2: NOSTR-CMS PRs (against bitkarrot/nostr-cms main)      │
│  (branch from current upstream/main = dadc676)                  │
│                                                                 │
│  2a. Branch fork/pr-fix-community-zap from upstream/main        │
│      useCommunityZapStats calls /dashboard/zap-stats which 404s │
│      Option A: delete it, revert AdminZaplytics to useZapAnalytics│
│      Option B: build client-side community aggregation           │
│      ASK BITKARROT which they prefer before implementing         │
│      Push, open PR                                              │
│                                                                 │
│  2b. Branch fork/pr-fix-my-stats from upstream/main             │
│      Rewrite MyActivityCard.tsx to use nostr.query()             │
│        with authors: [pubkey], aggregate locally                │
│      Push, open PR                                              │
│                                                                 │
│  2c. (Optional) Branch fork/pr-nip86-client from upstream/main  │
│      Add useNip86 hook (~50 lines) for allow/ban kinds, IPs      │
│      Does NOT replace AdminRelayAccess or useNostrJsonUsers      │
│      Push, open PR                                              │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 3: UPSTREAM REVIEW & MERGE                               │
│                                                                 │
│  Swarm PRs reviewed by HiveTalk/swarm maintainers               │
│  nostr-cms PRs reviewed by bitkarrot/nostr-cms maintainers      │
│  Iterate on feedback                                            │
│  Merge when approved                                            │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 4: REDEPLOY (manual, when ready)                         │
│                                                                 │
│  4a. Pull zeabur-dashboard into fresh branch (swarm)            │
│      Pull upstream/main into fresh branch (nostr-cms)           │
│  4b. Build frontend: npm run build → dist/                      │
│  4c. Build swarm: go build → swarm-relay                        │
│  4d. Test: verify NIP-98 login, NIP-86 management,             │
│      verify scheduler works, verify no 404s on community tab    │
│  4e. Deploy: copy dist/ → nostr-cms/dist/,                       │
│       restart swarm-relay with new binary                       │
│  4f. Update nginx config if endpoint paths changed              │
│                                                                 │
│  REDEPLOY DELTA: pending scheduled posts survive (identical    │
│  file format). zap-stats / video / backfill and the community   │
│  tab disappear (if not merged upstream). Call this delta out    │
│  explicitly.                                                    │
└─────────────────────────────────────────────────────────────────┘
```

### 6.3 What Stays on buttercup.exe.xyz During Phases 0-3

| Component | Stays as-is? | Why |
|---|---|---|
| Frontend (`nostr-cms/dist`) | YES | Built artifact, not affected by git branches. Contains REST calls but the running relay serves them. |
| Relay binary (`swarm-relay`) | YES | Running process. Not rebuilt until Phase 4. Has scheduler, zap_stats, dashboard, video transcode. |
| `deploy-latest` branch | YES | Production tracking branch. Not modified by PR work. |
| `origin/main` / `github/main` | YES | Canonical deployed code. Not modified by PR work. |
| nginx config | YES (except Phase 0 stopgap) | Continues proxying `/api/`, `/dashboard`, `/process-video` to the running relay. |

### 6.4 Redeploy Delta (Phase 4)

When we redeploy after upstream merges, these features will **disappear** from buttercup.exe.xyz (because they're in our running binary but NOT on `zeabur-dashboard`):

| Feature | In our binary? | On `zeabur-dashboard`? | After redeploy |
|---|---|---|---|
| `zap_stats.go` (community zap stats) | YES | NO | **Gone** (unless new PR merges) |
| Video transcoding (`/process-video`) | YES | NO | **Gone** (unless PR 1d merges) |
| `/backfill-owners` | YES | NO | **Gone** (one-time migration, not needed) |
| Blob owner/mime sidecar maps | YES | NO | **Gone** (unless separate PR merges) |
| `/stats`, `/events`, `/my-stats` endpoints | YES | NO | **Gone** (AdminExplorer WS fallback covers this) |

These will **survive** (already on `zeabur-dashboard`):

| Feature | After redeploy |
|---|---|
| Scheduler (533 scheduled posts) | **Survives** — identical file format |
| Dashboard (`/api/dashboard/*`, `/api/admin/*`) | **Survives** — already upstream |
| `NostrLogin.js` | **Survives** — already upstream (unless replaced) |
| Rate limiters | **Survives** — already upstream |

Call this delta out explicitly before redeploying.

### 6.5 Dependencies Between PRs

```
Phase 0 (nginx security stopgap) ──→ independent, do now

swarm PR 1 (bugfixes)     ──→ independent, ship first (includes NIP-98 login fix)
swarm PR 2 (NIP-86)       ──→ independent
swarm PR 3 (video)        ──→ independent, optional

nostr-cms PR 1 (community zap) ──→ ask bitkarrot first
nostr-cms PR 2 (my-stats)    ──→ independent
nostr-cms PR 3 (NIP-86 client)──→ depends on swarm PR 2 (NIP-86 server)
```

No hard dependencies except: **swarm PR 2 (NIP-86 server) must merge before nostr-cms PR 3 (NIP-86 client) can work.** Everything else is independent.

### 6.6 Risk Mitigation

| Risk | Mitigation |
|---|---|
| `bitkarrot` wants to keep REST paths | Strategy B keeps REST where it already exists. Only adds NIP-86 and fixes bugs. Ask before removing anything. |
| Community zap leaderboard lost | Ask `bitkarrot`: delete it (lose leaderboard) or build client-side aggregation (real work). Don't assume. |
| Client-side zap analytics too slow for large communities | `useZapAnalytics` has progressive loading. If insufficient, separate PR for cached aggregation (without external relay ingestion). |
| Scheduler doesn't work when client is offline | Custodial scheduler already upstream. Recurring works client-side (CMS pre-signs series). One-shot scheduler stays server-side. |
| Security vulnerability exploited before fix | Phase 0 nginx stopgap: deny `/api/dashboard/` and `/api/admin/` entirely. Takes admin UI dark but prevents `nostr.json` rewriting. `/api/scheduler/*` stays open (NIP-98-gated). |

### 6.7 Correction Log

**Revision 2 (wrong-branch fix):**

| Original claim | Correction |
|---|---|
| Audited `beeswax` as upstream | Should have audited `zeabur-dashboard` (PR #16 base, repo default, 73 commits ahead of `beeswax`) |
| "Upstream swarm has no REST/dashboard" | False on `zeabur-dashboard`: has `/api/dashboard/*`, `/api/admin/*`, scheduler, `NostrLogin.js`, `npubToHex` |
| Strategy B PR 1 (scheduler) and PR 2 (small fixes) | Already upstream — nothing to split |
| "Close PR #16" because rejected | Close because **superseded** |
| Reviewer said "no need for REST APIs" | Not in the review text. That was the user's paraphrase. Review is about bundling and per-item defects. |
| "Reviewer approved the scheduler with recurring post support" | `bitkarrot` retracted: "The scheduler's intended scope was always one-shot." Recurring works client-side (459 posts with repeat tags). |
| NIP-86 replaces `AdminRelayAccess`/`useNostrJsonUsers` | **Cannot.** NIP-86 has no names field. Names CRUD stays on REST. |
| `useZapAnalytics` is a drop-in for community tab | **Not equivalent.** Single-pubkey (`#p: [pubkey]`) vs. all-members leaderboard. Client-side equivalence is real work. |
| "Keep the auth check via `useAdminAuth`" | `useAdminAuth` exists (from `useRemoteNostrJson`) but reads `nostr.json`, not REST. `AdminExplorer`'s `useAdminApi` calls REST `/login`. A WS-native explorer arguably needs no auth (only sees public relay data). |
| Strategy C "keep server-side analytics" | Partially moot — `zap_stats.go` never merged. It's a new PR, not preserving existing code. |
| `getEnvironmentVars` "leaks secrets" | Masks values (`***MASKED***`) but exposes presence/absence. Real problem is the session model (no signature verification). |
| "Remove `vercel.json` proxy" | Maintainer created it. Ask before removing. |

**Revision 3 (security stopgap + kind number + bundling fix):**

| Original claim | Correction |
|---|---|
| Phase 0: "restrict `/api/dashboard/login` to local-only" | **Ineffective.** The cookie is forgeable without calling `/login` — `requireAdminSession` just compares `cookie.Value` to the public `"_"` pubkey. Blocking `/login` is cosmetic. Real stopgap: deny `/api/dashboard/` and `/api/admin/` entirely. |
| Phase 0: "signature check at the proxy level" | Not realistic in vanilla nginx (no schnorr verification without njs/Lua). Stopgap is denial, not verification. |
| `fetchWithNip98` uses kind 24242 | **Wrong.** Uses kind 27235 (`useScheduledPosts.ts:41`, `scheduler.go:579`). 24242 is the Blossom auth kind. |
| `fetchWithNip98` can be reused for NIP-86 | **Cannot as-is.** Sends `u` + `method` tags but no `payload` hash tag. khatru `HandleNIP86` requires `payload` tag with SHA-256 of request body. `useNip86` hook must add it. |
| PR 1 "Bugfixes" bundles bech32 + `save()` + env vars + NIP-98 login | **Repeats the "unrelated changes" pattern.** Split: trivial fixes (bech32, `save()`, env vars) ride together; NIP-98 login is a separate, larger PR (~200-300 lines across Go + JS, breaks bundled dashboard UI). |
| NIP-86 "NOT AllowPubKey/BanPubKey — those need names CRUD" | **Wrong reason.** `allowpubkey` doesn't need names CRUD — it needs a separate pubkey allowlist consulted by the event-reject path, which swarm doesn't have (swarm's membership = `nostr.json` names map). Same conclusion (don't implement `AllowPubKey`/`BanPubKey`), correct reason: no separate allowlist to write to. |
| `useCommunityZapStats` has "4 fetch/dashboard calls" | **2 fetch calls** (login + zap-stats). The grep count of 5 included comments and error strings. |
| NIP-98 login fix is server-only | **Breaking change.** `dashboard.html`/`dashboard_view.html`/`NostrLogin.js` call `/api/dashboard/login` with just `{pubkey}` and use the session cookie for subsequent requests. Adding NIP-98 breaks the bundled dashboard UI until updated. Ask `bitkarrot` before shipping — breaks `swarm.hivetalk.org`. |
| `pr-c-backend` is on our fork | **Same-repo PR.** `headRepositoryOwner.login = HiveTalk`. New branches can go on `ButtercupRobrts/swarm` (fork) or `HiveTalk/swarm` (same repo). |
