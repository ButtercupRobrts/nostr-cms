import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useNostr } from '@nostrify/react';
import { useCurrentUser } from '@/hooks/useCurrentUser';
import { useNostrJsonUsers } from '@/hooks/useNostrJsonUsers';
import type { NostrEvent } from '@nostrify/nostrify';
import type { NStore } from '@nostrify/types';
import type {
  AnalyticsData,
  ParsedZap,
  ZapReceipt,
  CustomDateRange,
  TimeRange,
} from '@/types/zaplytics';
import {
  isValidZapReceipt,
  parseZapReceipt,
  getDateRange,
  groupZapsByPeriod,
  groupZapsByContent,
  groupZapsByKind,
  getTopZappers,
  groupZapsByHour,
  groupZapsByDayOfWeek,
  analyzeZapperLoyalty,
} from '@/lib/zaplytics/utils';

export interface MemberStats {
  pubkey: string;
  name: string;
  totalEarnings: number;
  totalZaps: number;
  uniqueZappers: number;
  topContentSats: number;
  topContentPreview: string;
  earningsByPeriod?: ReturnType<typeof groupZapsByPeriod>;
  topZappers?: ReturnType<typeof getTopZappers>;
}

export interface CommunityZapStats {
  aggregate: AnalyticsData;
  members: MemberStats[];
  lastUpdated: number;
}

export type CommunityTimeRange = '24h' | '7d' | '30d' | 'all';

const RECEIPT_LIMIT = 5000;
const CHUNK = 150;

/** '30d' isn't a single-pubkey TimeRange — express it as a custom window. */
function toRangeParams(timeRange: CommunityTimeRange): { tr: TimeRange; custom?: CustomDateRange } {
  if (timeRange === '30d') {
    const to = new Date();
    const from = new Date(to.getTime() - 30 * 24 * 60 * 60 * 1000);
    return { tr: 'custom', custom: { from, to } };
  }
  return { tr: timeRange };
}

async function queryChunked(nostr: NStore, ids: string[], signal: AbortSignal): Promise<NostrEvent[]> {
  const out: NostrEvent[] = [];
  for (let i = 0; i < ids.length; i += CHUNK) {
    try {
      out.push(...(await nostr.query([{ ids: ids.slice(i, i + CHUNK) }], { signal })));
    } catch {
      // chunk failed — continue with what resolved
    }
  }
  return out;
}

