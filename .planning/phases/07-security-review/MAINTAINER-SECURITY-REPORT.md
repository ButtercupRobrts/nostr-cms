# Security report — swarm + nostr-cms (for HiveTalk maintainers)

**Date:** 2026-10-01
**Reporter:** Buttercup deployment (buttercup.exe.xyz — swarm + nostr-cms stack on exe.dev)
**Scope of testing:** read-only probes against our own production deployment only. No data modified; GETs and auth probes only.
**Contact channel:** [fill in — suggest a private channel rather than a public issue for C1]

---

## TL;DR

swarm's dashboard session endpoint (`POST /api/admin/login`) issues an admin session for any pubkey present in the admin/nostr.json map **without requiring proof of key possession**, and the resulting cookie (`dashboard_session=<pubkey>`) is self-declarable — it can be forged client-side without ever calling login. The blast radius varies by deployment: on deployments where swarm's port (`:3334`, bound to all interfaces by default) is reachable directly, the full cookie-auth'd admin API surface is open to anyone who knows an admin pubkey — which is public information by design (nostr.json). Root fix belongs in swarm.

---

## Finding 1 — CRITICAL: proof-free admin session issuance + forgeable session cookie (swarm)

### Evidence (reproduced on our own deployment)

```bash
# 1. Obtain any admin pubkey (public by design):
curl -s https://<host>/.well-known/nostr.json

# 2. Request a session with NO signature, proof, or secret:
curl -X POST http://<swarm>:3334/api/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"pubkey":"<admin-pubkey-hex>"}'
# → HTTP 200, Set-Cookie: dashboard_session=<pubkey>; HttpOnly; Max-Age=3600
#    The cookie value IS the pubkey — no opaque token, no server-side proof binding.

# 3. Forge the cookie directly (no login call needed):
curl -H 'Cookie: dashboard_session=<admin-pubkey>' \
     https://<host>/api/admin/users
# → HTTP 200 with the users map — even with /api/admin/login denied at the proxy.
```

Verified responses on our instance (all via forged/session-less cookie or proof-free login): `/api/admin/users` → 200 (works with a forged cookie, no session needed — appears to check only "pubkey ∈ admin map"); `/api/admin/stats`, `/api/dashboard/zap-stats` → 200 when a server-side session exists for that pubkey (created via the proof-free `/login`); `/api/admin/environment`, `/api/dashboard/my-stats` → correctly rejected forged cookies (401) — they enforce a real session / stricter checks.

Unprobed: write endpoints under the cookie path (`/api/admin/user/*` methods beyond GET, `/api/token`, `/api/dashboard/user/*`). We did not mutate production; blast radius for writes is unknown and worth checking before assuming read-only.

### Root cause (as observed)

Two layered issues in swarm:

1. `POST /api/admin/login` treats a JSON-declared pubkey as authenticated identity. There is no NIP-98 / challenge-response proof that the caller controls the key.
2. The session artifact is `dashboard_session=<pubkey>` — a self-asserted value the client can set without the server's participation. Endpoints that check "cookie pubkey ∈ admin map" (e.g., `/api/admin/users`) are forgeable outright; endpoints checking server-side session state are only gated by the (proof-free) login.

Note: `/login` does reject pubkeys not in the admin set (401 on random pubkey — verified), so it is checking membership — just not possession.

### Exposure by deployment

| Topology | Exposure |
|---|---|
| nginx hardening in front (our exe.dev deploy) | `/login` denied at edge, BUT forged `dashboard_session` cookie still reaches allow-listed paths (`/api/admin/users` → 200 verified). Endpoints requiring server-side session state are masked only until a session exists (or swarm is reached directly). |
| swarm bound to all interfaces, no host firewall (observed default: `*:3334`) | Any locally-routable process/container can mint admin sessions; publicly routable if the host exposes the port. |
| Reverse-proxy-free deployments (e.g., oneshot "General VPS" if no hardening layer) | Full chain internet-exposed: bare-pubkey login → session → all cookie-authed endpoints. |

### Recommended fix (swarm)

