# Roadmap: Nostr-CMS Upstream Project

## Overview

This roadmap takes the complete set of improvements deployed on buttercup.exe.xyz (source: github/main at commit 60c8059, 255 files, 49,747 insertions) and upstreams them to bitkarrot/nostr-cms via a stack of clean, reviewable PRs. The work proceeds in four phases: source preparation, PR splitting, stack building, and AI-powered review. The deployment remains untouched throughout.

## Phases

- [ ] **Phase 1: Source Preparation** - Verify source of truth, create clean working branch, close broken PRs
- [ ] **Phase 2: PR Splitting** - Design and execute the feature-based split into reviewable PRs
- [ ] **Phase 3: Stack Building** - Build dependency-ordered PR stack on the fork
- [ ] **Phase 4: Upstream Restructuring & Security Fixes** - Security stopgap, supersede PR #16, bugfix/NIP-98/NIP-86 PRs to zeabur-dashboard, nostr-cms 404 fixes, redeploy

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

### Phase 3: Fresh PRs
**Goal**: Create 3 clean, reviewable PRs (A=core/bugs, B=features, C=backend) and submit to upstream
**Depends on**: Phase 2
**Requirements**: STACK-01, STACK-02, STACK-03, STACK-04, STACK-05
**Success Criteria** (what must be TRUE):
  1. Source files sanitized (no buttercup.exe.xyz, no secrets, no personal data)
  2. PR A (core/bugs) compiles independently with tsc --noEmit = 0 errors
  3. PR B (features) compiles stacked on PR A with tsc --noEmit = 0 errors
  4. PR C (backend) compiles with go build = 0 errors
  5. All 3 PRs pushed and created on GitHub with clear descriptions
**Plans**: 1 plan

Plans:
- [x] 03-01: Sanitize, split into 3 PRs, verify compilation, create PRs on GitHub

### Phase 4: Upstream Restructuring & Security Fixes
**Goal**: Close the live admin-API hole on buttercup, retire superseded PR #16, and ship focused follow-up PRs per the restructuring report (`.planning/PR-RESTRUCTURING-REPORT.md`)
**Depends on**: Phase 3 (merged #78/#79; #16 superseded)
**Requirements**: SEC-01, FIX-01, FIX-02, FIX-03, AUTH-01, CMS-01, CMS-02, NIP86-01, DEPLOY-01
**Success Criteria** (what must be TRUE):
  1. `/api/dashboard/` + `/api/admin/` writes are not exploitable on buttercup (per chosen stopgap)
  2. PR #16 closed on HiveTalk/swarm with supersession note
  3. bitkarrot has been asked: community-zap disposition, NIP-98 breaking-change sign-off, NIP-86 appetite
  4. Trivial-fixes PR open against `zeabur-dashboard` (bech32→nip19, `go s.save()` sync, env secrets removed)
  5. NIP-98 dashboard-login PR open or explicitly deferred by bitkarrot
  6. `MyActivityCard` 404 fixed via client-side `nostr.query()` PR against upstream/main
  7. Community zap stats disposition decided by bitkarrot and acted on
  8. Redeploy delta (lost features: zap-stats/video/backfill/community tab) called out before Phase 4 redeploy
**Plans**: 7 plans

Plans:
- [ ] 04-01: Security stopgap + upstream coordination (nginx deny [decision gate], close PR #16, send bitkarrot questions)
- [ ] 04-02: Swarm trivial-fixes PR against zeabur-dashboard
- [ ] 04-03: Swarm NIP-98 dashboard-auth PR (blocked on bitkarrot ack)
- [ ] 04-04: nostr-cms fix PRs (my-stats rewrite; community-zap decision gate)
- [ ] 04-05: Optional PRs — NIP-86 server+client, video transcoding
- [ ] 04-06: Redeploy buttercup from merged upstream (delta-aware)
- [ ] 04-07: Remove nsec login from NostrLogin.js (NIP-07-only) + vendor nostr-tools bundle

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Source Preparation | 0/2 | Not started | - |
| 2. PR Splitting | 0/3 | Not started | - |
| 3. Fresh PRs | 1/1 | Complete | 2026-09-02 |
| 4. Upstream Restructuring & Security Fixes | 3/7 | In progress | - |
