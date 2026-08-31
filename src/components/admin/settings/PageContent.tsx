import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeRaw from 'rehype-raw';
import { useAppContext } from '@/hooks/useAppContext';

interface PageContentProps {
  content: string;
  className?: string;
}

/**
 * Renders page content as HTML or Markdown, matching the homepage rendering.
 * If the content starts with '<', it's treated as raw HTML.
 * Otherwise, it's rendered as Markdown with GFM and raw HTML support.
 *
 * `nostr:note1…`, `nostr:nevent1…`, and `nostr:naddr1…` references in the
 * Markdown are rendered as inline preview cards via `NostrEventEmbed`,
 * and `nostr:npub1…` / `nostr:nprofile1…` references are rendered as
 * `@name` mentions via `NostrMention` (see `remarkNostrEmbed`).
 */
export function PageContent({ content, className }: PageContentProps) {
  const { config } = useAppContext();
  const gateway = config.siteConfig?.nip19Gateway || 'https://nostr.at';
  const cleanGateway = gateway.endsWith('/') ? gateway.slice(0, -1) : gateway;

  const isHtml = content.trim().startsWith('<');

  if (isHtml) {
    return <div className={className} dangerouslySetInnerHTML={{ __html: content }} />;
  }

  // Map the custom <nostr-embed> and <nostr-mention> elements (produced by
  // remarkNostrEmbed and parsed by rehype-raw) to their React components.
  // These are not standard HTML elements, so they're not in
  // JSX.IntrinsicElements — we cast the components map to `Components`
  // to satisfy react-markdown's prop type.
  const components = {
    'nostr-embed': ({ 'data-identifier': identifier }: { 'data-identifier'?: string }) =>
      identifier ? <NostrEventEmbed identifier={identifier} gateway={cleanGateway} /> : null,
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
