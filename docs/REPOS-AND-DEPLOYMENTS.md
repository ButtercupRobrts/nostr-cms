# Repo & Deployment Map — how the pieces relate

*Updated 2026-10-05 · kept current alongside the repo map*

## The three repos — roles in one line

| Repo | Role | Visibility | Owner |
|---|---|---|---|
| `HiveTalk/nostr-cms` | **Upstream** — the canonical codebase maintained by the HiveTalk community | public | HiveTalk |
| `ButtercupRobrts/nostr-cms` | **Our fork** — the PR interface + our deployment branch | public | us |
| `ButtercupRobrts/NostrCMSbtcp` | **Ops monorepo** — private deployment record + runbook + ops scripts | private | us |

## How code flows

```
HiveTalk/nostr-cms (upstream, main)
        │  upstream fixes + features
        ▼
ButtercupRobrts/nostr-cms
  ├─ main            = stock upstream, kept synced
  └─ converge-main   = upstream + our deliberate deployment deltas
        │  code deploys from here
        ▼
buttercup.exe.xyz   = production site (exe.dev edge → nginx :8000)
```

Our changes go the other way: **fork → upstream PRs → merged → pulled back down** (already happened four times: #105, #116, #117, #122).

## What the ops monorepo holds

`NostrCMSbtcp` — private, unrelated history to the CMS repo:

```
root/            ops scripts: backup-*.sh, harvest, sync-relay, nginx-meetup.conf,
                 AGENTS.md (runbook), docs/, .devin-plans
  nostr-cms/     the CMS source copy + dist/ that nginx serves
  swarm/         the relay source (Go)
  blossom/       media blobs (gitignored)
```

It records **what's deployed and how to run it** — not the code authority.
Sync rule: `nostr-cms/` tracks `converge-main`; refreshed by git-archive
extraction (tracked files only, no secrets).

## The two live sites

| Site | What it is | Source |
|---|---|---|
| `buttercup.exe.xyz` | Production — the actual community site | `converge-main` build → `nostr-cms/dist` → nginx :8000 → exe.dev edge :443 |
| `temporary-agile-gold-*.vercel.app` | Vercel preview/staging for upstream | fork `main` auto-deploy |

## Security boundaries that matter

- **edge → nginx :8000**: static + API proxying; `/api/admin|dashboard` → perl owner-gate (non-owner signed → 403)
- **`/mirror`**: localhost-only (403 publicly)
- **`:3334` swarm relay**: iptables DROP non-loopback (localhost tools only)
- **`nostr.json` `_` entry** = the owner identity the gate checks against
- **nsec login**: compile-time off in production (`VITE_ENABLE_NSEC_LOGIN=false`)

## Current pinned migrations (dependabot ignores + tracked issues)

| Issue | Dep | Trigger to pick up |
|---|---|---|
| #118 | zod 4 | forms migration — resolver typing |
| #119 | recharts 3 | shadcn chart.tsx port |
| #120 | getalby 8 | runtime wallet test needed |
| #121 | nostrify 0.6 | biggest; do when nostr-layer work needed anyway |

## Branch hygiene (converge-main's intentional deltas only)

`vite.config.ts`, `.env.production`, `useUploadFile` hard-error variant,
eslint ignore for `nostr-cms/` subdir — everything else == upstream.
