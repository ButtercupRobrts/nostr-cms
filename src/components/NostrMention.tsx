import { nip19 } from 'nostr-tools';
import { useAuthor } from '@/hooks/useAuthor';
import { genUserName } from '@/lib/genUserName';
import { cn } from '@/lib/utils';

interface NostrMentionProps {
  /** The bech32 identifier (npub1... or nprofile1...) without the nostr: prefix. */
  identifier: string;
  /** Base gateway URL for the external link, e.g. https://nostr.at */
  gateway: string;
}

/**
 * Renders a `nostr:npub1…` / `nostr:nprofile1…` reference as an inline
 * `@name` link. Fetches the kind 0 profile for a human-readable display
 * name, falling back to a deterministic generated name.
 *
 * Used by both the notes read path (`NoteContent`) and the blog post
 * markdown read path (`MarkdownWithEventEmbeds` via `remarkNostrEmbed`).
 */
export function NostrMention({ identifier, gateway }: NostrMentionProps) {
  let pubkey: string | null = null;
  try {
    const decoded = nip19.decode(identifier);
    if (decoded.type === 'npub') {
      pubkey = decoded.data as string;
    } else if (decoded.type === 'nprofile') {
      pubkey = (decoded.data as { pubkey: string }).pubkey;
    }
  } catch {
    // malformed bech32 — fall through to raw text
  }

  if (pubkey === null) {
    return <>{identifier}</>;
  }

  return <NostrMentionInner pubkey={pubkey} href={`${gateway}/${identifier}`} />;
}

function NostrMentionInner({ pubkey, href }: { pubkey: string; href: string }) {
  const { data: author } = useAuthor(pubkey);
  const hasRealName = !!author?.metadata?.name;
  const displayName = author?.metadata?.name ?? genUserName(pubkey);

  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        'font-medium hover:underline',
        hasRealName ? 'text-blue-500' : 'text-gray-500 hover:text-gray-700',
      )}
    >
      @{displayName}
    </a>
  );
}
