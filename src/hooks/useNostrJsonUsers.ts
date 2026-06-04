import { useQuery } from '@tanstack/react-query';
import { getSwarmAdminApiUrl, isUnifiedSetup } from '@/lib/relay';

interface RelayUsersResponse {
  users: Record<string, string>;
  isRemote: boolean;
  npubDomain?: string;
}

interface NostrJsonUser {
  name: string;
  pubkey: string;
}

interface NostrJsonResponse {
  names: Record<string, string>;
  relays?: Record<string, string[]>;
  nip46?: Record<string, string[]>;
}

const DEFAULT_NOSTR_JSON_URL = import.meta.env.VITE_REMOTE_NOSTR_JSON_URL || '/.well-known/nostr.json';

async function parseError(response: Response): Promise<string> {
  const text = await response.text();
  return text || `${response.status} ${response.statusText}`;
}

/**
 * Unified hook to fetch nostr.json users.
 *
 * In unified setup (CMS and Swarm on same domain):
 * - Fetches from Swarm Admin API (/api/admin/users)
 * - Provides full CRUD capability via the API
 *
 * In separate setup (CMS and Swarm on different domains):
 * - Fetches from VITE_REMOTE_NOSTR_JSON_URL or /.well-known/nostr.json
 * - Read-only access; relay access managed externally
 */
export function useNostrJsonUsers() {
  const unified = isUnifiedSetup();
  const adminApiBase = getSwarmAdminApiUrl();

  return useQuery({
    queryKey: ['nostr-json-users', unified, adminApiBase, DEFAULT_NOSTR_JSON_URL],
    queryFn: async (): Promise<{ users: NostrJsonUser[]; isRemote: boolean; source: 'swarm-api' | 'remote-json' }> => {
      if (unified) {
        // Unified mode: fetch from Swarm Admin API
        const response = await fetch(`${adminApiBase}/users`, {
          method: 'GET',
          credentials: 'include',
        });

        if (!response.ok) {
          throw new Error(await parseError(response));
        }

        const data: RelayUsersResponse = await response.json();
        const users = Object.entries(data.users || {}).map(([name, pubkey]) => ({
          name,
          pubkey: pubkey.toLowerCase().trim(),
        }));

        return {
          users,
          isRemote: data.isRemote,
          source: 'swarm-api',
        };
      } else {
        // Separate mode: fetch from remote nostr.json URL
        const response = await fetch(DEFAULT_NOSTR_JSON_URL);

        if (!response.ok) {
          throw new Error('Failed to fetch nostr.json');
        }

        const data: NostrJsonResponse = await response.json();
        const users = Object.entries(data.names || {}).map(([name, pubkey]) => ({
          name,
          pubkey: pubkey.toLowerCase().trim(),
        }));

        return {
          users,
          isRemote: false,
          source: 'remote-json',
        };
      }
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
    enabled: unified || !!DEFAULT_NOSTR_JSON_URL,
  });
}

/**
 * Helper to dedupe users by pubkey, preferring the '_' (root) alias.
 */
export function dedupeUsersByPubkey(users: NostrJsonUser[]): NostrJsonUser[] {
  const byPubkey = new Map<string, NostrJsonUser>();

  for (const entry of users) {
    const normalizedPubkey = entry.pubkey.toLowerCase().trim();
    const normalizedEntry = {
      name: entry.name,
      pubkey: normalizedPubkey,
    };

    const existing = byPubkey.get(normalizedPubkey);
    if (!existing) {
      byPubkey.set(normalizedPubkey, normalizedEntry);
      continue;
    }

    const existingIsRootAlias = existing.name.trim() === '_';
    const incomingIsRootAlias = normalizedEntry.name.trim() === '_';
    if (!existingIsRootAlias && incomingIsRootAlias) {
      byPubkey.set(normalizedPubkey, normalizedEntry);
    }
  }

  return Array.from(byPubkey.values());
}
