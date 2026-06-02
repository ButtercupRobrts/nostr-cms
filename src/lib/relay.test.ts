import { describe, expect, it } from 'vitest';
import { getApiBaseUrl, getDefaultRelayUrl, getSwarmAdminApiUrl, isUnifiedSetup } from './relay';

describe('relay helpers', () => {
  it('builds admin API URL from API base', () => {
    const apiBase = getApiBaseUrl().replace(/\/$/, '');
    expect(getSwarmAdminApiUrl()).toBe(`${apiBase}/admin`);
  });

  it('returns relay URL using ws or wss scheme', () => {
    const relay = getDefaultRelayUrl();
    expect(relay.startsWith('ws://') || relay.startsWith('wss://')).toBe(true);
  });

  it('returns API URL using relative or http(s) scheme', () => {
    const apiBase = getApiBaseUrl();
    expect(apiBase === '/api' || apiBase.startsWith('http://') || apiBase.startsWith('https://')).toBe(true);
  });

  it('matches unified detection to host comparison logic', () => {
    const relayHost = new URL(getDefaultRelayUrl().replace(/^wss:\/\//, 'https://').replace(/^ws:\/\//, 'http://')).host;
    const apiBase = getApiBaseUrl();
    const apiHost = new URL(apiBase, window.location.origin).host;

    expect(isUnifiedSetup()).toBe(relayHost === window.location.host && apiHost === window.location.host);
  });
});
