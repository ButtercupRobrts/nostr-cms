/**
 * Relay URL utilities for Swarm integration.
 * 
 * When VITE_DEFAULT_RELAY is set (build-time), that value is used.
 * When unset, the relay URL is auto-derived from the current browser domain,
 * enabling zero-config same-domain deployments.
 */

/**
 * Get the default relay WebSocket URL.
 * Priority: VITE_DEFAULT_RELAY env var → auto-derive from window.location
 */
export function getDefaultRelayUrl(): string {
  const envRelay = import.meta.env.VITE_DEFAULT_RELAY;
  if (envRelay) return envRelay;

  // Auto-derive from current domain (same-domain Swarm deployment)
  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}`;
  }

  // SSR/build-time fallback
  return 'ws://localhost:3334';
}

/**
 * Derive the HTTP(S) API base URL from the relay URL.
 * Converts wss:// → https:// and ws:// → http://, then appends /api.
 * Falls back to /api for same-domain deployments when no relay env var is set.
 */
export function getApiBaseUrl(): string {
  const envSwarmApi = import.meta.env.VITE_SWARM_API_URL;
  if (envSwarmApi) return envSwarmApi;

  // If no explicit relay env var, use relative /api (same-domain)
  if (!import.meta.env.VITE_DEFAULT_RELAY) return '/api';

  // Derive from explicit relay URL
  const relay = getDefaultRelayUrl();
  const httpUrl = relay
    .replace(/\/$/, '')
    .replace(/^wss:\/\//, 'https://')
    .replace(/^ws:\/\//, 'http://');
  return `${httpUrl}/api`;
}
