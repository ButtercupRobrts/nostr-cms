# Future Phases

Reference document for features deferred during PR #72 review. Each phase has enough context to pick up for planning and discussion without re-investigating the codebase.

---

## Phase 1: Multi-Server Video Replication

### Origin

Identified as **Bug 3** in the Devin review of PR #72 ("Selected backup servers ignore videos"). Deferred because it's a feature gap, not a bug — the current single-server behavior works correctly, it just lacks the redundancy that images already have.

### Problem

When a user selects multiple Blossom servers for backup, images and videos are handled asymmetrically:

- **Images**: `BlossomUploader` fans out across all selected servers via `Promise.any`. If 3 servers are selected, the image is uploaded to all 3. If one goes down, the image is still accessible from the others.
- **Videos**: `streamProcessVideo` sends the video to the local relay's `/process-video-stream` endpoint, which runs `ffmpeg` and stores the result on **that one relay only**. The other selected servers never receive the video. If that relay goes down, the video is gone.

### Why it can't be a simple fix

Transcoding (`ffmpeg`) can only run on the local relay — you can't ask a remote Blossom server to transcode because:
1. The raw video file isn't on the remote server yet
2. The remote server may not have ffmpeg installed
3. Even if it did, you'd be uploading the raw file twice (once to transcode locally, once to transcode remotely)

So the replication must happen **after** transcoding completes, as a separate mirroring step.

### Proposed approach

After `streamProcessVideo` returns the processed video's hash and URL, mirror the processed blob to each additional selected Blossom server using the existing `/mirror` endpoint.

**Current flow (single-server):**
```
User selects video → streamProcessVideo(local relay) → ffmpeg transcode → stored on local relay → done
```

**Proposed flow (multi-server):**
```
User selects video → streamProcessVideo(local relay) → ffmpeg transcode → stored on local relay
  → for each additional selected server:
    → PUT /mirror { url: processedVideoUrl } on that server
  → all servers now have the processed video
```

### Key files

- `src/lib/mediaProcessing.ts` — `streamProcessVideo()` function (line ~335). Returns `{ sha256, url, size, original_size, original_sha, mime }`. This is where the mirroring step would be added, or a new wrapper function.
- `src/components/admin/AdminMedia.tsx` — `UploadMediaSection` component. The video upload path (line ~985) calls `streamProcessVideo` with only `blossomRelays[0]`. The full `blossomRelays` array is available but unused for videos.
- `src/components/admin/MediaSelectorDialog.tsx` — Also calls `streamProcessVideo` with only `uploadRelays[0]` (line ~219). Same pattern.
- Server: `/mirror` endpoint already exists on the Blossom server (used by the harvest feature). It takes a JSON body `{ url: "..." }` and copies the blob from the source URL.

### Implementation sketch

```typescript
// New function in mediaProcessing.ts
export async function streamProcessVideoAndReplicate(
  blossomUrl: string,        // local relay (does transcoding)
  file: File,
  quality: VideoQuality,
  resolution: VideoResolution,
  signer: { signEvent: (event: unknown) => Promise<unknown> },
  additionalServers: string[],  // other selected servers to mirror to
): Promise<VideoProcessResult> {
  // 1. Transcode on local relay (existing behavior)
  const result = await streamProcessVideo(blossomUrl, file, quality, resolution, signer);

  // 2. Mirror to additional servers
  if (additionalServers.length > 0) {
    await Promise.allSettled(
      additionalServers.map(async (server) => {
        const res = await fetch(`${server}/mirror`, {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            'X-Owner-Pubkey': ownerPubkey, // need to pass this in
          },
          body: JSON.stringify({ url: result.url }),
        });
        if (!res.ok) {
          console.warn(`Failed to mirror video to ${server}: HTTP ${res.status}`);
        }
      })
    );
  }

  return result;
}
```

### Considerations

1. **Auth for mirror endpoint**: The `/mirror` endpoint may require a Blossom auth event (kind 24242). Need to check if the existing harvest mirroring signs an auth event or if it's open. The harvest code in `AdminMedia.tsx` (line ~2050) uses `PUT /mirror` with `X-Owner-Pubkey` header but no auth event — so it may be open, or auth may be handled differently.

