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

## Update 2026-09-12 (bitkarrot directive — Strategy A: client-first)

Her reply: don't change swarm; fix on nostr-cms main using WebSockets not REST; relay changes discussed later and must be individually justified.

- Decision gate RESOLVED → client-side aggregation (her words: "use websockets and not rest api")
- **PR #82** open on bitkarrot/nostr-cms (`pr-community-zap-ws` → `main`): `useCommunityZapStats` rewritten — single `{kinds:[9735], '#p':[members]}` WS query + existing zaplytics/utils pipeline; verified live (2,197 receipts). Same return shape; AdminZaplytics untouched. +194/−245, one file.
- Caveat noted in PR: counts only receipts on this relay — no external fan-out (that was the rejected part anyway).
- Swarm PRs #18/#19 stay open but dormant; NIP-98 branches stay local — all become the "justify later" list (Tier 1: session-auth fix; Tier 2: #18 bugfixes; Tier 3: #19 hardening).
