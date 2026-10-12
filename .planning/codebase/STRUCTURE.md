# Codebase Structure

**Analysis Date:** 2026-05-09

## Directory Layout

```
[project-root]/
├── src/
│   ├── components/          # React components organized by feature
│   │   ├── admin/           # Admin dashboard components
│   │   ├── auth/            # Login, signup, account switching
│   │   ├── comments/        # Comment thread UI (NIP-22 replies)
│   │   ├── dm/              # Direct messaging UI (NIP-04/NIP-17)
│   │   ├── ui/              # shadcn/ui primitives (Radix + Tailwind)
│   │   ├── zaplytics/       # Zap analytics visualization
│   │   └── *.tsx            # Top-level feature components
│   ├── contexts/            # React context definitions
│   ├── hooks/               # Custom React hooks
│   ├── lib/                 # Utility functions and helpers
│   │   └── zaplytics/       # Zap analytics processing
│   ├── pages/               # Route components
│   │   └── admin/           # Admin route components
│   ├── test/                # Test setup files
│   ├── types/               # TypeScript type definitions
│   ├── App.tsx              # Provider orchestration
│   ├── AppRouter.tsx        # Route definitions
│   ├── main.tsx             # Application entry point
│   └── vite-env.d.ts        # Vite environment types
├── public/                  # Static assets
├── scripts/                 # Build/utility scripts
├── docs/                    # Documentation
├── eslint-rules/            # Custom ESLint rules
├── .agent/skills/           # GSD agent skills
├── .planning/               # Planning documents (output)
├── index.html               # HTML template
├── package.json             # Dependencies and scripts
├── vite.config.ts           # Vite configuration
├── tsconfig.json            # TypeScript configuration
├── tailwind.config.ts       # Tailwind CSS configuration
└── components.json          # shadcn/ui configuration
```

## Directory Purposes

**`src/components/`:**
- Purpose: All React components except pages
- Contains: Feature components, UI components, admin components, auth UI
- Key files: `Navigation.tsx`, `FeedItem.tsx`, `NostrProvider.tsx`, `ZapDialog.tsx`

**`src/components/admin/`:**
- Purpose: Admin dashboard feature modules
- Contains: Per-admin-section components (Dashboard, Blog, Events, etc.)
- Key files: `AdminLayout.tsx`, `AdminDashboard.tsx`, `AdminForms.tsx`, `AdminSettings.tsx`

**`src/components/ui/`:**
- Purpose: shadcn/ui component library (auto-generated, manual edits discouraged)
- Contains: Radix UI primitives wrapped with Tailwind classes
- Key files: `button.tsx`, `dialog.tsx`, `form.tsx`, `sidebar.tsx`, `toast.tsx`

**`src/components/auth/`:**
- Purpose: Authentication UI (login, signup, account management)
- Contains: `LoginDialog.tsx`, `SignupDialog.tsx`, `AccountSwitcher.tsx`, `LoginArea.tsx`

**`src/components/dm/`:**
- Purpose: Direct messaging interface for encrypted DMs
- Contains: `DMChatArea.tsx`, `DMConversationList.tsx`, `DMStatusInfo.tsx`

**`src/components/comments/`:**
- Purpose: Nested comment threads using NIP-22 reply hierarchy
- Contains: `Comment.tsx`, `CommentForm.tsx`, `CommentsSection.tsx`

**`src/components/zaplytics/`:**
- Purpose: Zap analytics visualization components
- Contains: Charts, tables, dashboards for zap earnings analysis

**`src/contexts/`:**
- Purpose: React context definitions and exports
- Contains: `AppContext.ts` (config), `DMContext.ts` (messages), `AdminAuthContext.tsx`, `NWCContext.tsx`

**`src/hooks/`:**
- Purpose: Custom React hooks for data fetching and state management
- Contains: Nostr queries, mutations, local storage hooks, feature-specific hooks
- Key files: `useNostrPublish.ts`, `useZaps.ts`, `useComments.ts`, `useZapAnalytics.ts`, `useAppContext.ts`

**`src/lib/`:**
- Purpose: Pure utility functions and helpers
- Contains: Relay management, Nostr utilities, DM helpers, CSV export
- Key files: `relay.ts`, `queryRelays.ts`, `dmUtils.ts`, `utils.ts`, `scheduler.ts`

**`src/pages/`:**
- Purpose: Route-bound page components
- Contains: Public pages (Index, Events, Blog, Feed, Profile, etc.)
- Key files: `Index.tsx`, `EventsPage.tsx`, `BlogPage.tsx`, `FeedPage.tsx`, `NIP19Page.tsx`

**`src/pages/admin/`:**
- Purpose: Admin route components (rendered inside AdminLayout)
- Contains: Thin wrappers that import from `src/components/admin/`
- Key files: `AdminWrapper.tsx` (auth protection), `AdminPage.tsx`, etc.

**`src/types/`:**
- Purpose: Shared TypeScript type definitions
- Contains: `zaplytics.ts` (analytics types), `scheduled.ts` (scheduled posts)

## Key File Locations

**Entry Points:**
- `src/main.tsx`: Application bootstrap, polyfills, ErrorBoundary wrapper
- `src/App.tsx`: Provider orchestration, default configuration
- `src/AppRouter.tsx`: Route definitions, nested admin routes
- `index.html`: HTML template with root div

