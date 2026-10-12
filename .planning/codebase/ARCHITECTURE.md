<!-- refreshed: 2025-05-09 -->
# Architecture

**Analysis Date:** 2026-05-09

## System Overview

```text
┌─────────────────────────────────────────────────────────────────────┐
│                           Browser (SPA)                              │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                        React Router                           │  │
│  │  ┌───────────────┬───────────────┬─────────────────────────┐ │  │
│  │  │ Public Routes │ Admin Routes  │ NIP-19 / Dynamic Routes │ │  │
│  │  │ `/events`     │ `/admin/*`    │ `/npub1...` `/p/:path`   │ │  │
│  │  └───────┬───────┴───────┬───────┴───────────┬─────────────┘ │  │
│  │          │               │                   │               │  │
│  │  ┌───────▼────────────────▼───────────────────▼───────┐     │  │
│  │  │              Provider Layer (App.tsx)               │     │  │
│  │  │  ┌───────────────────────────────────────────────┐ │     │  │
│  │  │  │ UnheadProvider → QueryClient → NostrLogin     │ │     │  │
│  │  │  │ → NostrProvider → NWCProvider → AdminAuth    │ │     │  │
│  │  │  └───────────────────────────────────────────────┘ │     │  │
│  │  └───────────────────────────────────────────────────────┘     │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                       Contexts                               │  │
│  │  ┌─────────────┬───────────────┬─────────────┬────────────┐ │  │
│  │  │ AppContext  │ AdminAuthCtx  │ NWCContext  │ DMContext │ │  │
│  │  │ (config)    │ (auth check)  │ (wallet)    │ (chat)    │ │  │
│  │  └─────────────┴───────────────┴─────────────┴────────────┘ │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                    Component Layers                          │  │
│  │  ┌────────────────┬────────────────┬──────────────────────┐ │  │
│  │  │ Page Components│ Admin Components│ Feature Components  │ │  │
│  │  │ Index, Events  │ Dashboard, etc │ Zaplytics, DM, Chat  │ │  │
│  │  └────────┬───────┴────────┬───────┴──────────┬───────────┘ │  │
│  │           │                │                   │             │  │
│  │  ┌────────▼────────────────▼───────────────────▼───────────┐ │  │
│  │  │              UI Components (shadcn/ui)                  │ │  │
│  │  └─────────────────────────────────────────────────────────┘ │  │
│  └─────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
         │                                               │
         ▼                                               ▼
┌───────────────────┐                       ┌──────────────────────┐
│  Nostr Relays     │                       │ External Services    │
│  (Content Store)  │                       │ - Blossom (media)    │
│                   │                       │ - Alby (NWC)         │
│  - Default relay  │                       │ - Formstr (forms)    │
│  - NIP-65 relays  │                       └──────────────────────┘
└───────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| `AppRouter` | Route definition and nested route structure | `src/AppRouter.tsx` |
| `App` | Provider orchestration (wraps all contexts) | `src/App.tsx` |
| `AppProvider` | App config state, localStorage persistence, SEO, theme | `src/components/AppProvider.tsx` |
| `NostrProvider` | Nostr pool initialization, relay routing strategy | `src/components/NostrProvider.tsx` |
| `NostrSync` | Sync NIP-65 relay lists and site config from Nostr | `src/components/NostrSync.tsx` |
| `AdminWrapper` | Admin route protection via `useAdminAuth` check | `src/pages/admin/AdminWrapper.tsx` |
| `AdminLayout` | Admin sidebar, navigation, query prefetching | `src/components/admin/AdminLayout.tsx` |
| `DMProvider` | Direct messaging state (NIP-04/NIP-17), IndexedDB cache | `src/components/DMProvider.tsx` |
| `ZaplyticsDashboard` | Zap analytics aggregation and visualization | `src/components/zaplytics/ZaplyticsDashboard.tsx` |

## Pattern Overview

**Overall:** Provider-per-Feature + Context-Based State Management

**Key Characteristics:**
- **Provider-per-Feature pattern:** Each major feature (DM, NWC, Admin Auth, App Config) has its own provider/context
- **Nostr-as-Database:** All CMS content lives on Nostr relays; local state is derived/cached
- **Dual Query Strategy:** CMS content queries default relay only; social features query NIP-65 relays via fanout
- **React Router v6:** Route-based code splitting with nested admin routes
- **TanStack Query:** Centralized caching, deduplication, and background refetching
- **shadcn/ui:** Headless UI primitives with Tailwind styling

## Layers

**Provider Layer:**
- Purpose: Global state, external service integrations, authentication
- Location: `src/App.tsx` (orchestration), `src/components/*Provider.tsx`, `src/contexts/`
- Contains: Context providers, Nostr pool setup, query client configuration
- Depends on: External Nostr relays, localStorage, IndexedDB
- Used by: All pages and components via context hooks

**Routing Layer:**
- Purpose: Navigation and access control
- Location: `src/AppRouter.tsx`
- Contains: Public routes, admin nested routes, dynamic/catch-all routes
- Depends on: React Router, `AdminWrapper` for auth checks
- Used by: Browser for URL navigation

**Page Layer:**
- Purpose: Route-bound UI containers
- Location: `src/pages/`, `src/pages/admin/`
- Contains: Page-level components that compose feature components
- Depends on: Hooks for data fetching, context providers, UI components
- Used by: React Router for route rendering

**Feature Component Layer:**
- Purpose: Reusable feature-specific UI (DM, zaplytics, auth)
- Location: `src/components/dm/`, `src/components/zaplytics/`, `src/components/auth/`, `src/components/admin/`
- Contains: Complex multi-part UI with internal state
- Depends on: Context hooks, custom hooks, UI components
- Used by: Page components and admin routes

**UI Component Layer:**
- Purpose: Headless/styled primitives
- Location: `src/components/ui/`
- Contains: Radix UI primitives wrapped with Tailwind classes
- Depends on: Radix UI, Tailwind CSS
- Used by: All layers for UI rendering

**Hooks Layer:**
- Purpose: Encapsulated business logic and data fetching
- Location: `src/hooks/`
- Contains: Nostr queries, mutations, local storage, derived state
- Depends on: Context providers, TanStack Query, Nostr libraries
- Used by: Components and pages

**Utility Layer:**
- Purpose: Pure functions and helpers
- Location: `src/lib/`
- Contains: Nostr utilities, relay management, DM helpers, CSV export
- Depends on: Nostr libraries, date-fns, other utils
- Used by: Hooks and components

## Data Flow

### Primary Request Path (Page Load)

1. **Browser Request** (`main.tsx:10-14`) - Entry point renders `App` inside `ErrorBoundary`
2. **Provider Initialization** (`App.tsx:80-104`) - All providers mount in order:
   - `AppProvider` loads config from localStorage, applies theme/SEO
   - `QueryClientProvider` initializes TanStack Query with 30s stale time
   - `NostrLoginProvider` handles user authentication
   - `NostrProvider` creates NPool with default relay routing
   - `NostrSync` fetches NIP-65 relays and site config from master user
   - `NWCProvider` initializes wallet connection state
   - `AdminAuthProvider` checks if logged-in user is an admin
3. **Route Match** (`AppRouter.tsx:42-96`) - React Router matches URL to page component
4. **Page Render** - Page component uses hooks to fetch data, renders UI components
5. **UI Composition** - Feature components and UI primitives render final output

### Admin Access Flow

1. **User Navigates to `/admin/*`** (`AppRouter.tsx:69-86`)
2. **AdminWrapper Check** (`AdminWrapper.tsx:5-17`) - Calls `useAdminAuth()` hook
3. **Auth Verification** (`useRemoteNostrJson.ts`) - Queries `nostr.json` to check admin role
4. **Redirect or Render** - Non-admins redirected to `/admin/login`; admins see `AdminLayout`

### Content Publishing Flow

1. **User Action** (e.g., create blog post in admin)
2. **useNostrPublish Hook** (`useNostrPublish.ts:8-61`) - Mutation signs event with user's signer
3. **Relay Broadcast** - Event published to all write relays (NIP-65 + default)
4. **Query Invalidation** - TanStack Query invalidates relevant queries, triggering refetch
5. **UI Update** - Components re-render with new data

### Social Feature Data Flow (Feed, Zaps, Comments, DMs)

1. **Component Mount** (e.g., `FeedPage.tsx`)
2. **NIP-65 Fanout** (`queryRelays.ts:14-42`) - `queryWithNip65Fanout()` queries:
   - Default relay pool (configured default relay)
   - User's NIP-65 read relays (if configured)
   - Optionally, publish relays (if `feedReadFromPublishRelays` enabled)
3. **Deduplication** - Events merged and deduplicated by ID
4. **State Update** - Results cached in TanStack Query with `queryKey: ['feed-notes', ...]`

**State Management:**
- **Global config:** `AppContext` with localStorage persistence via `useLocalStorage` hook
- **Nostr state:** `@nostrify/react`'s `NostrContext` provides pool instance
- **Admin auth:** Derived from `nostr.json` query result, cached in TanStack Query
- **DM state:** Complex state in `DMContext` with IndexedDB persistence (`dmMessageStore.ts`)
- **Zap analytics:** Progressive loading with module-level cache (`useZapAnalytics.ts`)

## Key Abstractions

**Nostr Pool (`NPool`):**
- Purpose: Relay multiplexing with custom routing logic
- Examples: `src/components/NostrProvider.tsx:32-70`
- Pattern: Singleton pool created once, routed via `reqRouter` (reads to default only) and `eventRouter` (writes to NIP-65 + default)

**Query Keys:**
- Purpose: TanStack Query cache identification and invalidation
- Examples: `['events']`, `['blog-posts']`, `['feed-notes', pubkeys, ...]`, `['zaps', eventId]`
- Pattern: Hierarchical keys starting with feature name, including dependencies for cache invalidation

**NIP-65 Fanout:**
- Purpose: Query social data across multiple relays
- Examples: `src/lib/queryRelays.ts:14-42`
- Pattern: Query default pool + user's NIP-65 relays in parallel, merge and deduplicate by event ID

**Provider Hook Pattern:**
- Purpose: Type-safe context access with error handling
- Examples: `useAppContext()`, `useAdminAuth()`, `useNWC()`
- Pattern: Context value exported separately; hook throws if used outside provider

## Entry Points

**`main.tsx`:**
- Location: `src/main.tsx`
- Triggers: Application bootstrap
- Responsibilities: Polyfill loading, React root creation, ErrorBoundary wrapper

**`App.tsx`:**
- Location: `src/App.tsx`
- Triggers: Called by `main.tsx`
- Responsibilities: Provider orchestration, default config definition

**`AppRouter.tsx`:**
- Location: `src/AppRouter.tsx`
- Triggers: Rendered by `App` component
- Responsibilities: Route definitions, nested admin routes, catch-all handling

**`vite.config.ts`:**
- Location: `vite.config.ts`
- Triggers: Vite dev server / build
- Responsibilities: Path alias (`@/*`), proxy configuration, test setup

## Architectural Constraints

- **Threading:** Single-threaded React event loop. No worker threads used. Heavy operations (like DM decryption) use batching and debouncing to avoid blocking.
- **Global state:**
  - `userZapCache` and `profileCache` in `useZapAnalytics.ts:48-62` (module-level Map caches)
  - DM module-level constants in `dmConstants.ts`
  - QueryClient singleton defined in `App.tsx:26-35`
- **Circular imports:** None detected. Contexts and hooks follow import direction: `pages → components → hooks → contexts/lib`
- **Nostr event limits:** Relays may return partial results; progressive loading used for zap analytics with adaptive batch sizing
- **Authentication state:** Managed by `@nostrify/react/login`'s `NostrLoginProvider`, persisted to localStorage with key `nostr:login`

## Anti-Patterns

### Direct Nostr Query Usage in Components

**What happens:** Components call `nostr.query()` directly instead of using hooks
**Why it's wrong:** Bypasses TanStack Query caching, causes duplicate requests, misses loading/error states
**Do this instead:** Use custom hooks like `useDefaultRelay()`, `useAuthor()`, or create new hooks in `src/hooks/`

### Missing Admin Role Checks

**What happens:** Admin components render without verifying user's role in `adminRoles`
**Why it's wrong:** Non-primary admins could access restricted features
**Do this instead:** Check `config.siteConfig.adminRoles[userPubkey]` and use `readOnlyAdminAccess` for non-master admins

### Bypassing NIP-65 Fanout for Social Data

**What happens:** Social features (feed, zaps, DMs) query only default relay
**Why it's wrong:** Social data lives on user's relays, not CMS relay; missing data
**Do this instead:** Use `queryWithNip65Fanout()` from `src/lib/queryRelays.ts` with `getNip65ReadRelays(config.relayMetadata)`

## Error Handling

**Strategy:** TanStack Query error boundaries + Toast notifications

**Patterns:**
- **Query errors:** Caught by TanStack Query, exposed as `error` in query result
- **Mutation errors:** Logged in `useMutation.onError` callbacks, displayed via `useToast()`
- **Render errors:** Caught by top-level `ErrorBoundary` component (`src/components/ErrorBoundary.tsx`)
- **Nostr errors:** Timeout after 5-10s via `AbortSignal.timeout()`, results in empty arrays or query errors

## Cross-Cutting Concerns

**Logging:** Console-based with prefixes (`[NostrSync]`, `[DM]`, `[FeedPage]`)
**Validation:** Zod schemas in `AppProvider.tsx:17-67` for config validation
**Authentication:** `@nostrify/react/login` with `NostrLoginProvider`, pubkey-based admin checks via `nostr.json`
**Theming:** CSS classes on `<html>` element (`light`/`dark`), system theme detection via `matchMedia`
**SEO:** `@unhead/react` with `useSeoMeta()` for dynamic meta tags based on route and content
**Persistence:** localStorage for config/auth, IndexedDB for DM messages (`dmMessageStore.ts`)

---

*Architecture analysis: 2026-05-09*
