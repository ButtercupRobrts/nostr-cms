# Coding Conventions

**Analysis Date:** 2026-05-09

## Naming Patterns

**Files:**
- Components: PascalCase (e.g., `FeedItem.tsx`, `Navigation.tsx`, `ZapButton.tsx`)
- Hooks: camelCase with `use` prefix (e.g., `useCurrentUser.ts`, `useNostrPublish.ts`, `useLocalStorage.ts`)
- Utilities: camelCase (e.g., `genUserName.ts`, `utils.ts`, `csvExport.ts`)
- Types: camelCase (e.g., `scheduled.ts`, `zaplytics.ts`)
- Pages: PascalCase (e.g., `Index.tsx`, `EventsPage.tsx`, `BlogPage.tsx`)
- Test files: PascalCase with `.test.ts` or `.test.tsx` suffix (e.g., `NoteContent.test.tsx`, `genUserName.test.ts`)

**Functions:**
- camelCase for regular functions (e.g., `formatPubkey`, `normalizeToHexPubkeys`, `genUserName`)
- `use` prefix for React hooks (e.g., `useCurrentUser`, `useAuthor`, `useAppContext`)

**Variables:**
- camelCase (e.g., `showReplyForm`, `displayName`, `timeAgo`)
- Underscore prefix (`_`) for intentionally unused variables (ESLint configured to allow `_` prefix pattern)

**Types/Interfaces:**
- PascalCase for interfaces and types (e.g., `AppConfig`, `NostrEvent`, `ScheduledPost`, `ZapReceipt`)
- Descriptive names that often reference domain concepts (e.g., `RelayMetadata`, `EarningsByPeriod`)

## Code Style

**Formatting:**
- No explicit Prettier config detected
- Uses `tailwind-merge` + `clsx` for conditional class names via `cn()` utility (`src/lib/utils.ts`)
- Consistent indentation and formatting across files

**Linting:**
- ESLint with `typescript-eslint` recommended config
- Custom ESLint rules in `eslint-rules/` directory:
  - `no-placeholder-comments`: Detects "// In a real" placeholder comments
  - `no-inline-script`: Prevents inline `<script>` tags in HTML
  - `require-webmanifest`: Ensures web manifest file exists and is linked
- `no-warning-comments` rule enabled for "fixme" terms
- React Hooks and React Refresh plugins enabled
- Unused variables with `_` prefix are allowed

**Import/Export:**
- ES modules (`import`/`export`)
- Absolute imports using `@/` alias for `src/` directory
- Explicit file extensions in imports (`.ts`, `.tsx`)

## Import Organization

**Observed order:**
1. External library imports (React, Nostr libraries, utilities)
2. Internal imports from `@/` aliases
3. Relative imports (rare, usually for co-located files)
4. Type imports (when using `import type`)

**Path Aliases:**
- `@/*` maps to `./src/*` (configured in `tsconfig.json` and `vite.config.ts`)
- `@/components` - UI components
- `@/hooks` - Custom React hooks
- `@/lib` - Utility functions
- `@/contexts` - React contexts
- `@/types` - TypeScript type definitions

## Error Handling

**Patterns:**
- Try-catch blocks for operations that may fail (e.g., localStorage access, Nostr operations)
- `console.warn()` for non-fatal errors with descriptive messages
- Generic fallback values (e.g., `return 'Anonymous'` when name generation fails)
- Error boundaries for React component errors (`src/components/ErrorBoundary.tsx`)
- Toast notifications for user-facing errors via `useToast` hook

**Example from `src/lib/utils.ts`:**
```typescript
export function formatPubkey(pubkey: string) {
  try {
    if (pubkey.startsWith('npub1')) return pubkey;
    return nip19.npubEncode(pubkey);
  } catch {
    return pubkey;
  }
}
```

## Logging

**Framework:** `console` methods (no external logging library detected)

**Patterns:**
- `console.warn()` for recoverable issues
- `console.error()` suppressed in tests via `vi.spyOn(console, 'error').mockImplementation(() => {})`
- Descriptive messages with context

