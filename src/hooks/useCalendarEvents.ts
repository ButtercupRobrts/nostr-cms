/**
 * Hook for fetching and normalizing calendar events.
 *
 * Queries for both NIP-52 (31922/31923) and NIP-53 (30313) events,
 * normalizes them to a unified interface, and provides filtering.
 */

import { useQuery } from '@tanstack/react-query';
import { useDefaultRelay } from './useDefaultRelay';
import { useAppContext } from './useAppContext';
import { useRemoteNostrJson } from './useRemoteNostrJson';
import { normalizeEvent, type UnifiedCalendarEvent } from '@/lib/calendarEvents';
import type { NostrEvent } from '@nostrify/nostrify';

export type EventFilter = 'all' | 'calendar' | 'live';

/**
 * Fetch and normalize calendar events.
 *
 * Uses useRoomDetails for proper room details caching.
 */
export function useCalendarEvents(
  userPubkey: string | undefined,
  filter: EventFilter = 'all',
) {
  const { poolNostr, publishRelays } = useDefaultRelay();
  const { config: appContext } = useAppContext();
  const { data: nostrJson } = useRemoteNostrJson();

  return useQuery({
    queryKey: ['calendar-events', userPubkey, filter, nostrJson],
    queryFn: async () => {
      if (!poolNostr) return [];

      // Build the set of allowed pubkeys from nostr.json + admin_roles
      const allowedPubkeys = new Set<string>();
      if (nostrJson?.names) {
        for (const pk of Object.values(nostrJson.names)) {
          allowedPubkeys.add(pk.toLowerCase().trim());
        }
      }
      const adminRoles = appContext?.siteConfig?.adminRoles;
      if (adminRoles) {
        for (const [pk, role] of Object.entries(adminRoles)) {
          if (role === 'publisher') {
            allowedPubkeys.add(pk.toLowerCase().trim());
          }
        }
      }

      const signal = AbortSignal.timeout(10000);

      // Query from default relay only (same relay we publish to)
      let events;
      try {
        events = await poolNostr.query([
          { kinds: [31922, 31923, 30313], limit: 100 }
        ], { signal });
      } catch (error) {
        return [];
      }

      // Filter to only whitelisted pubkeys (if we have a whitelist)
      const filteredEvents = allowedPubkeys.size > 0
        ? events.filter(e => allowedPubkeys.has(e.pubkey.toLowerCase().trim()))
        : events;

      // Normalize each event and deduplicate by ID
      const eventMap = new Map<string, UnifiedCalendarEvent>();

      const fetchRoom = (coords: string) => fetchRoomDetails(coords, poolNostr);

      for (const event of filteredEvents) {
        try {
          // Skip 30312 room events - they're room definitions, not displayable events
          if (event.kind === 30312) {
            continue;
          }

          const normalizedEvent = await normalizeEvent(event, fetchRoom);

          // Deduplicate: keep the most recent event (highest created_at)
          const existing = eventMap.get(event.id);
          if (!existing || event.created_at > existing.created_at) {
            eventMap.set(event.id, normalizedEvent);
          }
        } catch (error) {
          // Skip malformed events
        }
      }

      const normalizedEvents = Array.from(eventMap.values());

      // Apply type filter
      if (filter === 'calendar') {
        return normalizedEvents.filter(e => e.type === 'calendar');
      }
      if (filter === 'live') {
        return normalizedEvents.filter(e => e.type === 'live');
      }
      return normalizedEvents;
    },
    enabled: !!poolNostr,
    staleTime: 5 * 60 * 1000, // 5 minutes
    refetchInterval: 30 * 1000, // Poll every 30 seconds for live status updates
  });
}