export function useCommunityZapStats(timeRange: CommunityTimeRange = 'all') {
  const { user } = useCurrentUser();
  const { nostr } = useNostr();
  const queryClient = useQueryClient();
  const { data: nostrJsonUsers } = useNostrJsonUsers();

  const members = (nostrJsonUsers?.users || [])
    .filter((u) => u.pubkey)
    .map((u) => ({ name: u.name, pubkey: u.pubkey.toLowerCase().trim() }));
  const memberKey = members.map((m) => m.pubkey).sort().join(',');

  const { data, isLoading, error } = useQuery({
    queryKey: ['community-zap-stats', memberKey, timeRange],
    queryFn: async (): Promise<CommunityZapStats> => {
      const memberSet = new Set(members.map((m) => m.pubkey));
      const nameByPubkey = new Map(members.map((m) => [m.pubkey, m.name]));
      if (memberSet.size === 0) {
        return { aggregate: emptyAnalytics(), members: [], lastUpdated: Date.now() };
      }

      const { tr, custom } = toRangeParams(timeRange);
      const { since, until } = getDateRange(tr, custom);
      const signal = AbortSignal.timeout(60_000);

      // One query: all kind-9735 receipts addressed to any member (#p OR-match)
      const events = await nostr.query(
        [{
          kinds: [9735],
          '#p': [...memberSet],
          limit: RECEIPT_LIMIT,
          ...(since ? { since } : {}),
          ...(until ? { until } : {}),
        }],
        { signal },
      );

      const receipts = events
        .filter((e): e is ZapReceipt => isValidZapReceipt(e as NostrEvent));
      const parsedZaps = receipts
        .map(parseZapReceipt)
        .filter((z): z is ParsedZap => z !== null);

      // Enrich zapped events (e-tags → real kind/content/author/created_at)
      const eIds = [...new Set(
        parsedZaps.map((z) => z.zappedEvent?.id).filter((id): id is string => !!id),
      )];
      const contentMap = new Map<string, NostrEvent>();
      for (const evt of await queryChunked(nostr, eIds, signal)) {
        contentMap.set(evt.id, evt);
      }

      // Enrich zapper profiles (kind 0) for name/picture
      const zapperPks = [...new Set(parsedZaps.map((z) => z.zapper.pubkey))];
      const profileMap = new Map<string, Record<string, unknown>>();
      const profileEvents = await nostr.query(
        [{ kinds: [0], authors: zapperPks.slice(0, 500), limit: 500 }],
        { signal },
      ).catch(() => [] as NostrEvent[]);
      for (const evt of profileEvents) {
        try { profileMap.set(evt.pubkey, JSON.parse(evt.content)); } catch { /* skip */ }
      }

      for (const zap of parsedZaps) {
        if (zap.zappedEvent && contentMap.has(zap.zappedEvent.id)) {
          const evt = contentMap.get(zap.zappedEvent.id)!;
          zap.zappedEvent = {
            ...zap.zappedEvent,
            kind: evt.kind,
            author: evt.pubkey,
            content: evt.content,
            created_at: evt.created_at,
          };
        }
        const profile = profileMap.get(zap.zapper.pubkey);
        if (profile) {
          zap.zapper = {
            ...zap.zapper,
            name: (profile.name as string) || (profile.display_name as string),
            nip05: profile.nip05 as string,
            picture: profile.picture as string,
          };
        }
      }

      // Per-member grouping by the receipt's `p` tag (recipient)
      const byMember = new Map<string, ParsedZap[]>();
      for (const zap of parsedZaps) {
        const recipient = zap.receipt.tags.find((t) => t[0] === 'p')?.[1]?.toLowerCase();
        if (!recipient || !memberSet.has(recipient)) continue;
        const arr = byMember.get(recipient) || [];
        arr.push(zap);
        byMember.set(recipient, arr);
      }

      const memberStats: MemberStats[] = [...byMember.entries()].map(([pubkey, zaps]) => {
        const top = groupZapsByContent(zaps)[0];
        return {
          pubkey,
          name: nameByPubkey.get(pubkey) || '',
          totalEarnings: zaps.reduce((s, z) => s + z.amount, 0),
          totalZaps: zaps.length,
          uniqueZappers: new Set(zaps.map((z) => z.zapper.pubkey)).size,
          topContentSats: top?.totalSats || 0,
          topContentPreview: (top?.content || '').slice(0, 100),
          earningsByPeriod: groupZapsByPeriod(zaps, tr, custom),
          topZappers: getTopZappers(zaps).slice(0, 5),
        };
      }).sort((a, b) => b.totalEarnings - a.totalEarnings);

      const communityZaps = parsedZaps.filter((zap) => {
        const recipient = zap.receipt.tags.find((t) => t[0] === 'p')?.[1]?.toLowerCase();
        return !!recipient && memberSet.has(recipient);
      });

      const aggregate: AnalyticsData = {
        totalEarnings: communityZaps.reduce((s, z) => s + z.amount, 0),
        totalZaps: communityZaps.length,
        uniqueZappers: new Set(communityZaps.map((z) => z.zapper.pubkey)).size,
        period: 'community',
        earningsByPeriod: groupZapsByPeriod(communityZaps, tr, custom),
        topContent: groupZapsByContent(communityZaps).slice(0, 5),
        earningsByKind: groupZapsByKind(communityZaps),
        topZappers: getTopZappers(communityZaps).slice(0, 5),
        allZaps: [],
        temporalPatterns: {
          earningsByHour: groupZapsByHour(communityZaps),
          earningsByDayOfWeek: groupZapsByDayOfWeek(communityZaps),
        },
        zapperLoyalty: analyzeZapperLoyalty(communityZaps),
        contentPerformance: [],
        hashtagPerformance: [],
      };

      return { aggregate, members: memberStats, lastUpdated: Date.now() };
    },
    enabled: !!user?.pubkey && memberKey.length > 0,
    staleTime: 60 * 1000,
    retry: 1,
  });

  const refresh = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['community-zap-stats'] });
  }, [queryClient]);

  return { data, isLoading, error, refetch: refresh };
}

function emptyAnalytics(): AnalyticsData {
  return {
    totalEarnings: 0,
    totalZaps: 0,
    uniqueZappers: 0,
    period: 'community',
    earningsByPeriod: [],
    topContent: [],
    earningsByKind: [],
    topZappers: [],
    allZaps: [],
    temporalPatterns: { earningsByHour: [], earningsByDayOfWeek: [] },
    zapperLoyalty: {
      newZappers: 0,
      returningZappers: 0,
      regularSupporters: 0,
      averageLifetimeValue: 0,
      topLoyalZappers: [],
    },
    contentPerformance: [],
    hashtagPerformance: [],
  };
}
