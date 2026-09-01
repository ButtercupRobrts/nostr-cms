import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface PageContentProps {
  content: string;
  className?: string;
}

/**
 * Renders page content as HTML or Markdown, matching the homepage rendering.
 * If the content starts with '<', it's treated as raw HTML.
 * Otherwise, it's rendered as Markdown with GFM (raw HTML is stripped for security).
 */
export function PageContent({ content, className }: PageContentProps) {
  const isHtml = content.trim().startsWith('<');

  if (isHtml) {
    return <div className={className} dangerouslySetInnerHTML={{ __html: content }} />;
  }

  return (
    <div className={className}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
