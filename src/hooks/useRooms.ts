/**
 * Hook for fetching and managing NIP-53 rooms (kind 30312).
 *
 * Provides room details fetching with caching, collision detection,
 * and room creation.
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useDefaultRelay } from './useDefaultRelay';
import { useCurrentUser } from './useCurrentUser';
import { parseRoomEvent, generateRoomId } from '@/lib/roomEvents';
import type { NostrEvent, NostrFilter, NRelay } from '@nostrify/nostrify';

/**
 * Simple fetch function for room details (not a hook).
 * Can be used inside query functions.
 */
export async function fetchRoomDetails(
  roomCoords: string,
  nostr: { relay: (url: string) => NRelay; query: (filters: NostrFilter[], opts: { signal?: AbortSignal }) => Promise<NostrEvent[]> },
): Promise<RoomDetails | null> {
  if (!roomCoords || !nostr) return null;

  // Parse coordinates: "30312:pubkey:d-tag"
  const [kind, pubkey, dTag] = roomCoords.split(':');
  if (kind !== '30312' || !pubkey || !dTag) return null;

  // Try hivetalk swarm relay first
  try {
    const hivetalkRelay = nostr.relay('wss://swarm.hivetalk.org');
    const signal = AbortSignal.timeout(5000);
    const events = await hivetalkRelay.query([
      { kinds: [30312], authors: [pubkey], '#d': [dTag], limit: 1 }
    ], { signal });

    if (events.length > 0) {
      return parseRoomEvent(events[0]);
    }
  } catch (error) {
    console.error('Failed to fetch room details from hivetalk swarm:', error);
  }

  // Fallback to default relay
  const signal = AbortSignal.timeout(5000);
  const events = await nostr.query([
    { kinds: [30312], authors: [pubkey], '#d': [dTag], limit: 1 }
  ], { signal });

  if (events.length === 0) return null;

  return parseRoomEvent(events[0]);
}

/**
 * Fetch room details by coordinates (a tag format: "30312:pubkey:d-tag").
 * Uses React Query caching with 5min stale time.
 */
export function useRoomDetails(roomCoords: string) {
  const { nostr } = useDefaultRelay();

  return useQuery({
    queryKey: ['room', roomCoords],
    queryFn: async () => {
      if (!roomCoords || !nostr) return null;

      // Parse coordinates: "30312:pubkey:d-tag"
      const [kind, pubkey, dTag] = roomCoords.split(':');
      if (kind !== '30312' || !pubkey || !dTag) return null;

      // Query for the room event
      const signal = AbortSignal.timeout(5000);
      const events = await nostr.query([
        { kinds: [30312], authors: [pubkey], '#d': [dTag], limit: 1 }
      ], { signal });

      if (events.length === 0) return null;

      return parseRoomEvent(events[0]);
    },
    enabled: !!roomCoords && !!nostr,
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: 2,
  });
}

/**
 * Fetch the current user's room events (kind 30312).
 * Note: Hivetalk relays (hivetalk.nostr1.com, honey.nostr1.com) are not accessible
 * from the browser due to network/firewall issues. This hook currently returns
 * empty since we can't query those relays.
 *
 * Users should use Custom URL mode for room selection.
 */
export function useUserRoomEvents(userPubkey: string | undefined) {
  const { poolNostr } = useDefaultRelay();

  return useQuery<NostrEvent[]>({
    queryKey: ['user-rooms', userPubkey],
    queryFn: async () => {
      // Hivetalk relays are not accessible from browser (NS_ERROR_WEBSOCKET_CONNECTION_REFUSED)
      // Return empty array - users should use Custom URL mode
      return [];
    },
    enabled: !!userPubkey && !!poolNostr,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Create a new room (kind 30312 event).
 * Note: Room creation is currently done directly in CreateEventDialog.
 * This hook is available for future use if needed.
 */
export function useCreateRoom() {
  const { user } = useCurrentUser();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      name: string;
      serviceUrl: string;
      status: 'open' | 'private' | 'closed';
      summary?: string;
      image?: string;
    }) => {
      if (!user) throw new Error('User is not logged in');

      const roomId = generateRoomId(input.name);
      const unsigned = {
        kind: 30312,
        content: input.summary || '',
        tags: [
          ['d', roomId],
          ['room', input.name],
          ['service', input.serviceUrl],
          ['status', input.status],
          ...(input.image ? [['image', input.image]] : []),
        ],
        created_at: Math.floor(Date.now() / 1000),
      };

      const signedEvent = await user.signer.signEvent(unsigned);

      // Publish to relays
      // TODO: Use useNostrPublish or similar
      // For now, just return the signed event
      return { signedEvent, roomId };
    },
    onSuccess: (_, _variables) => {
      // Invalidate user rooms cache
      if (user?.pubkey) {
        queryClient.invalidateQueries({
          queryKey: ['user-rooms', user.pubkey],
        });
      }
    },
  });
}
