/**
 * Hook for creating NIP-18 reposts (kind 6 / kind 16).
 *
 * Supports:
 * - Immediate repost (publish now)
 * - Scheduled repost (publish at a future time via the scheduler)
 * - Repeating scheduled reposts (multiple reposts at regular intervals)
 */

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCurrentUser } from './useCurrentUser';
import { useNostrPublish } from './useNostrPublish';
import { schedulePostViaApi } from './useScheduledPosts';
import { buildRepostEvent, type RepostTarget } from '@/lib/repost';
import type { NostrEvent } from '@/types/scheduled';

export interface RepeatConfig {
  count: number; // 1-10
  intervalMs: number; // milliseconds between reposts
}

export interface CreateRepostOptions {
  target: RepostTarget;
  relayUrl: string; // relay where the original event can be found
  scheduleFor?: Date | null; // if set, schedules; if null, publishes immediately
  publishRelays: string[]; // relays to publish/schedule to
  repeat?: RepeatConfig | null; // optional: schedule multiple reposts
}

interface CreateRepostResult {
  signedEvent: NostrEvent;
  scheduledPostIds?: string[];
}

export function useCreateRepost() {
  const { user } = useCurrentUser();
  const publishEvent = useNostrPublish();
  const queryClient = useQueryClient();

  return useMutation<CreateRepostResult, Error, CreateRepostOptions>({
    mutationFn: async (opts: CreateRepostOptions) => {
      if (!user) throw new Error('User is not logged in');

      const { target, relayUrl, scheduleFor, publishRelays, repeat } = opts;

      // --- Immediate repost (no scheduling) ---
      if (!scheduleFor) {
        const createdAt = Math.floor(Date.now() / 1000);
        const unsigned = buildRepostEvent(target, relayUrl, createdAt);
        const signedEvent = await user.signer.signEvent(unsigned);
        await publishEvent.mutateAsync({
          event: {
            kind: signedEvent.kind,
            content: signedEvent.content,
            tags: signedEvent.tags,
            created_at: signedEvent.created_at,
          },
          relays: publishRelays,
        });
        return { signedEvent };
      }

      // --- Scheduled repost(s) ---
      // We call the scheduler API directly instead of using useCreateScheduledPost,
      // because calling mutateAsync in a loop triggers onSuccess → query invalidation
      // → refetch on each iteration. The refetch signs a NIP-98 event via
      // window.nostr, which races with the next loop iteration's signing request.
      // Browser extensions can only handle one signing request at a time, so the
      // second request fails, causing all subsequent posts to be lost.
      // By calling the API directly and invalidating once at the end, we avoid
      // the race condition entirely.
      const count = repeat?.count ?? 1;
      const intervalMs = repeat?.intervalMs ?? 0;
      const scheduledPostIds: string[] = [];
      let lastSignedEvent: NostrEvent | null = null;

      for (let i = 0; i < count; i++) {
        const scheduledDate = new Date(scheduleFor.getTime() + i * intervalMs);
        const createdAt = Math.floor(scheduledDate.getTime() / 1000);
        const unsigned = buildRepostEvent(target, relayUrl, createdAt, {
          total: count,
          index: i,
          intervalMs,
        });
        const signedEvent = await user.signer.signEvent(unsigned);
        lastSignedEvent = signedEvent;

        // Call the scheduler API directly (no query invalidation per call)
        const result = await schedulePostViaApi({
          signedEvent,
          relays: publishRelays,
          scheduledFor: scheduledDate,
        });

        if (result?.id) {
          scheduledPostIds.push(result.id);
        }
      }

      // Invalidate the scheduled posts query ONCE after all posts are created
      queryClient.invalidateQueries({
        queryKey: ['scheduled-posts', user.pubkey],
      });
      queryClient.invalidateQueries({
        queryKey: ['scheduled-posts-stats', user.pubkey],
      });

      return { signedEvent: lastSignedEvent!, scheduledPostIds };
    },
  });
}
