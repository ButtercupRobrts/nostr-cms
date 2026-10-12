# External Integrations

**Analysis Date:** 2026-05-09

## APIs & External Services

**Nostr Protocol:**
- @nostrify/nostrify - Core Nostr implementation
  - NPool - Relay pool management
  - NRelay1 - WebSocket relay connections
  - NSecSigner - NIP-07 signer
  - NSchema - Event validation and parsing
  - BlossomUploader - File upload to Blossom servers

- nostr-tools - NIP utilities
  - `nip19` - bech32 encoding (npub, nprofile, note, nevent)
  - `generateSecretKey`, `getPublicKey` - Key generation

- @nostrify/react - React hooks
  - `useNostr()` - Access Nostr pool
  - `NostrLoginProvider` - Authentication provider

**Lightning Network / Payments:**
- @getalby/sdk - Nostr Wallet Connect (NWC)
  - `LN` class - Wallet connection and payments
  - Protocol: `nostr+walletconnect://` and `nostrwalletconnect://`
  - Methods: `pay_invoice`
  - Implementation: `src/hooks/useNWC.ts`

- light-bolt11-decoder - BOLT-11 invoice parsing
  - Used for zap invoice processing

**Forms (Optional):**
- @formstr/sdk ^0.1.4 - Form submission service
  - Reference in package.json, usage appears minimal/optional

## Data Storage

**Databases:**
- **Nostr Relays** - Primary content storage
  - Connection: WebSocket via `VITE_DEFAULT_RELAY` or auto-derived from domain
  - Implementation: `@nostrify/nostrify` NRelay1/NPool
  - Config: `src/lib/relay.ts`

- **IndexedDB** - Client-side caching
  - Package: `idb` ^8.0.3
  - Uses: DM message cache (`src/lib/dmMessageStore.ts`)

**File Storage:**
- **Blossom Protocol** - Media/file uploads
  - Implementation: `BlossomUploader` from `@nostrify/nostrify/uploaders`
  - Hook: `src/hooks/useUploadFile.ts`
  - Config: `blossomRelays` array in site config
  - Fallback: `https://blossom.primal.net/`

**Caching:**
- **TanStack Query** - Server state caching
  - Package: `@tanstack/react-query`
  - Invalidates on relay changes

## Authentication & Identity

**Nostr Native Auth:**
- NostrLoginProvider from `@nostrify/react/login`
- NIP-07 browser extensions support
- NIP-42 authentication challenges

**Admin Authentication:**
- Remote `nostr.json` verification
  - Config: `VITE_REMOTE_NOSTR_JSON_URL`
  - Implementation: `src/hooks/useRemoteNostrJson.ts`
  - Validates admin pubkey matches NIP-05/nip-05 records

**Master/Owner:**
- Config: `VITE_MASTER_PUBKEY` or server-injected `swarm-config.masterPubkey`
- Used for: Site ownership verification, admin access control

## Monitoring & Observability

**Error Tracking:**
- None detected

**Logs:**
- Console-based logging
- Custom error boundary: `src/components/ErrorBoundary.tsx`

## CI/CD & Deployment

**Hosting:**
- Static site (Vite build to `dist/`)
- Deployable to any static host (Vercel, Netlify, GitHub Pages, etc.)
- Optional: Go/Swarm backend for proxy and server-injected config

**CI Pipeline:**
- None configured (manual deployment)

**Build Scripts:**
- `npm run dev` - Development server
- `npm run build` - Production build
- `npm run test` - Full test suite (type check, lint, unit tests, build)

## Environment Configuration

**Required env vars:**
```bash
VITE_DEFAULT_RELAY=          # WebSocket URL (default: auto from domain)
VITE_MASTER_PUBKEY=          # Admin/owner pubkey (npub1 or hex)
VITE_REMOTE_NOSTR_JSON_URL=  # For admin auth verification
VITE_SWARM_API_URL=          # Scheduled posts API (default: /api)
```

**Optional env vars:**
- Theme customization
- Feature flags

**Secrets location:**
- `.env` file (not committed)
- Server-side injection via `<meta name="swarm-config">` tag

## Webhooks & Callbacks

**Incoming:**
- None (client-side only app)

**Outgoing:**
- Nostr event publishing to relays (writes)
- WebSocket subscriptions (reads)

## Third-Party Services

**Fonts:**
- Google Fonts - Loaded via `@fontsource-variable/inter` or dynamic injection
- Custom font URLs via site config

**Analytics:**
- None detected

**CDN:**
- Blossom servers for media
- Configurable via `blossomRelays` in site config

---

*Integration audit: 2026-05-09*
