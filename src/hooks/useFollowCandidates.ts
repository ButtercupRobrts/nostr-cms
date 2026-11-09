import { useQuery } from '@tanstack/react-query';
import { useNostr } from '@nostrify/react';
import { nip19 } from 'nostr-tools';
import { NSchema as n, type NostrEvent } from '@nostrify/nostrify';
import { useCurrentUser } from '@/hooks/useCurrentUser';
import { useAppContext } from '@/hooks/useAppContext';
import { getNip65ReadRelays } from '@/lib/queryRelays';
import type { MentionSearchResult } from '@/hooks/useMentionSearch';

const PROFILE_CHUNK = 100;
const MAX_FOLLOWS = 1500;
const FETCH_TIMEOUT_MS = 8000;

/**
 * Relays likely to hold social-graph data. The default pool only reads the
 * site relay — a team relay that doesn't store kind-3 contact lists or
 * arbitrary members' kind-0s — so the follows tier must query profile
 * relays explicitly, the same way useMentionSearch does.
 *
 * `purplepag.es` and `nostr.wine` index profiles; `relay.damus.io` and
 * `relay.nostr.band` are large general stores that keep contact lists.
 */
const SOCIAL_RELAYS = [
  'wss://purplepag.es',
  'wss://relay.damus.io',
  'wss://nostr.wine',
  'wss://relay.nostr.band',
];

/**
 * Profiles of people the current user follows (NIP-02 kind-3 contact list),
 * ranked ahead of external relay results in the mention dropdown. Follows
 * are the strongest available trust signal — a scammer impersonating a
 * followed account won't appear here.
 *
 * The kind-3 is fetched from the user's NIP-65 read relays first (canonical
 * location), then SOCIAL_RELAYS as fallback. Profiles are batch-fetched
 * across the same set, deduped newest-first. Cached for 5 minutes.
 */
export function useFollowedProfiles() {
  const { nostr } = useNostr();
  const { user } = useCurrentUser();
  const { config } = useAppContext();
  const nip65ReadRelays = getNip65ReadRelays(config.relayMetadata);

  return useQuery({
    queryKey: ['mention-followed-profiles', user?.pubkey, nip65ReadRelays],
    queryFn: async ({ signal }): Promise<MentionSearchResult[]> => {
      if (!user?.pubkey) return [];

      const abort = AbortSignal.any([signal, AbortSignal.timeout(FETCH_TIMEOUT_MS)]);
      const relays = Array.from(new Set([...nip65ReadRelays, ...SOCIAL_RELAYS]));

      // Find the newest kind-3 across relays — parallel so a dead relay
      // can't stall the probe sequence.
      let contactEvent: NostrEvent | undefined;
      const settled = await Promise.allSettled(
        relays.map((url) =>
          nostr.relay(url).query(
            [{ kinds: [3], authors: [user.pubkey], limit: 1 }],
            { signal: abort },
          ),
        ),
      );
      for (const r of settled) {
        if (r.status === 'fulfilled' && r.value[0] &&
          (!contactEvent || r.value[0].created_at > contactEvent.created_at)) {
          contactEvent = r.value[0];
        }
      }
      if (!contactEvent) return [];

      const followPubkeys = contactEvent.tags
        .filter((t) => t[0] === 'p' && t[1])
        .map((t) => t[1])
        .slice(0, MAX_FOLLOWS);
      if (followPubkeys.length === 0) return [];

      // Batch-fetch kind-0 profiles across the relay set in parallel —
      // relays return disjoint subsets; merge dedupes by pubkey.
      const profiles: NostrEvent[] = [];
      for (let i = 0; i < followPubkeys.length; i += PROFILE_CHUNK) {
        if (abort.aborted) break;
        const chunk = followPubkeys.slice(i, i + PROFILE_CHUNK);
        const settled = await Promise.allSettled(
          relays.map((url) =>
            nostr.relay(url).query(
              [{ kinds: [0], authors: chunk, limit: chunk.length }],
              { signal: abort },
            ),
          ),
        );
        for (const r of settled) {
          if (r.status === 'fulfilled') profiles.push(...r.value);
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
