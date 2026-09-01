# Nostr-CMS Upstream Project

## What This Is

An effort to upstream custom improvements made to nostr-cms on the buttercup.exe.xyz deployment back to the upstream bitkarrot/nostr-cms repository via clean, reviewable pull requests. The buttercup.exe.xyz deployment is a one-click deployment from bitkarrot/nostr-cms that has been enhanced with additional features. This project designs and executes the PR strategy to contribute those improvements upstream without disrupting the running deployment.

## Core Value

Buttercup.exe.xyz remains fully functional and unchanged while its improvements are contributed to bitkarrot/nostr-cms as reviewable, secure, and incrementally mergeable PRs.

## Requirements

### Validated

- ✓ nostr-cms base deployment operational on buttercup.exe.xyz — existing (one-click deploy from bitkarrot/nostr-cms)
- ✓ Custom improvements deployed and running on buttercup.exe.xyz — existing (built Aug 7, 2026)
- ✓ Fork repository (ButtercupRobrts/nostr-cms) created and linked to upstream — existing

### Active

- [ ] Identify the complete set of improvements deployed on buttercup.exe.xyz vs upstream
- [ ] Design a PR splitting strategy that produces reviewable, independently compilable PRs
- [ ] Recover any lost code from the pre-split working branch (class A issues)
- [ ] Resolve the adminRoles migration ownership (#48 vs #37)
- [ ] Fix genuine defects (class D: hideCloseButton, centerMedia/truncateEmbeds, autoHarvest24h, FALLBACK_DISCOVERY_RELAYS)
- [ ] Build dependency-ordered PR stack on the fork
- [ ] Each PR in the stack passes `npx tsc --noEmit` at its stack level
- [ ] Each PR is reviewed by Devin AI for correctness, security, and Nostr protocol compliance
- [ ] PRs are opened against bitkarrot/nostr-cms with clear merge order documentation

### Out of Scope

- Changing buttercup.exe.xyz's running deployment — the deployed dist (Aug 7) is immutable; no rebuilds from non-main branches
- Modifying the upstream bitkarrot/nostr-cms repo directly — external contributor, PRs only
- Adding new features beyond what's already deployed on buttercup.exe.xyz — this is upstreaming existing work, not new development
- Fixing the 19 existing broken PRs in place — they will be superseded by a clean re-split

## Context

### Deployment Architecture
- buttercup.exe.xyz is an exe.dev VM running nostr-cms
- The deployed site is static files in `nostr-cms/dist/` (built Aug 7, 2026)
- The exe.dev HTTPS proxy serves these static files directly
- The source repo is at `/home/exedev/meetup-space/` with git remotes:
  - `origin`: `nostrcmsbtcp.int.exe.xyz/ButtercupRobrts/NostrCMSbtcp.git` (exe.dev internal)
  - `fork`: `github.com/ButtercupRobrts/nostr-cms.git` (GitHub fork)
  - `upstream`: `github.com/bitkarrot/nostr-cms.git` (upstream)
- The vite build outputs to `dist/` (not `nostr-cms/dist/`), so rebuilds cannot overwrite the deployed files

### Current Git State
- `main` branch is at upstream base commit 2ed6676 (PR #31 merge) — safe, matches deployment base
- **Source of truth found**: `github/main` (ButtercupRobrts/NostrCMSbtcp) at commit 60c8059 (Aug 2, 2026)
  - 191 commits, 366 files changed, 34,510 insertions since upstream base
  - nostr-cms app code is in `nostr-cms/` subdirectory (alongside deployment scripts, nginx config, etc.)
  - 255 files changed in nostr-cms/src/ alone, 49,747 insertions
  - **Compiles cleanly**: `npx tsc --noEmit` passes with 0 errors
  - Has complete AdminEvents.tsx with both `repostOpen`/`RepostDialog` AND `shareNoteOpen`/`ShareAsNoteDialog`
  - Contains ALL features deployed on buttercup.exe.xyz (verified against Aug 7 dist bundle)
- 5 feature commits exist on `stack/05-media-improvements` (Sep 1, subset of github/main)
- 19 PR branches exist on the fork (#32-#50), all broken (15 fail tsc, 4 have lost code)
- 19 local `stack/01` through `stack/19` branches from restacking experiment (unpushed)
- The 19 PRs were a poor split of github/main — they lost code and created artificial dependencies

### Restacking Report Findings
A detailed review (see `/tmp/nostr-cms-handoff-prompt.md`) identified 4 classes of issues:
- **Class A (lost code)**: AdminEvents.tsx and AdminBlog.tsx have empty JSX conditionals in all PR branches. Not recoverable from the PR set.
- **Class B (cross-PR deps)**: PRs reference symbols from other PRs. Fixed by proper stacking.
- **Class C (adminRoles migration)**: 'primary'|'secondary' → 'publisher'|'user' rename. Needs ownership decision.
- **Class D (real defects)**: hideCloseButton, centerMedia/truncateEmbeds, autoHarvest24h, FALLBACK_DISCOVERY_RELAYS.

### Constraints
- External contributor to bitkarrot/nostr-cms (PRs only, no push access)
- GitHub requires PR base branches to exist in the base repo
- Stack-on-fork is the viable approach: build stacks inside the fork, each PR targets the previous PR's branch
- Devin AI is the review tool — PRs must be sized for effective AI review
- The deployed buttercup.exe.xyz must not be affected by any of this work

## Constraints

- **Access**: External contributor to bitkarrot/nostr-cms — can only open PRs from fork, cannot push branches to upstream
- **Deployment safety**: buttercup.exe.xyz's deployed dist (Aug 7, 2026) is immutable — no rebuilds from non-main branches, no changes to nostr-cms/dist/
- **Git structure**: GitHub requires PR base branches to exist in the base repo — stack-on-fork is the only viable stacking approach without upstream push access
- **Review tool**: Devin AI reviews each PR — PRs must be sized to fit effective AI review context windows
- **Type safety**: Each PR must pass `npx tsc --noEmit` at its stack level before being opened
- **Nostr protocol**: All Nostr-related code must be NIP-compliant (NIP-18, NIP-19, NIP-52, NIP-53 as applicable)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Source of truth is buttercup.exe.xyz deployed code | The deployment is the working, validated state; everything else is scratch material | — Pending |
| Stack-on-fork PR strategy | Only viable approach for dependency-ordered PRs without upstream push access; enables incremental review | — Pending |
| Supersede the 19 broken PRs with a clean re-split | The existing PRs have lost code, broken JSX, and over-segregation; fixing them in place is more work than starting fresh | — Pending |
| buttercup.exe.xyz main branch stays at upstream base | Keeps the VM's git state safe; deployed dist is independent of git state | ✓ Good |
| GSD Core for project structure | Provides spec-driven, artifact-persistent workflow for planning and executing the upstreaming | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-09-01 after initialization*
