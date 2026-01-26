import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { nip19 } from 'nostr-tools'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Formats a pubkey (hex or npub) for display as an npub.
 */
export function formatPubkey(pubkey: string) {
  try {
    if (pubkey.startsWith('npub1')) return pubkey;
    return nip19.npubEncode(pubkey);
  } catch {
    return pubkey;
  }
}

/**
 * Normalizes a list of identifiers (hex or npub) into hex pubkeys.
 */
export function normalizeToHexPubkeys(ids: (string | undefined | null)[]): string[] {
  if (!ids || !Array.isArray(ids)) return [];
  
  return ids
    .filter((id): id is string => !!id && typeof id === 'string')
    .map(id => {
      const trimmed = id.trim().toLowerCase();
      // If it's already a hex pubkey
      if (trimmed.length === 64 && /^[0-9a-f]+$/.test(trimmed)) {
        return trimmed;
      }
      // If it's an npub
      try {
        const decoded = nip19.decode(trimmed);
        if (decoded.type === 'npub') return decoded.data as string;
        return null;
      } catch {
        return null;
      }
    })
    .filter((pk): pk is string => !!pk);
}
