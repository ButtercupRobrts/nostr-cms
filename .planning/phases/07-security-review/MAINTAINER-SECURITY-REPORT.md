# Security report — swarm + nostr-cms (for HiveTalk maintainers)

**Date:** 2026-10-01 (rev. 2 — all claims re-verified by live probe)
**Reporter:** Buttercup deployment (buttercup.exe.xyz — swarm + nostr-cms on exe.dev)
**Scope of testing:** read-only probes against our own production deployment. GETs/OPTIONS and auth probes only; no data created, modified, or deleted. No write-endpoint payloads were executed.
**Contact channel:** [fill in — suggest private channel, not a public issue, for Finding 1]

---

## TL;DR

swarm's `/api/admin/login` issues an admin session for any pubkey in the admin/nostr.json map **with no proof of key possession** (bare JSON `{"pubkey": ...}`). The session cookie is `dashboard_session=<pubkey>` — a self-asserted value. Independently, at least one allow-listed endpoint (`/api/admin/users`) answers with **no authentication at all**, and `/login` currently also sits reachable on swarm's directly-bound port (`*:3334`, all interfaces). The combined blast radius depends on topology: anywhere `:3334` is routable, the full cookie-authenticated admin surface is open to anyone — admin pubkeys are public by design (nostr.json). Root fix belongs in swarm.

---

## Finding 1 — CRITICAL: proof-free session issuance + weak auth model (swarm)

### Reproduced on our deployment (commands are generic — run against any instance)

```bash
# Admin pubkeys are public by design:
curl -s https://<host>/.well-known/nostr.json

# (a) Proof-free session issuance — observed directly on swarm:
curl -X POST http://<swarm-host>:3334/api/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"pubkey":"<admin-pubkey-hex>"}'
# → 200 OK; Set-Cookie: dashboard_session=<pubkey>; HttpOnly; Max-Age=3600
#   No signature, challenge, or NIP-98 event required.

# (b) Unauthenticated read — no cookie/session at all:
curl https://<host>/api/admin/users
# → 200 with the users map. Verified: same response with no Cookie header.

# (c) Write paths DO enforce session checks:
curl -X POST http://<host>/api/admin/user/<pk> -d 'null'   # → 401 (pre-handler)
curl -X OPTIONS http://<host>/api/admin/user/<pk>          # → 401
```

### Verified behavior matrix (probed live)

| Endpoint | No cookie | Forged cookie only¹ | Session created via proof-free `/login` |
|---|---|---|---|
| `GET /api/admin/users` | **200** | 200 | 200 |
| `GET /api/admin/stats` | 401 | 401 | 200 |
| `GET /api/dashboard/zap-stats` | 401 | 401 | 200 |
| `GET /api/admin/environment` | — | 401 | 200 |
| `GET /api/dashboard/my-stats` | — | 401 (random pubkey) | — |
| `POST /api/admin/user/<pk>` | **401** | — | — |
| `OPTIONS /api/admin/user/<pk>` | **401** | — | — |

¹ "Forged cookie" = `Cookie: dashboard_session=<pubkey>` where no `/login` call was ever made for that pubkey. Note the cookie's value IS the pubkey — forging it requires no secret material.

### Root cause (observed)

