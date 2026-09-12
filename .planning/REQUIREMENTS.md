# Requirements: Nostr-CMS Upstream Project

**Defined:** 2026-09-01
**Core Value:** Buttercup.exe.xyz remains fully functional and unchanged while its improvements are contributed to bitkarrot/nostr-cms as reviewable, secure, and incrementally mergeable PRs.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Source Preparation

- [ ] **PREP-01**: Identify and verify the complete source of truth (github/main at 60c8059) against the deployed buttercup.exe.xyz dist
- [ ] **PREP-02**: Create a clean working branch from github/main:nostr-cms/ mapped to the meetup-space repo structure
- [ ] **PREP-03**: Verify the working branch compiles with `npx tsc --noEmit` (0 errors)
- [ ] **PREP-04**: Close or supersede the 19 broken PRs on the fork with clear documentation

### PR Splitting

- [ ] **SPLIT-01**: Design a PR splitting strategy that produces 5-10 logical, reviewable PRs
- [ ] **SPLIT-02**: Each PR must be a coherent feature or feature group, not an arbitrary file slice
- [ ] **SPLIT-03**: Each PR must pass `npx tsc --noEmit` at its stack level
- [ ] **SPLIT-04**: Shared code (kinds.ts, repost.ts, blossom.ts, etc.) must live in exactly one PR, not duplicated
- [ ] **SPLIT-05**: The adminRoles migration ('primary'|'secondary' → 'publisher'|'user') must be in exactly one PR
- [ ] **SPLIT-06**: PRs must be sized for effective Devin AI review (target: 500-3000 lines per PR)

### PR Stacking

- [ ] **STACK-01**: Build dependency-ordered PR stack on the fork (ButtercupRobrts/nostr-cms)
- [ ] **STACK-02**: Each PR targets the previous PR's branch in the fork, not main
- [ ] **STACK-03**: The first PR in the stack targets upstream main (2ed6676)
- [ ] **STACK-04**: Document the merge order clearly for bitkarrot maintainers
- [ ] **STACK-05**: Push all stack branches to the fork

### PR Review

- [ ] **REVIEW-01**: Each PR is reviewed by Devin AI for TypeScript correctness
- [ ] **REVIEW-02**: Each PR is reviewed by Devin AI for security (no exposed secrets, no XSS, no injection)
- [ ] **REVIEW-03**: Each PR is reviewed by Devin AI for Nostr protocol compliance (NIP-18, NIP-19, NIP-52, NIP-53)
- [ ] **REVIEW-04**: Each PR has a clear description with what changed, why, and how to verify
- [ ] **REVIEW-05**: Genuine defects from the original code are fixed before PR (not upstreamed as-is)

### Deployment Safety

- [ ] **SAFE-01**: buttercup.exe.xyz deployed dist (Aug 7, 2026) remains untouched
- [ ] **SAFE-02**: No rebuilds from non-main branches on the VM
- [ ] **SAFE-03**: The VM's main branch stays at upstream base (2ed6676)
- [ ] **SAFE-04**: No deployment scripts, nginx configs, or backup scripts are included in PRs (app code only)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Additional Features

- **V2-01**: Admin content sync page (if not included in v1)
- **V2-02**: Follow list backup and recovery (if not included in v1)
- **V2-03**: Settings page modularization (if not included in v1)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Changing buttercup.exe.xyz's running deployment | The deployed dist is immutable; no rebuilds from non-main branches |
| Modifying bitkarrot/nostr-cms directly | External contributor, PRs only |
| Adding new features beyond what's deployed | This is upstreaming existing work, not new development |
| Including deployment scripts/configs in PRs | Upstream only wants the app code, not deployment infrastructure |
| Fixing the 19 broken PRs in place | They will be superseded by a clean re-split from github/main |
| Upgrading dependencies | Separate concern from feature upstreaming |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PREP-01 | Phase 1 | Pending |
| PREP-02 | Phase 1 | Pending |
| PREP-03 | Phase 1 | Pending |
| PREP-04 | Phase 1 | Pending |
| SPLIT-01 | Phase 2 | Pending |
| SPLIT-02 | Phase 2 | Pending |
| SPLIT-03 | Phase 2 | Pending |
| SPLIT-04 | Phase 2 | Pending |
| SPLIT-05 | Phase 2 | Pending |
| SPLIT-06 | Phase 2 | Pending |
| STACK-01 | Phase 3 | Pending |
| STACK-02 | Phase 3 | Pending |
| STACK-03 | Phase 3 | Pending |
| STACK-04 | Phase 3 | Pending |
| STACK-05 | Phase 3 | Pending |
| REVIEW-01 | Phase 4 | Pending |
| REVIEW-02 | Phase 4 | Pending |
| REVIEW-03 | Phase 4 | Pending |
| REVIEW-04 | Phase 4 | Pending |
| REVIEW-05 | Phase 4 | Pending |
| SAFE-01 | Phase 1 | Pending |
| SAFE-02 | Phase 1 | Pending |
| SAFE-03 | Phase 1 | Pending |
| SAFE-04 | Phase 2 | Pending |

**Coverage:**
- v1 requirements: 24 total
- Mapped to phases: 24
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-01*
*Last updated: 2026-09-01 after initial definition*
