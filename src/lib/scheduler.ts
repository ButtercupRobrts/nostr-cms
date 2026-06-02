/**
 * Scheduler Configuration helpers.
 */

import { getApiBaseUrl } from '@/lib/relay';

/**
 * Get the Scheduler API base URL
 */
export function getSchedulerApiUrl(): string {
  return getApiBaseUrl();
}

/**
 * Get the Swarm admin API base URL.
 * Used for migrating relay dashboard functionality into nostr-cms.
 */
export function getSwarmAdminApiUrl(): string {
  return `${getApiBaseUrl()}/admin`;
}
