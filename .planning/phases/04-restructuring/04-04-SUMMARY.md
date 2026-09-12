# 04-04 Summary: nostr-cms fix PRs

**Date:** 2026-09-12
**Status:** Partial — my-stats fix shipped; community-zap still gated on bitkarrot

## Done
- **PR #81** on bitkarrot/nostr-cms (`ButtercupRobrts:pr-fix-my-stats` → `main` @ `dadc676`)
- `MyActivityCard` no longer calls `POST /dashboard/login` + `GET /dashboard/my-stats` (both 404 upstream)
- Client-side equivalents: `nostr.query({authors:[pk]})` for totals/kinds/last-activity/embedded-media (mirrors the server endpoint's imeta + tag + kind-0 + content-URL logic), `GET /list/<pk>` (BUD-02, public) for blossom stats
- `tsc --noEmit` clean; single file, +100/−24
- Verified upstream file was identical to ours before rewriting — no assumption drift

## Still gated
- Community zap stats disposition — waiting on bitkarrot's DM answer (delete / client-side aggregate / fixed server PR)
