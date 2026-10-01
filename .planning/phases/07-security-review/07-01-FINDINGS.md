# 07-01 Security Review — nostr-cms / buttercup.exe.xyz

**Date:** 2026-09-28 · **Scope:** full repo (src/, hooks/, lib/, server/nginx config, deps) · **Method:** 5-pass audit, evidence-linked, black-box server tests on live :8000

## Severity-ordered findings

### HIGH

**H1. nsec login persists plaintext private key to localStorage**
- `NLogin.fromNsec()` stores `{ nsec }` in the login payload → `useNostrLoginReducer` writes it to `localStorage['nostr:login']` on every state change (`node_modules/@nostrify/react/dist/login/useNostrLoginReducer.js:9`).
- Reachable via `LoginDialog` nsec field *and* the `.txt` file import (`LoginDialog.tsx:151`).
- Any XSS, any compromised dependency, any injected script = full key theft. Extension (NIP-07) and bunker (NIP-46) logins are unaffected — key never enters the page.
- **Mitigation:** on buttercup, prefer/force extension or bunker for admin accounts; consider disabling nsec login in prod builds; upstream-worthy discussion.
- Corollary: in-page nsec signers sign `signEvent` silently — no per-operation confirmation.

### MEDIUM

**M1. StaticPage fetches blob content by sha256 without verifying the hash**
- `src/pages/StaticPage.tsx` `fetchFromBlossom` GETs `${server}${sha256}` and uses the body as-is. A malicious/misconfigured Blossom server can return arbitrary content for a known hash → injects (DOMPurify'd) content into public pages. Worse if a browser's relay list drifts to `blossom.primal.net` fallback.
- **Fix:** sha256 the response bytes and compare to the tag before use (~10 lines, @noble/hashes already imported).

**M2. Blossom relay list is per-browser localStorage config, silent public fallback**
- `useBlossomRelays` reads `config.siteConfig.blossomRelays` (localStorage); empty → uploads default to `blossom.primal.net`. A newly logged-in admin browser silently ships media to a public third-party server.
- **Fix:** warn when the list is empty instead of silent fallback.

### LOW

- **L1.** dompurify 3.4.13 — GHSA-p98j-92pf-mc4p (low; only exploitable via `IN_PLACE` mode, which we don't use). `npm audit fix` or dependabot PR #97 resolves.
- **L2.** DOMPurify `ADD_TAGS: ['iframe','style','svg']` on PageContent/StaticPage — permissive sanitizer surface; content is publisher/master-gated so exposure is bounded, but the svg+style+iframe trio is historically the bypass-heavy combination. Consider tightening if not needed.
- **L3.** `GET /list/<pubkey>` returns blob enumeration unauthenticated — BUD-04 standard behavior; informational.
- **L4.** Unauthenticated blob DELETE returns 502 (proxy error) instead of 401 — cosmetic.
- **L5.** `/api/health` public — trivial disclosure only.
- **L6.** NIP-98 `expiration` tag enforcement on swarm — unverified (didn't test expired-token acceptance).

### VERIFIED SAFE

- **Rendering**: untrusted markdown/nostr content → `rehype-raw` + `rehypeSanitize` with tight `defaultSchema` + only `nostr-embed`; `javascript:` URIs blocked by default; `chart.tsx` innerHTML is theme CSS only; CSP `script-src 'self'` (post-converge) blocks inline-script execution as a second layer.
- **Server authz (black-box verified)**: upload 401 unauth; blob DELETE 403 wrong-key / 502 unauth (not a hole); scheduler list 401 unauth AND wrong-key (owner-scoped); `/api/admin/*` + `/api/dashboard/*` 401 or nginx-denied; `/mirror` 405; `/backfill-owners` + `/dashboard` nginx-denied.
- **Client authz layering**: AdminProtection is UI-only BUT real enforcement is server-side auth + author filtering — correct pattern (e.g., anyone can *publish* a kind-34128 page event, but rendering filters to master/publisher authors).
- **Logs**: no key/token/event-signature material in console output paths.
- **Auth tokens**: Blossom 24242 uploads carry 15-min expiry; NIP-98 tokens are per-request.
- **Supply chain**: 0 npm vulnerabilities post-converge (minus L1); only esbuild carries a postinstall script — minimal lifecycle surface.

### WATCH (upstream decisions)

- Dependabot **#98** `@noble/hashes` 1→2: likely breaks `@noble/hashes/sha256` subpath import in `mediaProcessing.ts` (streamUpload) — verify before merge.
- Dependabot **#92** `@nostrify/react` 0.2→0.6: ~81 import sites; needs changelog review, not a blind merge.

## Recommended actions (priority order)

1. Decide nsec-login policy for buttercup (extension/bunker only?) — H1
2. Add sha256 verification to `fetchFromBlossom` — M1
3. Warn-on-empty blossom relay list instead of primal.net fallback — M2
4. `npm audit fix` dompurify — L1
5. Review #98/#92 before next converge — watch
