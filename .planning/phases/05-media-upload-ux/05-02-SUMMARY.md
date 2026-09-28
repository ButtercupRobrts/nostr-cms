# 05-02 Summary: Unified blossom upload transport (Plan C)

**Date:** 2026-09-27
**Status:** Complete — code merged in working tree; NOT yet deployed to buttercup.exe.xyz (deploy = follow-up)

## Outcome
- New shared transport in `src/lib/mediaProcessing.ts`: `uploadMediaFile` (per-file: image→strip+BlossomUploader fan-out, video→streamUpload, AggregateError unwrap, sha/url extraction, `warnings[]`) and `uploadMediaFiles` (sequential loop + onProgress).
- `streamUpload` signer param tightened to `NostrSigner` — surfaced and fixed a stale `as Record<string, unknown>` cast on its signEvent result (removed; NostrEvent serializes directly).
- Migrated all four divergent loops: `MediaSelectorDialog.handleFiles`, `AdminNotes.handleFileUpload`, `AdminBlog.handleFileUpload`, `AdminMedia` transport tail (`stripMetadata:false` — it pre-processes), plus stretch task `useUploadFile`.

## Bugs fixed by consolidation (verified drift, now gone)
- **Editor video uploads always failed** — AdminNotes/AdminBlog called `stripImageMetadata` on every file; `loadImage` rejects non-images → "Upload Failed". Video drop/paste on note & blog editors works now.
- **iOS crash path** — editors sent videos through `BlossomUploader` (`file.arrayBuffer()`); now `streamUpload`.
- **Single-server uploads** — editors used `blossomRelays[0]`; now fan out to all configured relays (deliberate behavior change, picker parity).
- **Useless errors** — `AggregateError` now unwrapped in the helper for every path.
- **Unstripped uploads** — `useUploadFile` (profile pics, DM attachments) now strips EXIF/GPS too.

## Deliberate behavior changes (for PR body)
- Editors: multi-relay fan-out (was first relay only); success toast now says "N server(s)".
- Dialog: image auth tokens now 15 min (BlossomUploader default was silently 60 s).
- `useUploadFile`: images re-encoded/stripped; videos via streamUpload.
- GIF uploads anywhere → warning toast, still uploads.

## Verified
- `tsc --noEmit`: 0 errors · `eslint` touched files: clean · `vitest`: 27/27 (6 files)
- `grep BlossomUploader` → only mediaProcessing.ts + useFollowBackup.ts (encrypted backup blobs — out of scope: not user media, gains nothing from strip; flag for a future consistency pass if desired)
- `grep stripImageMetadata` → only mediaProcessing.ts (helper) + AdminMedia.tsx (pre-processing)
- `grep streamUpload(` → only inside uploadMediaFile

## Not verified (needs browser)
- Video drop on note/blog editor (headline fix)
- AdminMedia compress/transcode/pendingVideo regression
- useUploadFile callers (profile pic, DM attachment)

## Post-review fixes (user testing, 2026-09-27)
- **Editor file drops never worked** (pre-existing, both editors): `onDrop` was set but no `onDragOver` — HTML5 DnD won't fire `drop` for file drags without dragover prevention. Added `handleDragOver` claiming only `'Files'` drags (native text-drops preserved) + hardened `handleDrop` to claim all file drops so non-media files can't navigate the browser away. AdminNotes.tsx:669-685, AdminBlog.tsx:358-372.
- **AdminNotes had no upload indicator** — only a subtle button-icon spinner; AdminBlog's "Uploading media..." pattern ported over (AdminNotes.tsx:910-915). Explains "video drops silently" — large videos stream invisibly.
- **Scheduled posts 401 on prod** (pre-existing, unmasked by dev env): `fetchWithNip98` signed the `u` tag with the RELATIVE URL `/api/scheduler/list` (API_BASE='/api' in env-free prod builds). NIP-98 requires absolute URL → swarm 401s. Fixed: URL now resolved via `new URL(..., window.location.origin).href` (useScheduledPosts.ts:26-32). Confirmed via nginx access log: user's prod browser got 401 on every /api/scheduler/list hit. **User-verified on prod 2026-09-27.**
- **Typeless media files silently dropped** (user report: video drop "no reaction" on prod): `file.type` is empty for e.g. .mkv/.mov on Linux (no xdg MIME entry) → drop filters rejected it silently. Added `mediaMimeType()` extension-sniff fallback in mediaProcessing.ts; `uploadMediaFile` normalizes empty `file.type` via `new File([file], name, {type})` before processing so stored Content-Type is correct (prevents octet-stream blobs that won't inline-play). Drop filters in both editors, dialog, and AdminMedia's isImage/isVideo checks all use it; non-media drops toast "Unsupported files". **User-verified: same video file uploaded + transcoded after fix.**
- **GIF warning toast invisible** (user report): `TOAST_LIMIT = 1` in useToast.ts — warning toast was immediately displaced by "Upload Successful". Fix: warnings merged into the success toast text (destructive variant when warnings present); AdminMedia got its own per-file toast incl. compress-on path + honest "(GIF: uploaded as-is)" status instead of fake "→ WebP (-0%)". **User-verified 2026-09-27.**

## Transcode report investigation (user report, unresolved pending detail)
- Media-section transcode code path verified byte-for-byte intact (AdminMedia.tsx:907-932, panel 1232-1326).
- Most plausible cause: `processVideosOnUpload` is `useState(false)` — resets per mount. If unchecked at drop time, video uploads raw then shows pendingVideo OFFER panel (estimate only, needs "Process Video" click) — matches "no final size/preview" + "barely affected size".
- Alternative: transcode genuinely ran with tiny savings (medium CRF23 on already-compressed source), or uploaded via the media picker dialog which has NO transcode option (feature gap).

## Follow-ups
- Deploy to buttercup: build with `.env.local` moved aside → timestamped backup → `rsync -a --delete dist/ nostr-cms/dist/` (same as 05-01).
- Upstream: stacks on fork PR branches (#20 lineage), not upstream main.
- `useFollowBackup` BlossomUploader call could adopt the helper later (marginal — unwrap only).
