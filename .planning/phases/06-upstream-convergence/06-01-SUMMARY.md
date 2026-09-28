# 06-01 Summary — Converge buttercup onto hivetalk/main

**Status:** ✅ Executed 2026-09-28. Live bundle `index-DRfpXLeL.js`, backup `dist.backup-20260928-161056`.

## What happened

- `git merge hivetalk/main` onto branch `buttercup/converge-main` (parent: snapshot `cf9dcb5` on `buttercup/deployed-20260928`).
- **Conflicts (as predicted by `git merge-tree` dry-run)**: `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml` — all resolved take-theirs; upstream's dep set was a strict superset of ours (our security pins went upstream via #85/#87). `useScheduledPosts.ts` auto-merged clean.
- Post-merge `git diff hivetalk/main HEAD` = **only** `.planning/` docs + `vite.config.ts` (exe.dev `allowedHosts`/nostr.json proxy, 7 lines). Byte-exact convergence achieved.
- `npm install`: 0 vulnerabilities.
- **Surprise find**: vitest 5 broke jest-dom matcher *types* — `tsc --noEmit` failed on upstream's own `NoteContent.test.tsx`. Fix: `import '@testing-library/jest-dom/vitest'` in `src/test/setup.ts` (commit `471a9c2`). **Upstreamable** — upstream's `npm test` gate is broken the same way.
- Gates: tsc 0 errors, eslint 0 errors/6 pre-existing warnings, vitest 27/27, build+route-meta+404.

## What buttercup gained (upstream #80–#87)

- **My Activity card + community zap stats un-broken on prod** — upstream rewrote both as pure `nostr.query` over WS (kind 9735), eliminating the `/api/dashboard/login` dependency that nginx 403s. Aligns with the WS-not-REST prerogative.
- **Scheduler retry/reschedule** (#86) + UI on AdminScheduledPage
- AdminExplorer relay stats via WS + master-pubkey gate
- react-router 7, vitest 5, security overrides, `rehype-sanitize`/`sanitizeSchema`, tightened CSP (`unsafe-eval` removed — verified nothing needs it)
- `vercel.json` (inert here — nginx, not Vercel)

## Manual verification pending (browser)

- Zaplytics page + My Activity card populate (previously broken by dashboard deny)
- Scheduled posts page + retry UI
- Routing/nav sanity (react-router 7)
- Media upload regression spot-check
- CSP: console free of eval violations

## Known deltas to upstream (intentional)

- `vite.config.ts` — exe.dev dev-proxy config (deployment-local)
- `.planning/` — local planning docs (not upstream material)
- Runtime dirs stay untracked (`blossom/`, `swarm/`, `nostr-cms/`, ops scripts)

## Follow-ups

- Upstream the `jest-dom/vitest` setup fix (one-liner PR)
- Optional: push `buttercup/converge-main` to fork as the new tracking branch; archive `pr-b-features` lineage
- Retire stale `dist.backup-*` beyond a retention window