**Configuration:**
- `vite.config.ts`: Vite dev server, path aliases (`@/*`), proxy setup, test config
- `tsconfig.json`: TypeScript configuration (strict: false, baseUrl: ".", paths: "@/*")
- `tailwind.config.ts`: Tailwind CSS configuration with custom theme
- `components.json`: shadcn/ui component configuration

**Core Logic:**
- `src/components/NostrProvider.tsx`: Nostr pool initialization and relay routing
- `src/components/NostrSync.tsx`: NIP-65 and site config sync from Nostr
- `src/components/AppProvider.tsx`: App config state, localStorage persistence, theme/SEO
- `src/lib/queryRelays.ts`: NIP-65 fanout query strategy
- `src/lib/relay.ts`: Relay URL resolution, master pubkey, API base URL

**Testing:**
- `src/test/setup.ts`: Vitest test setup with jsdom environment
- `src/test/TestApp.tsx`: Test wrapper with providers

## Naming Conventions

**Files:**
- Components: PascalCase (e.g., `AdminDashboard.tsx`, `ZapDialog.tsx`)
- Utilities/hooks: camelCase (e.g., `useNostrPublish.ts`, `queryRelays.ts`)
- Types: PascalCase (e.g., `zaplytics.ts`, `scheduled.ts`)
- Tests: Same name with `.test.tsx` or `.test.ts` suffix

**Directories:**
- Feature-based grouping (e.g., `admin/`, `auth/`, `dm/`, `zaplytics/`)
- Plural for utility collections (e.g., `hooks/`, `contexts/`, `types/`, `lib/`)

**Components:**
- Page components: Descriptive names (e.g., `EventsPage.tsx`, `BlogPostPage.tsx`)
- Feature components: Feature name + type (e.g., `AdminForms.tsx`, `DMChatArea.tsx`)
- UI components: Lowercase shadcn convention (e.g., `button.tsx`, `dialog.tsx`)

**Functions/Hooks:**
- Hooks: `use` prefix (e.g., `useZaps`, `useComments`, `useAppContext`)
- Utilities: Descriptive verbs (e.g., `normalizeToHexPubkeys`, `queryWithNip65Fanout`)
- Event handlers: `handle` prefix (e.g., `handleRefresh`, `handleSubmit`)

## Where to Add New Code

**New Page (Public):**
- Implementation: `src/pages/[PageName].tsx`
- Route: Add `<Route path="/path" element={<PageName />} />` in `src/AppRouter.tsx`

**New Admin Page:**
- Implementation: `src/components/admin/Admin[Feature].tsx`
- Route wrapper: `src/pages/admin/Admin[Feature]Page.tsx` (thin wrapper)
- Route: Add `<Route path="feature" element={<Admin[Feature]Page />} />` in `src/AppRouter.tsx`
- Sidebar: Add navigation entry in `src/components/admin/AdminLayout.tsx:94-113`

**New Feature Component:**
- Implementation: `src/components/[feature]/[Component].tsx`
- Use existing patterns in `admin/`, `auth/`, `dm/`, `zaplytics/` as reference

**New Hook:**
- Implementation: `src/hooks/use[Feature].ts`
- Export: Named export for the hook function
- Use `useNostr()`, `useAppContext()`, `useQuery()`/`useMutation()` as needed

**New Context:**
- Definition: `src/contexts/[ContextName].ts` or `.tsx`
- Provider: Create in same file or `src/components/[ContextName]Provider.tsx`
- Hook: Add `src/hooks/use[ContextName].ts` for type-safe access

**Utilities:**
- Shared helpers: `src/lib/[utility].ts`
- Feature-specific: `src/lib/[feature]/[utility].ts` (like `zaplytics/`)

**UI Components:**
- Prefer shadcn/ui: Use `npx shadcn-ui@latest add [component]` to add
- Custom UI: `src/components/ui/[component].tsx` following shadcn patterns

**Types:**
- Feature-specific types: Inline in component/hook file, or in `src/types/[feature].ts`
- Shared types: `src/types/common.ts` (create if needed)

## Special Directories

**`src/components/ui/`:**
- Purpose: shadcn/ui component library
- Generated: Yes (via shadcn CLI)
- Committed: Yes
- Note: Manual edits allowed but may be overwritten by CLI updates

**`src/test/`:**
- Purpose: Test configuration and utilities
- Generated: No
- Committed: Yes
- Contains: `setup.ts` (Vitest setup), `TestApp.tsx` (test wrapper)

**`eslint-rules/`:**
- Purpose: Custom ESLint rules for project-specific linting
- Generated: No
- Committed: Yes

**`docs/`:**
- Purpose: Additional documentation
- Generated: No
- Committed: Yes

**`.agent/skills/`:**
- Purpose: GSD agent skill definitions
- Generated: No
- Committed: Yes

**`.planning/codebase/`:**
- Purpose: Codebase analysis documents (this file and ARCHITECTURE.md)
- Generated: Yes (by GSD codebase mapper)
- Committed: Yes

**`scripts/`:**
- Purpose: Build and utility scripts (e.g., `generate-route-meta.mjs`)
- Generated: No
- Committed: Yes

---

*Structure analysis: 2026-05-09*