## Comments

**JSDoc/TSDoc:**
- Used for complex utilities and hooks (e.g., `src/lib/genUserName.ts`, `src/hooks/useLocalStorage.ts`)
- Parameter and return type documentation
- Inline comments for business logic explanation

**When to Comment:**
- Above functions to explain purpose
- For non-obvious business logic
- For data migration handling (e.g., old navigation format in `AppProvider.tsx`)
- Stability warnings (e.g., "NOTE: This file is stable..." in `LoginArea.tsx`)

**Anti-pattern comments flagged:**
- "// In a real..." placeholder comments are flagged by custom ESLint rule
- "fixme" comments flagged by `no-warning-comments` rule

## Function Design

**Size:**
- No strict size limit observed
- Components tend to be larger (hundreds of lines) with multiple responsibilities
- Utility functions are smaller and focused

**Parameters:**
- Destructured props in components (e.g., `{ className }: LoginAreaProps`)
- Options objects for complex configurations
- Callback functions for state updates

**Return Values:**
- Hooks return tuples or objects with named properties
- Consistent return types from utility functions
- Explicit return typing in many functions

## Component Patterns

**Function components with hooks:**
- All components are function components (no class components)
- React hooks for state and side effects
- `React.forwardRef` for components that need ref forwarding
- `asChild` pattern for Radix UI polymorphic components

**State management:**
- Custom hooks for domain logic (e.g., `useCurrentUser`, `useNostrPublish`)
- Context API for app-wide state (`AppContext`, `NWCContext`, `DMContext`)
- TanStack Query for server state
- Local state with `useState` for component-specific state

**UI Components:**
- shadcn/ui pattern with Radix UI primitives
- `class-variance-authority` for variant-based styling
- Tailwind CSS for styling with CSS variables for theming
- Consistent `displayName` assignment on forwarded ref components

**Example from `src/components/ui/button.tsx`:**
```typescript
const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"
```

## TypeScript Conventions

**Configuration:**
- `strict: false` in `tsconfig.json` (non-strict mode)
- `strictNullChecks: true`
- `noImplicitAny: false`
- Path aliases: `@/*` for `src/*`

**Type patterns:**
- Interface for object shapes (e.g., `AppConfig`, `NostrEvent`)
- Type aliases for unions and literals (e.g., `Theme`, `ScheduledPostStatus`)
- Generic types for utilities (e.g., `useLocalStorage<T>`)
- `satisfies` operator for Zod schema typing

**Import type:**
- Some files use `import type` for type-only imports
- Many files use regular imports for both values and types

## Module Design

**Exports:**
- Named exports for utilities and hooks
- Default exports for some components (mixed convention)
- Barrel files in `eslint-rules/index.js`

**Barrel Files:**
- `eslint-rules/index.js` exports all custom rules
- `components/ui/` has individual component files (no barrel file)

## CSS/Styling Conventions

**Tailwind CSS:**
- Utility-first approach with Tailwind classes
- CSS variables for theme values in `src/index.css`
- `cn()` utility for merging class names (`src/lib/utils.ts`)
- Dark mode via `class` strategy
- Custom animations defined in `tailwind.config.ts`

**Theme variables:**
- HSL color format
- Semantic names: `--background`, `--foreground`, `--primary`, `--muted`
- Sidebar-specific variables for admin UI

## File Organization

**Co-location:**
- Test files next to source files (e.g., `NoteContent.tsx` + `NoteContent.test.tsx`)
- Some tests in `src/test/` directory for shared utilities

**Directory structure:**
- `src/components/` - React components
- `src/components/ui/` - Reusable UI components (shadcn/ui)
- `src/hooks/` - Custom React hooks
- `src/contexts/` - React context providers
- `src/lib/` - Utility functions
- `src/pages/` - Page components
- `src/types/` - TypeScript type definitions

---

*Convention analysis: 2026-05-09*
