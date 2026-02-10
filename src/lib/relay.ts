/**
 * Swarm runtime config utilities.
 *
 * The Go server injects window.__SWARM_CONFIG__ into index.html at serve time,
 * providing runtime values (masterPubkey, relayName) without requiring build-time
 * Vite environment variables. Build-time VITE_* vars still work as overrides.
 */

interface SwarmConfig {
  masterPubkey?: string;
  relayName?: string;
}

/** Read the server-injected config (set by the Go server in index.html) */
function getSwarmConfig(): SwarmConfig {
  if (typeof window !== 'undefined' && '__SWARM_CONFIG__' in window) {
    return (window as unknown as Record<string, unknown>).__SWARM_CONFIG__ as SwarmConfig;
  }
  return {};
}

/**
 * Get the master/admin pubkey.
 * Priority: VITE_MASTER_PUBKEY env var → server-injected __SWARM_CONFIG__
 */
export function getMasterPubkey(): string {
  const envPubkey = import.meta.env.VITE_MASTER_PUBKEY;
  if (envPubkey) return envPubkey.toLowerCase().trim();

  const injected = getSwarmConfig().masterPubkey;
  if (injected) return injected.toLowerCase().trim();

  return '';
}

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
