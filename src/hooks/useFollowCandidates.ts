import { useQuery } from '@tanstack/react-query';
import { useNostr } from '@nostrify/react';
import { nip19 } from 'nostr-tools';
import { NSchema as n, type NostrEvent } from '@nostrify/nostrify';
import { useCurrentUser } from '@/hooks/useCurrentUser';
import type { MentionSearchResult } from '@/hooks/useMentionSearch';

const PROFILE_CHUNK = 100;
const MAX_FOLLOWS = 1500;

/**
 * Profiles of people the current user follows (NIP-02 kind-3 contact list),
 * ranked ahead of external relay results in the mention dropdown. Follows
 * are the strongest available trust signal — a scammer impersonating a
 * followed account won't appear here.
 *
 * The kind-3 event and the kind-0 profile fetches are cached for 5 minutes;
 * contact lists change rarely and profile data is largely static.
 */
export function useFollowedProfiles() {
  const { nostr } = useNostr();
  const { user } = useCurrentUser();

  return useQuery({
    queryKey: ['mention-followed-profiles', user?.pubkey],
    queryFn: async ({ signal }): Promise<MentionSearchResult[]> => {
      if (!user?.pubkey) return [];

      const [contactEvent] = await nostr.query(
        [{ kinds: [3], authors: [user.pubkey], limit: 1 }],
        { signal },
      );
      if (!contactEvent) return [];

      const followPubkeys = contactEvent.tags
        .filter((t) => t[0] === 'p' && t[1])
        .map((t) => t[1])
        .slice(0, MAX_FOLLOWS);
      if (followPubkeys.length === 0) return [];

      // Batch-fetch kind-0 profiles for the follows — relay `authors`
      // filters accept long lists; chunk to stay polite.
      const profiles: NostrEvent[] = [];
      for (let i = 0; i < followPubkeys.length; i += PROFILE_CHUNK) {
        if (signal.aborted) break;
        const chunk = followPubkeys.slice(i, i + PROFILE_CHUNK);
        try {
          profiles.push(
            ...(await nostr.query(
              [{ kinds: [0], authors: chunk, limit: chunk.length }],
              { signal },
            )),
          );
        } catch {
          // partial results are fine — a relay that fails one chunk may
          // still serve the next
        }
      }

      // Dedupe by pubkey — newest kind-0 wins.
      const byPubkey = new Map<string, NostrEvent>();
      for (const e of profiles) {
        const existing = byPubkey.get(e.pubkey);
        if (!existing || e.created_at > existing.created_at) {
          byPubkey.set(e.pubkey, e);
        }
      }

      const results: MentionSearchResult[] = [];
      for (const event of byPubkey.values()) {
        try {
          const metadata = n.json().pipe(n.metadata()).parse(event.content);
          results.push({
            pubkey: event.pubkey,
            npub: nip19.npubEncode(event.pubkey),
            name: metadata.name,
            displayName: metadata.display_name,
            picture: metadata.picture,
            nip05: metadata.nip05,
          });
        } catch {
          // skip unparseable metadata
        }
      }
      return results;
    },
    enabled: !!user?.pubkey,
    staleTime: 300_000,
  });
}