2. **Partial failure**: If mirroring to 2 of 3 servers fails, should the upload be considered successful? Probably yes — the video is on the local relay, and the user can be warned about the failed mirrors (same pattern as the GIF metadata warning).

3. **Bandwidth**: Mirroring a processed video to N servers means downloading it from the local relay and uploading it to each server. For large videos, this could be slow. Consider showing a progress indicator for the mirroring phase.

4. **Hash verification**: After mirroring, verify that the blob hash on the remote server matches `result.sha256`. The `/mirror` endpoint should return the hash; if it doesn't, a follow-up `/list` or HEAD request could confirm.

5. **UI**: The upload progress bar currently shows transcoding progress. If mirroring is added, the progress bar should either:
   - Show "Transcoding... then Mirroring to N servers..." as a two-phase progress
   - Or just show "Uploading..." and complete when all mirrors are done

6. **Kind 10063 server list**: The published server list (BUD-03) tells clients which servers to check for media. If videos are mirrored to multiple servers, clients will find them via the server list fallback — no additional metadata needed.

### Scope estimate

- 1 new function in `mediaProcessing.ts` (~30 lines)
- 2 call site updates in `AdminMedia.tsx` and `MediaSelectorDialog.tsx` (pass `blossomRelays.slice(1)` as additional servers)
- Minor UI update for mirroring progress
- Testing with multiple Blossom servers configured

Small feature, roughly half a day of work including testing.

---

## Phase 2: Server-Side Auto-Harvest Scheduler

### Origin

Identified as **Flag 4** in the Devin review of PR #72 ("Daily backup requires external implementation"). The `auto_harvest_24h` flag is published and synced across clients, but nothing reads it on a schedule.

### Problem

The "24h auto-backup" checkbox publishes `auto_harvest_24h = true` to the kind 30078 site-config event and runs one harvest immediately. But there's no scheduler that reads this flag and triggers daily harvests. The feature is half-built: the flag is stored and synced, but nothing acts on it daily.

The toast was updated to be honest: "Running a 24h harvest now. Daily scheduling requires server-side support (not yet implemented)."

### Proposed approach

Add a daily cron job to the Go server (`swarm/main.go`) that:
1. Reads the kind 30078 site-config event from the local relay
2. Checks if `auto_harvest_24h` is `true`
3. If yes, runs the same harvest logic that `runAutoHarvest24h` does on the client — scans all events from the last 24h and mirrors any media that isn't already stored

### Key files

- `swarm/main.go` — Server entry point. Already has a `Scheduler` (line ~266) for scheduled posts. A new cron job could be added alongside it.
- `nostr-cms/src/components/admin/AdminMedia.tsx` — `runAutoHarvest24h()` (line ~1688). The client-side harvest logic that would be replicated server-side.
- The server already has a `/mirror` endpoint and access to the relay's event store, so the harvest logic can be implemented entirely in Go.

### Considerations

1. **Server vs client**: A server-side scheduler is more reliable than a client-side one (runs even when no one has the browser open). But it requires the server to know which Blossom server to mirror to (the `targetBlossom` that the user selects in the UI).

2. **Config storage**: The `auto_harvest_24h` flag is in the kind 30078 event. The server needs to read this event from the local relay. The server already has relay access.

3. **Target Blossom server**: The client lets the user pick a target Blossom server. For the server-side scheduler, this would need to be stored in the site-config event (e.g., a `harvest_target_blossom` tag) or default to the local relay's Blossom endpoint.

4. **Rate limiting**: The harvest scans all events from the last 24h. If run daily, this is reasonable. But if the relay has many users, the scan could be expensive. Consider batching or limiting the scan to specific kinds.

### Scope estimate

- New Go function for the harvest logic (~100-150 lines, mirroring the client-side logic)
- Cron job setup in server startup (~20 lines)
- Config tag for target Blossom server (if needed)
- Testing

Medium feature, roughly 1-2 days of work including testing.