1. `/api/admin/login` accepts a self-declared pubkey (checked against the admin set — random pubkeys correctly 401) and issues a session **without NIP-98/challenge proof of possession**.
2. The session artifact is forgeable (`dashboard_session=<pubkey>`), though most sensitive endpoints appear to consult server-side session state (a forged cookie alone did not authenticate them).
3. `GET /api/admin/users` performs **no auth check whatsoever** (serves the nostr.json names map — already public data, but evidence the auth middleware isn't uniformly applied).

### What we did NOT verify (stated honestly)

- Whether write endpoints accept a forged `dashboard_session` cookie for a pubkey that has a live server-side session — we did not execute writes against production.
- Whether `/mirror` (POST returned 405 unauthenticated), `/api/token`, `/api/dashboard/user/*`, `/list_manual`, `/dashboard_view` (names from binary strings) enforce auth in all methods.
- Whether NIP-98 `expiration` tags are enforced on scheduler paths.

### Exposure by topology

| Deployment shape | Exposure |
|---|---|
| Reverse proxy denying `/login` + allow-listing read paths (our nginx front) | `/api/admin/users` readable unauthenticated (public data). Session-required endpoints unreachable unless a session exists; `/login` is edge-denied. **Residual: small but real.** |
| swarm reachable directly at `:3334` (swarm binds `*`) | Anyone who can route to the port can mint admin sessions → all session-checked endpoints open → unverified write surface. On our VM: port binds all interfaces, no host firewall rules observed; on exe.dev the port is not publicly proxied, but any local process/pivot can reach it. |
| No proxy in front at all (e.g., VPS deploys per oneshot "General VPS" flow) | Full chain internet-exposed. |

### Recommended fix (swarm)

1. `/api/admin/login` should require a NIP-98 kind:27235 event (verify signature, `u` = endpoint URL, fresh `created_at`, `expiration` tag, pubkey ∈ admin set) — the same machinery already verifying `/api/scheduler/*`. `HiveTalk/dashboard`'s `src/pages/api/auth/login.ts` is a working reference implementation (challenge nonce + signed event + opaque token).
2. Issue opaque random session tokens (≥128-bit), stored server-side; never derive the session artifact from the pubkey alone.
3. Apply the session check uniformly — `GET /api/admin/users` currently skips it.
4. Bind to `127.0.0.1` by default (`RELAY_PORT` is configurable; no bind-address env exists — add one, e.g. `BIND_ADDR`).
5. Audit every cookie-authed endpoint's write methods before assuming read-only blast radius.

## Finding 2 — `ALLOWED_MIRROR_HOSTS=*` (SSRF-adjacent, verify)

Our deployment's `swarm/.env` sets `ALLOWED_MIRROR_HOSTS=*`. `/mirror` fetches remote blobs server-side; if the fetch honors arbitrary hosts, an authenticated caller could direct swarm at internal addresses (metadata endpoints, localhost services). We did not probe `/mirror` with a real auth event. Recommend an explicit allowlist default and documentation.

## Finding 3 — `IP_RATE_LIMIT` supported but unset

The binary references an `IP_RATE_LIMIT` env var (present in its env-name table) that is not set in our `.env`. Semantics undocumented — recommend documenting it (and its default) in deploy docs.

## Finding 4 — nostr-cms: raw nsec persisted in localStorage (HIGH)

`@nostrify/react` `NLogin.fromNsec` → `useNostrLoginReducer` serializes login state — including the secret — to `localStorage['nostr:login']`. Reachable via the LoginDialog key tab (the default tab), `.txt` import, and the signup key-generation flow (which produces an `nsec.txt` download designed to be re-imported). Any XSS or compromised dependency = full key theft. NIP-49 `ncryptsec` is the standard mitigation for at-rest storage.

**Also affects HiveTalk/dashboard** (verified in its repo): same `@nostrify/react` persistence path — nsec logins land in `localStorage['nostr:login']` identically, and `NostrCreateAccountDialog.tsx` exports a raw `nostr-backup.txt` nsec.

**Our local mitigation (upstreamable):** env-gated the raw-nsec UI behind `VITE_ENABLE_NSEC_LOGIN` (default off in prod builds) — hides the key tab, `.txt` import, and signup key-gen. Policy control, not a sandbox — the mechanism remains bundled. Long-term answer: accept `ncryptsec` + session-scoped decryption, or default to extension/bunker.

## Secondary observations

- **No rate limiting** on public endpoints in our nginx (`limit_req`/`limit_conn` absent); `client_max_body_size 1024m` on `/upload`, `/mirror`, `/process-video*` — while swarm's own `MAX_UPLOAD_SIZE_MB=200` caps at 200 MB. The edge should match (~256 MB) to avoid accepting 5× oversized uploads the backend will reject anyway.
- `/process-video*` is auth'd (401 unauth ✓) but CPU-heavy per call — worth a job-concurrency cap swarm-side.
- **StaticPage fetched Blossom blobs by sha256 without verifying the hash** — a hostile/misconfigured blossom server could substitute content under a trusted page's hash (bounded by DOMPurify sanitization). Patched locally (verify hash, skip-to-next-server on mismatch); upstreamable.
- **Silent `blossom.primal.net` fallback** in `useUploadFile` when no relay list is configured — silently ships uploads to a third-party public server. Ours now hard-errors; suggest warn-or-error upstream. **Also present in HiveTalk/dashboard** (`src/lib/nostr/hooks/useUploadFile.ts` hardcodes primal.net).
- dompurify 3.4.13 → GHSA-p98j-92pf-mc4p (low; `IN_PLACE` mode only, unused by the CMS) — dependabot #97 covers it.
- **Supply-chain watch on open dependabot PRs**: `#98` (@noble/hashes 1→2) likely breaks the `@noble/hashes/sha256` subpath import used by streaming uploads; `#92` (@nostrify/react 0.2→0.6) touches ~81 import sites. Neither should merge unverified.

## Verified sound (actively probed, for completeness)

- NIP-98 scheduler endpoints: unauth → 401, wrong-key → 401, owner-scoped; relative `u` tag rejected
- Blossom: `/upload` → 401 unauth; blob `DELETE` → 403 wrong-key
- Relay WS writes: non-team pubkey → `["OK", …, false, "blocked: you are not part of the team"]`
- `/api/admin/login` with non-admin pubkey → 401 (membership check works)
- `POST`/`OPTIONS` on `/api/admin/user/*` → 401 without session (auth check runs pre-handler)
- `my-stats`/`environment`: forged cookie → 401 (server-side session enforcement exists — the pattern to extend)

## Suggested disclosure handling

Finding 1 is exploitable by anonymous parties wherever `:3334` (or the session endpoints) are reachable. Recommend private fix + release, then public advisory. Happy to coordinate timing; full internal threat-model detail available on request.
