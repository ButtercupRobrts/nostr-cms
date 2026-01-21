import { relayInit } from 'nostr-tools/relay';

const RELAY_URL = 'wss://swarm.hivetalk.org';
const MASTER_PUBKEY = '3878d95db7b854c3a0d3b2d6b7bf9bf28b36162be64326f5521ba71cf3b45a69';

async function checkRelay() {
  console.log('Connecting to relay:', RELAY_URL);
  const relay = relayInit(RELAY_URL);

  try {
    await relay.connect();
    console.log('✓ Connected to relay\n');

    console.log('Querying for Kind 30078 events from master pubkey:', MASTER_PUBKEY);
    console.log('Filter:', JSON.stringify({
      kinds: [30078],
      authors: [MASTER_PUBKEY],
      '#d': ['nostr-meetup-site-config']
    }, null, 2));
    console.log('\n---\n');

    const events = await relay.list([
      {
        kinds: [30078],
        authors: [MASTER_PUBKEY],
        '#d': ['nostr-meetup-site-config'],
        limit: 5
      }
    ]);

    if (events.length === 0) {
      console.log('✓ No Kind 30078 config events found on the relay.');
      console.log('The config has been successfully deleted!\n');
    } else {
      console.log(`✗ Found ${events.length} Kind 30078 config event(s):\n`);
      events.forEach((event, index) => {
        console.log(`Event ${index + 1}:`);
        console.log('  ID:', event.id);
        console.log('  Created:', new Date(event.created_at * 1000).toISOString());
        console.log('  Tags:', event.tags.slice(0, 5)); // Show first 5 tags
        console.log('  Content (first 200 chars):', event.content.substring(0, 200));
        console.log('');
      });
    }

    relay.close();
  } catch (error) {
    console.error('Error:', error.message);
    relay.close();
    process.exit(1);
  }
}

checkRelay();
