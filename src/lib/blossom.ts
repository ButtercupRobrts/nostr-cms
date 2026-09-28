import type { NostrSigner } from '@nostrify/nostrify';

/**
 * Shared Blossom media utilities used by AdminMedia and MediaSelectorDialog.
 */

export interface BlossomBlob {
  url: string;
  sha256: string;
  size: number;
  type?: string;
  uploaded?: number;
  owner?: string;
}

const EXT_MAP: Record<string, string> = {
  'image/jpeg': '.jpg',
  'image/jpg': '.jpg',
  'image/png': '.png',
  'image/gif': '.gif',
  'image/webp': '.webp',
  'image/avif': '.avif',
  'image/bmp': '.bmp',
  'image/svg+xml': '.svg',
  'image/heic': '.heic',
  'image/heif': '.heif',
  'video/mp4': '.mp4',
  'video/webm': '.webm',
  'video/ogg': '.ogg',
  'video/quicktime': '.mov',
  'video/x-matroska': '.mkv',
  'audio/mpeg': '.mp3',
  'audio/ogg': '.ogg',
  'audio/wav': '.wav',
  'audio/webm': '.weba',
  'application/pdf': '.pdf',
};

/**
 * Append the correct file extension to Blossom URLs (bare sha256 hashes).
 * Some browsers (Safari) won't render <video> elements without an extension.
 * Inserts the extension before any query string or fragment.
 */
export function urlWithExtension(blob: BlossomBlob): string {
  try {
    const url = new URL(blob.url);
    if (/\.[a-zA-Z0-9]{2,5}$/.test(url.pathname)) return blob.url;

    const mime = (blob.type || '').toLowerCase();
    const ext = EXT_MAP[mime];
    if (!ext) return blob.url;
    url.pathname += ext;
    return url.toString();
  } catch {
    // Malformed URL — return as-is rather than corrupting it
    return blob.url;
  }
}

/**
 * Determine if a blob can be previewed as an image or video.
 * AVIF/HEIC/HEIF return null because canvas can't re-encode them and
 * browser support is inconsistent.
 */
export function getMediaPreviewKind(blob: BlossomBlob): 'image' | 'video' | null {
  const mime = (blob.type || '').toLowerCase();

  if (mime === 'image/avif' || mime === 'image/heic' || mime === 'image/heif') {
    return null;
  }

  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('video/')) return 'video';
  return null;
}

/**
 * Fetch the BUD-02 /list/<pubkey> blob index for a server.
 *
 * Tries anonymously first — same-origin relays serve public lists and remote
 * servers often do too, which avoids a signer prompt. Only on a 401/403 does
 * it retry once with a signed kind-24242 list authorization (Nostr header,
 * same scheme AdminMedia uses for media browsing).
 *
 * Returns null when the list is unavailable (network error, non-OK status,
 * rejected signature, or malformed body) — callers distinguish that from an
 * empty list, which is a valid [] response.
 */
export async function fetchBlossomList(
  server: string,
  pubkey: string,
  signer?: NostrSigner,
): Promise<BlossomBlob[] | null> {
  const url = `${server.replace(/\/+$/, '')}/list/${pubkey}`;

  let res: Response;
  try {
    res = await fetch(url, { signal: AbortSignal.timeout(10_000) });
  } catch {
    return null;
  }

  if ((res.status === 401 || res.status === 403) && signer) {
    try {
      const authEvent = await signer.signEvent({
        kind: 24242,
        content: 'List my blobs',
        tags: [
          ['t', 'list'],
          ['expiration', String(Math.floor(Date.now() / 1000) + 300)],
        ],
        created_at: Math.floor(Date.now() / 1000),
      });
      res = await fetch(url, {
        signal: AbortSignal.timeout(10_000),
        headers: { Authorization: `Nostr ${btoa(JSON.stringify(authEvent))}` },
      });
    } catch {
      return null;
    }
  }

  if (!res.ok) return null;
  try {
    const list = await res.json();
    return Array.isArray(list) ? (list as BlossomBlob[]) : null;
  } catch {
    return null;
  }
}
