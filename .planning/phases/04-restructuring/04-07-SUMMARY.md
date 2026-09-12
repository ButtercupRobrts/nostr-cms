# 04-07 Summary: Remove nsec login from NostrLogin.js (Option A)

**Date:** 2026-09-12
**Status:** Complete — PR open

## Outcome
- **PR #19** on HiveTalk/swarm → base `zeabur-dashboard`, head `pr-nip07-only` (clean base at `a30fb61`, not stacked on pr-trivial-fixes)
- Files: `NostrLogin.js` (−161 net), `nostr-tools.bundle.js` (new, vendored 212KB), `dashboard.html`, `convert.html`, `dashboard_view.html` (1-line script tags each)
- Verified: `node --check` clean, `go build` clean, zero remaining nsec/privkey/shim references

## What was removed / fixed
- `loginWithNsec`, `setWindowNostrFromNsec` shim (silent signEvent — the core risk), nsec dialog UI + listeners, dead helpers (`sha256Hex`/`bytesToHex`/`hexToBytes`/`_nativeWindowNostr`), `_loginMethod`/`_privkeyHex` state
- Fixed residual `ReferenceError` path: `openDialog` referenced `_loginMethod` and dead `#nl-nsec-*` elements — simplified to `!!window.nostr`
- vendored `nostr-tools@2.10.4` (sha256 `dd2acfb3…4a38eeb`, published in PR body for byte-verify) — removes unsigned unpkg script from auth pages
- Latent bug fixed: `dashboard_view.html` used `NostrTools` with no loader — now includes the vendored bundle

## Follow-up (separate PR, not in scope)
- NIP-46 `bunker://` remote-signer login — only if bitkarrot wants mobile/extension-less admin sign-in
