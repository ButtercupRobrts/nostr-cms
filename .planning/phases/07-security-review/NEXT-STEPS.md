# NEXT — ordered plan (as of 2026-10-01, post-hardening)

State: buttercup prod hardened at edge (GET-only on admin/dashboard paths, rate limits live, body cap 256m, :3334 non-loopback dropped). Maintainer report delivered.

## A. Pending — awaits maintainer
- Swarm session-model fix (Finding 1). On release: pull swarm, re-run probe battery (proof-free /login must 401; forged cookie must 401; users must require session; verify writes), then decide whether to lift `limit_except GET` in nginx.
- Possible `BIND_ADDR`/`IP_RATE_LIMIT`/`ALLOWED_MIRROR_HOSTS` knobs upstream.

## B. Ours — ready now, no dependency
1. Persist the iptables rule (iptables-persistent or systemd drop-in) — currently reboot-fragile.
2. Upstream PR: non-sensitive batch — `VITE_ENABLE_NSEC_LOGIN` gate, sha256-verify on blob fetches, blossom-empty hard error (+ warn variant for upstream). No exploit detail.
3. Restart smoke: reboot resilience check — confirm service start order, dist intact, rate-limit conf loads.

## C. Follow-ups
4. Write-surface characterization on a disposable swarm instance (forged cookie → POST user/*) — fills the report's unverified row.
5. ncryptsec (NIP-49) login PR upstream — long-term safe nsec UX.
6. Dashboard XSS heads-up to maintainer (profileTabLoaders unescaped innerHTML) — held out of report per scope; decide whether to share.
7. Watch dependabot merges: #98 noble/hashes (breaks sha256 subpath), #92 nostrify 0.2→0.6 — verify before next converge.

## D. Deferrable / backlog
- iOS Safari large-video upload check (needs device)
- useUploadFile callers (avatar) browser test
- processVideosOnUpload persistence toggle
- Media-selector transcode option
- dist.backup retention trimming (5 kept)
- git reflog expire — done
- key rotation: NOT needed (master never entered page — verified)

## Rollback anchors
- `buttercup/deployed-20260928` (pre-converge snapshot commit, on fork)
- `buttercup/converge-main` (fork, current)
- `nostr-cms/dist.backup-*` (5 newest kept)
- nginx backup `meetup-space.bak-20261001`; iptables rule delete: `iptables -D INPUT 1`
