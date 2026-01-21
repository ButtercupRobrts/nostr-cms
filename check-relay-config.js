#!/usr/bin/env node

// Simple script to check what config event is stored on the relay

import { relayInit } from 'nostr-tools/relay';

const MASTER_PUBKEY = '3878d95db7b854c3a0d3b2d6b7bf9bf28b36162be64326f5521ba71cf3b45a69';
const DEFAULT_RELAY = 'wss://nos.lol'; // From your screenshot

console.log('🔍 Connecting to relay:', DEFAULT_RELAY);
console.log('👤 Master pubkey:', MASTER_PUBKEY);
console.log('');

const relay = relayInit(DEFAULT_RELAY);

relay.on('connect', () => {
  console.log('✅ Connected to relay');
});

relay.on('error', () => {
  console.log('❌ Failed to connect to relay');
});

async function checkConfig() {
  try {
    await relay.connect();

    console.log('📡 Querying for kind 30078 site config events...');

    const sub = relay.sub([
      {
        kinds: [30078],
        authors: [MASTER_PUBKEY],
        '#d': ['nostr-meetup-site-config'],
        limit: 1
      }
    ]);

    sub.on('event', (event) => {
      console.log('\n📦 Found config event:');
      console.log('  Event ID:', event.id);
      console.log('  Created:', new Date(event.created_at * 1000).toLocaleString());
      console.log('  Tags:', JSON.stringify(event.tags, null, 2));
      console.log('  Content:', event.content);

      // Parse the specific values
      const defaultRelay = event.tags.find(([name]) => name === 'default_relay')?.[1];
      const publishRelays = event.tags.find(([name]) => name === 'publish_relays')?.[1];
      const adminRoles = event.tags.find(([name]) => name === 'admin_roles')?.[1];

      console.log('\n📋 Parsed Values:');
      console.log('  Default Relay:', defaultRelay);
      console.log('  Publish Relays:', publishRelays);
      console.log('  Admin Roles:', adminRoles);
    });

    sub.on('eose', () => {
      console.log('\n✅ End of query');
      relay.close();
      process.exit(0);
    });

    // Timeout after 10 seconds
    setTimeout(() => {
      console.log('\n⏱️  Query timeout');
      relay.close();
      process.exit(0);
    }, 10000);

  } catch (error) {
    console.error('❌ Error:', error.message);
    relay.close();
    process.exit(1);
  }
}

checkConfig();
