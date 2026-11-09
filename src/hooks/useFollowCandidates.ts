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
export function useFollowedProfiles() {
  const { nostr } = useNostr();
  const { user } = useCurrentUser();
  const { config } = useAppContext();
  const nip65ReadRelays = getNip65ReadRelays(config.relayMetadata);

  return useQuery({
    queryKey: ['mention-followed-profiles', user?.pubkey, nip65ReadRelays],
    queryFn: async ({ signal }): Promise<MentionSearchResult[]> => {
      if (!user?.pubkey) return [];

      const relays = Array.from(new Set([...nip65ReadRelays, ...SOCIAL_RELAYS]));

      // Probe phase: collect kind-3 responses for a bounded window, then
      // take the NEWEST — relays can hold stale copies of the replaceable
      // contact list, so first-hit isn't enough (a stale kind-3 has been
      // observed in the wild: 8 follows vs the real 1765).
      const probeAbort = AbortSignal.any([
        signal,
        AbortSignal.timeout(FETCH_TIMEOUT_MS / 3),
      ]);
      let contactEvent: NostrEvent | undefined;
      let contactRelay: string | undefined;
      await new Promise<void>((resolve) => {
        let settled = 0;
        let done = false;
        const finish = () => {
          if (!done) { done = true; resolve(); }
        };
        for (const url of relays) {
          nostr.relay(url).query(
            [{ kinds: [3], authors: [user.pubkey], limit: 1 }],
            { signal: probeAbort },
          ).then((ev) => {
            if (ev[0] && (!contactEvent || ev[0].created_at > contactEvent.created_at)) {
              contactEvent = ev[0];
              contactRelay = url;
              // Grace period after the first hit lets slower relays
              // still beat a stale early answer on created_at.
              setTimeout(finish, 1500);
            }
          }).catch(() => {}).finally(() => {
            if (++settled === relays.length) finish();
          });
        }
        setTimeout(finish, FETCH_TIMEOUT_MS / 3);
      });
      if (!contactEvent) return [];

      const followPubkeys = contactEvent.tags
        .filter((t) => t[0] === 'p' && t[1])
        .map((t) => t[1])
        .slice(0, MAX_FOLLOWS);
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
      // One query() per relay with all chunk filters in a single REQ —
      // nostr.relay() opens a fresh websocket per call, and relays
      // (Primal included) drop rapid consecutive connection opens.
      // Independent budget: the probe phase already consumed part of the
      // overall window.
      const batchAbort = AbortSignal.any([
        signal,
        AbortSignal.timeout(FETCH_TIMEOUT_MS),
      ]);
      const profileSettled = await Promise.allSettled(
        batchRelays.map((url) =>
          nostr.relay(url).query(
            chunks.map((chunk) => ({
              kinds: [0],
              authors: chunk,
              limit: chunk.length,
            })),
            { signal: batchAbort },
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
      return results;
    },
    enabled: !!user?.pubkey,
    staleTime: 300_000,
  });
}
