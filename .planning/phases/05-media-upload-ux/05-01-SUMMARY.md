# 05-01 Summary: Drag & drop in MediaSelectorDialog upload tab via shared useFileDropzone hook

**Date:** 2026-09-26
**Status:** Complete — local change (buttercup first; upstream PR deferred until git workflow restored)

## Outcome
- New hook `src/hooks/useFileDropzone.ts` (~75 lines): single source of truth for dropzone drag state — `isDragging`, `dragCounter` (child-flicker-safe), 4 handlers, window `dragend`/`drop` reset, `disabled` flag, `onFiles(FileList)` callback.
- `MediaSelectorDialog.tsx` upload tab now accepts drops; benefits all 5 call sites (AdminNotes, AdminBlog, AdminEvents, CreateEventDialog, MarkdownToolbar).
- `AdminMedia.tsx` migrated to the hook — inline drag block deleted (~45 lines), markup/classes/labels unchanged (pure refactor).

## Behavior deltas (dialog path only)
- `handleUpload` split into `handleFiles(File[])` core + input wrapper — same dual-entry pattern as AdminMedia.
- Dropped files MIME-filtered to `image/|video/` (accept attr doesn't apply to drops); non-media drop → destructive toast.
- Images now EXIF/GPS-stripped via `stripImageMetadata` before upload — parity with AdminMedia and editor paths. GIFs pass through with a warning toast. Uploaded image bytes/hash therefore differ from source (re-encode, q0.95) — deliberate.
- Videos unchanged: raw `streamUpload`, no transcode option added.
- Drag styling/labels mirror AdminMedia: `border-primary bg-primary/10`, "Drop files here" / "Browse or drag & drop".

## Verified
- `tsc --noEmit`: 0 errors
- `eslint` on all touched files: clean
- `vitest run`: 27/27 passed (6 files — matches baseline)
- `vite build -l error`: exit 0
- `grep dragCounter|handleDrag*`: only `useFileDropzone.ts`
- **Manual (user, 2026-09-26): drag & drop upload in MediaSelectorDialog confirmed working** against live buttercup backend via https://buttercup.exe.xyz:8080/

## Dev-environment changes (same session, needed to test via exe.dev proxy)
- `vite.config.ts`: `allowedHosts: ["buttercup.exe.xyz"]` (vite 6 blocks proxy Host header → 403); `/.well-known/nostr.json` proxied to `localhost:8000` (static server has no CORS; public/ copy is sanitized-empty)
- `.env.local` (gitignored): `VITE_DEFAULT_RELAY=wss://buttercup.exe.xyz` (matches prod → same site config d-tag/blossom list), `VITE_MASTER_PUBKEY` set
- Dev instance talks to the LIVE backend — test uploads land in the real blossom library

## Follow-ups
- Package as standalone upstream PR to bitkarrot/nostr-cms once git metadata is restored (archived 2026-09-25). Diff is self-contained: 1 new file + 2 modified.
- Plan C candidate remains open: unify the three upload loops (dialog `handleFiles`, AdminMedia `handleFiles`, AdminNotes/AdminBlog editor drops) into `useBlossomUpload`.
- Optional extension: accept drops on the browse tab / whole dialog.
