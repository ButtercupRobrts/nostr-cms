# Roadmap: Nostr-CMS Upstream Project

## Overview

This roadmap takes the complete set of improvements deployed on buttercup.exe.xyz (source: github/main at commit 60c8059, 255 files, 49,747 insertions) and upstreams them to bitkarrot/nostr-cms via a stack of clean, reviewable PRs. The work proceeds in four phases: source preparation, PR splitting, stack building, and AI-powered review. The deployment remains untouched throughout.

## Phases

- [ ] **Phase 1: Source Preparation** - Verify source of truth, create clean working branch, close broken PRs
- [ ] **Phase 2: PR Splitting** - Design and execute the feature-based split into reviewable PRs
- [ ] **Phase 3: Stack Building** - Build dependency-ordered PR stack on the fork
- [ ] **Phase 4: AI Review & Ship** - Devin AI review of each PR, fix defects, open PRs

## Phase Details

### Phase 1: Source Preparation
**Goal**: Establish a clean, compiling working branch from the verified source of truth and close the broken PRs
**Depends on**: Nothing (first phase)
**Requirements**: PREP-01, PREP-02, PREP-03, PREP-04, SAFE-01, SAFE-02, SAFE-03
**Success Criteria** (what must be TRUE):
  1. github/main:nostr-cms/ at 60c8059 is verified as the source of truth (features match deployed dist)
  2. A clean working branch exists in the meetup-space repo with all features, compiling with 0 tsc errors
  3. The 19 broken PRs on the fork are closed with a comment pointing to the new PRs
  4. buttercup.exe.xyz's deployed dist is untouched and the VM main branch is at 2ed6676
**Plans**: 2 plans

Plans:
- [ ] 01-01: Verify source of truth and create clean working branch
- [ ] 01-02: Close broken PRs and document the supersession

### Phase 2: PR Splitting
**Goal**: Split the 49,747-line diff into 5-10 logical, reviewable PRs that each compile at their stack level
**Depends on**: Phase 1
**Requirements**: SPLIT-01, SPLIT-02, SPLIT-03, SPLIT-04, SPLIT-05, SPLIT-06, SAFE-04
**Success Criteria** (what must be TRUE):
  1. A PR splitting plan is designed with 5-10 PRs, each a coherent feature group
  2. Each PR's file set is identified with no duplicate shared code across PRs
  3. The adminRoles migration lives in exactly one PR
  4. No deployment scripts, nginx configs, or backup scripts are included
  5. Each PR is sized 500-3000 lines for effective Devin AI review
**Plans**: 3 plans

Plans:
- [ ] 02-01: Analyze the diff and design the PR splitting strategy (three options)
- [ ] 02-02: Execute the chosen splitting strategy — create branch per PR with correct file sets
- [ ] 02-03: Verify each branch compiles at its stack level with tsc --noEmit

### Phase 3: Stack Building
**Goal**: Build the dependency-ordered PR stack on the fork and push all branches
**Depends on**: Phase 2
**Requirements**: STACK-01, STACK-02, STACK-03, STACK-04, STACK-05
**Success Criteria** (what must be TRUE):
  1. Stack branches exist on the fork in dependency order (each based on the one below)
  2. The first PR targets upstream main (2ed6676)
  3. Each subsequent PR targets the previous PR's branch
  4. Merge order is documented for bitkarrot maintainers
  5. All stack branches are pushed to the fork
**Plans**: 2 plans

Plans:
- [ ] 03-01: Rebase branches into a stack on the fork (each based on the previous)
- [ ] 03-02: Push all stack branches and create PRs with merge order documentation

### Phase 4: AI Review & Ship
**Goal**: Devin AI reviews each PR for correctness, security, and Nostr protocol compliance; fix defects and finalize
**Depends on**: Phase 3
**Requirements**: REVIEW-01, REVIEW-02, REVIEW-03, REVIEW-04, REVIEW-05
**Success Criteria** (what must be TRUE):
  1. Each PR passes Devin AI review for TypeScript correctness
  2. Each PR passes Devin AI review for security (no secrets, no XSS, no injection)
  3. Each PR passes Devin AI review for Nostr protocol compliance
  4. Each PR has a clear description with what/why/how-to-verify
  5. Genuine defects from the original code are fixed before PR is finalized
**Plans**: 2 plans

Plans:
- [ ] 04-01: Run Devin AI review on each PR in stack order, collect findings
- [ ] 04-02: Fix defects identified by review, update PRs, finalize for bitkarrot submission

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Source Preparation | 0/2 | Not started | - |
| 2. PR Splitting | 0/3 | Not started | - |
| 3. Stack Building | 0/2 | Not started | - |
| 4. AI Review & Ship | 0/2 | Not started | - |