1. Require a NIP-98 kind:27235 event in the `/login` request body (the codebase already verifies these for `/api/scheduler/*` — same machinery). Verify signature, `u` tag = endpoint URL, recent `created_at`, `expiration` tag, and pubkey ∈ admin set.
2. Issue **opaque random session tokens** (≥128-bit) stored server-side; never derive session artifacts from the pubkey alone.
3. Bind to `127.0.0.1` by default (or document that :3334 must never be publicly exposed).
4. Audit every cookie-auth'd endpoint for write paths; the session check should be uniform (`environment`/`my-stats` already do stricter checks — extend that pattern).

## Finding 2 — swarm listens on all interfaces

`swarm-relay` bound `0.0.0.0:3334` on our VM; host firewall had no restricting rules (`iptables INPUT policy ACCEPT`). Anyone who can reach the port bypasses nginx entirely — including its `/login` deny. Recommend default-bind localhost; deployments needing remote swarm should opt in explicitly.

## Finding 3 — nostr-cms: raw nsec persisted in localStorage (HIGH)

`@nostrify/react` `NLogin.fromNsec` → `useNostrLoginReducer` serializes login state — including the secret — to `localStorage['nostr:login']`. Reachable via the LoginDialog key tab (default tab), `.txt` import, and the signup key-generation flow (which produces an `nsec.txt` download designed to be re-imported). Any XSS or compromised dependency = full key theft.

Ecosystem comparison: the `HiveTalk/dashboard` codebase uses the same nostrify path (same exposure), and its guest/LiveKit flows intentionally store only disposable ephemeral keys — that "only disposable keys in localStorage" model is the right rule. NIP-49 `ncryptsec` (scrypt + XChaCha20-Poly1305) is the standard mitigation for at-rest key storage.

**Our local mitigation (upstreamable):** env-gated the raw-nsec UI surface behind `VITE_ENABLE_NSEC_LOGIN` (default off in prod builds) — hides the key tab, file import, and signup key-gen. Policy control only — the mechanism remains bundled. Can package as an upstream PR with docs recommending extension (NIP-07) / bunker (NIP-46) for admins; longer-term, accepting `ncryptsec` + session-scoped decryption is the defensible nsec UX.

## Secondary observations

- **No rate limiting observed** on public endpoints in our nginx config (`limit_req`/`limit_conn` absent); `client_max_body_size 1024m` on `/upload`, `/mirror`, `/process-video*`. Auth-required still means "any valid key" for 24242 paths — resource exhaustion is feasible. Suggest documenting a reference nginx rate-limit block in the deploy docs.
- `/process-video*` requires auth (401 unauth ✓) but is CPU-heavy per call — consider job concurrency caps swarm-side.
- **StaticPage fetched Blossom blobs by sha256 without verifying the hash** — a hostile/misconfigured blossom server could substitute content for a trusted page's hash (bounded by DOMPurify sanitization). We patched locally (hash-verify before render, skip-to-next-server on mismatch); upstreamable.
- **Silent `blossom.primal.net` fallback** in `useUploadFile` when no relay list is configured — silently ships uploads to a third-party public server. We changed ours to a hard error; suggest warn-or-error upstream.
- dompurify 3.4.13 → GHSA-p98j-92pf-mc4p (low; `IN_PLACE` mode only, unused by the CMS) — dependabot #97 covers it.

## Verified sound (for completeness — these were actively probed)

- NIP-98 scheduler endpoints: unauth 401, wrong-key 401, owner-scoped; relative-`u` tag rejected
- Blossom: `/upload` 401 unauth; blob `DELETE` 403 wrong-key
- Relay WS writes: non-team pubkey → `["OK", …, false, "blocked: you are not part of the team"]`
- `/api/admin/login` with non-admin pubkey → 401 (membership check works)
- `my-stats`/`environment`: forged cookie → 401 (proper session enforcement exists — the pattern to extend)

## Suggested disclosure handling

- Finding 1 is exploitable by anonymous parties against any deployment exposing `:3334` (or the allow-listed admin paths with forgeable cookies). Recommend private fix + release, then public advisory; happy to coordinate timing.
- Full technical detail (trust boundaries, entry-point inventory, authz map with file:line cites) available in our internal report if useful.
