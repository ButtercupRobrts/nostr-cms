/**
 * React component that renders Markdown with event embed and mention support.
 *
 * This is a wrapper around ReactMarkdown that uses the remarkNostrEmbed
 * plugin to transform nostr: links into <nostr-embed> and <nostr-mention>
 * elements, which are then rendered as NostrEventEmbed and NostrMention
 * components respectively.
 */

import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeRaw from 'rehype-raw';
import { remarkNostrEmbed } from './remarkNostrEmbed';
import { NostrEventEmbed } from '@/components/NostrEventEmbed';
import { NostrMention } from '@/components/NostrMention';
import { useAppContext } from '@/hooks/useAppContext';
import { type Components } from 'react-markdown';

interface MarkdownWithEventEmbedsProps {
  content: string;
  className?: string;
}

export function MarkdownWithEventEmbeds({ content, className }: MarkdownWithEventEmbedsProps) {
  const { config } = useAppContext();
  const gateway = config.siteConfig?.nip19Gateway || 'https://nostr.at';
  const cleanGateway = gateway.endsWith('/') ? gateway.slice(0, -1) : gateway;

  // Map the custom <nostr-embed> and <nostr-mention> elements (produced by
  // remarkNostrEmbed and parsed by rehype-raw) to their React components.
  const components = {
    'nostr-embed': ({ 'data-identifier': identifier }: { 'data-identifier'?: string }) =>
      identifier ? <NostrEventEmbed identifier={identifier} gateway={cleanGateway} truncate={false} /> : null,
    'nostr-mention': ({ 'data-identifier': identifier }: { 'data-identifier'?: string }) =>
      identifier ? <NostrMention identifier={identifier} gateway={cleanGateway} /> : null,
    img: (props: React.ComponentProps<'img'>) => (
      <img
        {...props}
        className="rounded-lg max-w-full h-auto"
        loading="lazy"
      />
    ),
  } as unknown as Components;

  return (
    <div className={className}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkNostrEmbed]}
        rehypePlugins={[rehypeRaw]}
        components={components}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}