# NEXT — ordered plan (as of 2026-10-02, post-reconverge)

## Current state

- **Prod deployed**: `index-Db6-1qH-.js` at merge `748e944` — upstream deps (vaul, dompurify, radix, query, tiptap-removal) + our security batch, all verified live (byte-level blob check, WS, auth paths)
- **Prod hardening**: nginx GET-only on admin/dashboard paths, rate zones live, body cap 256m; iptables `:3334` rule persisted via `swarm-localhost-guard.service`
- **Upstream**: #101 #102 #103 #91 merged; #97/#98/#99 closed with rationale (noble v2 declined — Node floor); only #92 (nostrify 0.2→0.6 migration) remains open/deferred
- **Maintainer report**: rev2 delivered; awaits swarm fix

## A. Pending — awaits swarm maintainer
- C1 session fix: on release → pull swarm → re-run probe battery (bare-pubkey /login must 401; forged cookie must 401; users must require session) → lift nginx `limit_except` if verified → update report's unverified rows
- `BIND_ADDR`/`IP_RATE_LIMIT`/`ALLOWED_MIRROR_HOSTS` upstream decisions

## B. Ours — ready anytime
1. **#92 nostrify migration** — the only remaining upstream dep item; dedicated branch + full test pass, not a merge-button job. Defer until maintainer appetite.
2. **Dashboard heads-up** (drafted): `kick-user.ts` token logging + `profileTabLoaders` unescaped innerHTML — send when convenient.
3. Optional: disposable-swarm write-surface probe → fills report's last unverified row.
4. Optional docs PR upstream: reference hardened nginx conf + "one lockfile" + "majors need compat checks" note.

## C. Ongoing hygiene
- Watch exe.dev edge: the "upload sometimes fails" incident was client-connectivity (`ERR_NETWORK_CHANGED` + 499s — swarm healthy, ffmpeg kills = client disconnects). If it recurs on a stable connection → report stream drops to exe.dev.
- Monitor `:8765` report-share — kill when sharing done.
- Dependabot: future majors → check engines/API compat before merge (rule established).

## Rollback anchors
- `buttercup/pre-reconverge-20261001` (pre-merge snapshot, on fork)
- `buttercup/deployed-20260928` (pre-converge snapshot)
- `nostr-cms/dist.backup-*` (newest kept)
- nginx `meetup-space.bak-20261001`; iptables rule via `swarm-localhost-guard.service` (disable unit to revert)
