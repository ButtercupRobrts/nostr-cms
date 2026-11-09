import { useQuery } from '@tanstack/react-query';
import { useNostr } from '@nostrify/react';
import { nip19 } from 'nostr-tools';
import { NSchema as n, type NostrEvent } from '@nostrify/nostrify';
import { useCurrentUser } from '@/hooks/useCurrentUser';
import { useAppContext } from '@/hooks/useAppContext';
import { getNip65ReadRelays } from '@/lib/queryRelays';
import type { MentionSearchResult } from '@/hooks/useMentionSearch';

const PROFILE_CHUNK = 400;
const MAX_FOLLOWS = 1600;
const FETCH_TIMEOUT_MS = 15_000;

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
/** Dev-only: last-fetch debug info, surfaced in MentionTextarea's dbg line. */
export const followsDbg: { pubkey?: string; contact?: boolean; follows?: number; profiles?: number } = {};

export function useFollowedProfiles() {
  const { nostr } = useNostr();
  const { user } = useCurrentUser();
  const { config } = useAppContext();
  const nip65ReadRelays = getNip65ReadRelays(config.relayMetadata);

  return useQuery({
    queryKey: ['mention-followed-profiles', user?.pubkey, nip65ReadRelays],
    queryFn: async ({ signal }): Promise<MentionSearchResult[]> => {
      if (!user?.pubkey) return [];
      followsDbg.pubkey = user.pubkey.slice(0, 8);

      const abort = AbortSignal.any([signal, AbortSignal.timeout(FETCH_TIMEOUT_MS)]);
      const relays = Array.from(new Set([...nip65ReadRelays, ...SOCIAL_RELAYS]));

      // Find the newest kind-3 across relays — parallel so a dead relay
      // can't stall the probe sequence. Record the relay that served it;
      // it's proven-reachable and leads the profile batch below.
      let contactEvent: NostrEvent | undefined;
      let contactRelay: string | undefined;
      const settled = await Promise.allSettled(
        relays.map((url) =>
          nostr.relay(url).query(
            [{ kinds: [3], authors: [user.pubkey], limit: 1 }],
            { signal: abort },
          ).then((ev) => ({ ev, url })),
        ),
      );
      for (const r of settled) {
        if (r.status === 'fulfilled' && r.value.ev[0] &&
          (!contactEvent || r.value.ev[0].created_at > contactEvent.created_at)) {
          contactEvent = r.value.ev[0];
          contactRelay = r.value.url;
        }
      }
      followsDbg.contact = !!contactEvent;
      if (!contactEvent) return [];

      const followPubkeys = contactEvent.tags
        .filter((t) => t[0] === 'p' && t[1])
        .map((t) => t[1])
        .slice(0, MAX_FOLLOWS);
      followsDbg.follows = followPubkeys.length;
      if (followPubkeys.length === 0) return [];

      // Batch-fetch kind-0 profiles — every chunk × every relay in
      // parallel; relays return disjoint subsets and merge dedupes by
      // pubkey. Sequential chunking risked the fetch timeout firing
      // before later chunks were ever requested.
      const chunks: string[][] = [];
      for (let i = 0; i < followPubkeys.length; i += PROFILE_CHUNK) {
        chunks.push(followPubkeys.slice(i, i + PROFILE_CHUNK));
      }
      // The relay that served the kind-3 is proven reachable — lead with
      // it and cap the fanout so a dead relay can't starve the batch
      // inside the fetch budget.
      const batchRelays = Array.from(
        new Set([contactRelay, ...relays].filter((r): r is string => !!r)),
      ).slice(0, 3);
      const profiles: NostrEvent[] = [];
      const profileSettled = await Promise.allSettled(
        chunks.flatMap((chunk) =>
          batchRelays.map((url) =>
            nostr.relay(url).query(
              [{ kinds: [0], authors: chunk, limit: chunk.length }],
              { signal: abort },
            ),
          ),
        ),
      );
      for (const r of profileSettled) {
        if (r.status === 'fulfilled') profiles.push(...r.value);
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
      followsDbg.profiles = results.length;
      return results;
    },
    enabled: !!user?.pubkey,
    staleTime: 300_000,
  });
}
