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
