# Testing Patterns

**Analysis Date:** 2026-05-09

## Test Framework

**Runner:**
- Vitest 3.1.4
- Config: `vite.config.ts` (test section)

**Assertion Library:**
- Vitest built-in assertions
- `@testing-library/jest-dom` for DOM assertions

**Testing Libraries:**
- `@testing-library/react` 16.3.0
- `@testing-library/jest-dom` 6.6.3
- `jsdom` 26.1.0 (test environment)

**Run Commands:**
```bash
npm test              # Run full test suite (tsc + eslint + vitest + build)
npm run test          # Same as above
vitest                # Run Vitest in watch mode (interactive)
vitest run            # Run tests once
```

**Full test command (`package.json`):**
```bash
tsc --noEmit && eslint && vitest run --reporter=dot --silent && vite build -l error
```
This runs: TypeScript check, ESLint, Vitest, and production build.

## Test File Organization

**Location:**
- Mixed: Some tests co-located with source files, some in `src/test/`

**Co-located tests:**
- `src/components/NoteContent.test.tsx` (with `NoteContent.tsx`)
- `src/lib/genUserName.test.ts` (with `genUserName.ts`)

**Shared test utilities:**
- `src/test/setup.ts` - Global test setup
- `src/test/TestApp.tsx` - Test wrapper component
- `src/test/ErrorBoundary.test.tsx` - Error boundary tests

**Naming:**
- Same name as source file with `.test.ts` or `.test.tsx` suffix
- Test files mirror source file structure

**Structure:**
```
src/
├── test/
│   ├── setup.ts           # Global test configuration
│   ├── TestApp.tsx        # Test wrapper with providers
│   └── ErrorBoundary.test.tsx
├── components/
│   └── NoteContent.test.tsx
└── lib/
    └── genUserName.test.ts
```

## Test Structure

**Suite Organization:**
```typescript
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

describe('ComponentName', () => {
  it('does something specific', () => {
    // Arrange
    // Act
    // Assert
  });

  it('handles edge case', () => {
    // Test implementation
  });
});
```

**Setup pattern (from `src/test/setup.ts`):**
- Imports `@testing-library/jest-dom` for custom matchers
- Mocks `localStorage` with full interface
- Mocks `window.matchMedia`
- Mocks `window.scrollTo`
- Mocks `IntersectionObserver`
- Mocks `ResizeObserver`

**Teardown pattern:**
- Vitest handles automatic cleanup between tests
- Console spies restored with `mockRestore()`

**Assertion pattern:**
```typescript
// From NoteContent.test.tsx
expect(screen.getByRole('link', { name: 'https://example.com' })).toBeInTheDocument();
expect(link).toHaveAttribute('href', 'https://example.com');
expect(link).toHaveAttribute('target', '_blank');
```

## Mocking

**Framework:** Vitest (`vi`)

**Patterns:**

**Console error suppression:**
```typescript
// From ErrorBoundary.test.tsx
const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
// ... test code ...
consoleSpy.mockRestore();
```

**Window API mocking:**
```typescript
// From setup.ts
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    // ... other properties
  })),
});
```

**Observer mocking:**
```typescript
// IntersectionObserver
global.IntersectionObserver = vi.fn().mockImplementation((_callback) => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}));
```

**What to Mock:**
- Browser APIs (`localStorage`, `matchMedia`, `scrollTo`)
- Observers (`IntersectionObserver`, `ResizeObserver`)
- Console errors in error boundary tests
- External dependencies when needed

**What NOT to Mock:**
- React components (use TestApp wrapper instead)
- Utility functions under test

## Fixtures and Factories

**Test Data:**
```typescript
// From genUserName.test.ts
const event: NostrEvent = {
  id: 'test-id',
  pubkey: 'test-pubkey',
  created_at: Math.floor(Date.now() / 1000),
  kind: 1,
  tags: [],
  content: 'Check out this link: https://example.com',
  sig: 'test-sig',
};
```

**Location:**
- Test data inline in test files
- No centralized fixtures directory

**Test App Wrapper (`src/test/TestApp.tsx`):**
- Wraps components with all necessary providers:
  - `UnheadProvider` (head management)
  - `AppProvider` (app config)
  - `QueryClientProvider` (TanStack Query with retry disabled)
  - `NostrLoginProvider` (Nostr authentication)
  - `NostrProvider` (Nostr relay connection)
  - `NWCProvider` (Nostr Wallet Connect)
  - `BrowserRouter` (routing)

**Usage:**
```typescript
render(
  <TestApp>
    <NoteContent event={event} />
  </TestApp>
);
```

## Coverage

**Requirements:** None explicitly enforced

**Current coverage (as of 2026-05-09):**
- 4 test files in `src/` (excluding node_modules)
- 76 test assertions (describe/it/test/expect occurrences)
- ~199 source files
- **Estimated coverage: <5%** (very low)

**Tested files:**
- `src/lib/genUserName.ts` - Fully tested
- `src/components/NoteContent.tsx` - Well tested (multiple cases)
- `src/components/ErrorBoundary.tsx` - Well tested
- `src/App.tsx` - Minimal test (smoke test only)

**Not tested (major gaps):**
- Most hooks (`src/hooks/` - 26 files, 0 tests)
- Most pages (`src/pages/` - 15 files, 0 tests)
- Most UI components (`src/components/ui/` - 50+ files, 0 tests)
- Context providers
- Utility functions (except `genUserName`)

## Test Types

**Unit Tests:**
- **Scope:** Isolated functions and utilities
- **Example:** `genUserName.test.ts` tests deterministic name generation
- Pure function testing with direct assertions

**Integration Tests:**
- **Scope:** Component rendering with providers
- **Example:** `NoteContent.test.tsx` tests linkification in context
- Uses `TestApp` wrapper for provider context

**E2E Tests:**
- Not used

## Common Patterns

**Async Testing:**
- No async tests detected in current test suite
- TanStack Query configured with `retry: false` for tests

**Error Testing:**
```typescript
// From ErrorBoundary.test.tsx
it('catches and displays error when child throws', () => {
  const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

  render(
    <ErrorBoundary>
      <ThrowError shouldThrow={true} />
    </ErrorBoundary>
  );

  expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  consoleSpy.mockRestore();
});
```

**Role-based element selection:**
```typescript
screen.getByRole('link', { name: 'https://example.com' })
screen.getByText('Test content')
```

**Test doubles:**
- ThrowError component for testing error boundaries
- Mock implementations for browser APIs

## CI/CD Integration

**GitLab CI (`.gitlab-ci.yml`):**
```yaml
test:
  stage: test
  script:
    - npm run test
```

**GitHub Actions (`.github/workflows/codeql.yml`):**
- CodeQL analysis configured
- No explicit test job in visible workflow

## Testing Gaps and Recommendations

**Critical gaps:**
1. **Hooks:** Zero test coverage for 26+ custom hooks
2. **Pages:** No page component tests (routing, forms, authentication)
3. **Forms:** No tests for form validation or submission
4. **Admin functionality:** No tests for admin routes and protected features
5. **Nostr integration:** No tests for relay communication or event publishing
6. **Wallet features:** No tests for zap/NWC functionality

**Recommended additions:**
1. Add hook tests using `@testing-library/react-hooks`
2. Add integration tests for key user flows (login, post creation)
3. Add tests for form components with `react-hook-form` validation
4. Mock Nostr relay responses for testing
5. Add visual regression tests for UI consistency

---

*Testing analysis: 2026-05-09*
