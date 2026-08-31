/**
 * Custom remark plugin to replace nostr: links with rich embeds/mentions.
 *
 * This enables blog posts and page content to render:
 *  - `nostr:note1…` / `nostr:nevent1…` / `nostr:naddr1…` → inline preview
 *    cards via `NostrEventEmbed` (as `<nostr-embed>` elements).
 *  - `nostr:npub1…` / `nostr:nprofile1…` → inline `@name` mentions
 *    (as `<nostr-mention>` elements).
 *
 * Both Markdown links (`[text](nostr:npub1…)`) and bare `nostr:` URIs in
 * running text are handled. Text inside code blocks/inline code is left
 * untouched.
 */

import { visit } from 'unist-util-visit';
import { nip19 } from 'nostr-tools';
import type { Root, Link, Text, Html, Parent } from 'mdast';

export function remarkNostrEmbed() {
  return (tree: Root) => {
    // 1. Handle nostr: references that appear as Markdown link nodes
    //    (either autolinked or explicitly written as [text](nostr:…)).
    visit(tree, 'link', (node: Link) => {
      const url = node.url;

      // Check if this is a nostr: reference
      if (url.startsWith('nostr:')) {
        try {
          const identifier = url.replace('nostr:', '');
          const decoded = nip19.decode(identifier);

          const type = decoded.type as string;
          if (type === 'note' || type === 'nevent' || type === 'naddr') {
            // Replace the link node with a custom HTML element
            const htmlNode = node as unknown as Html;
            htmlNode.type = 'html';
            htmlNode.value = `<nostr-embed data-identifier="${identifier}"></nostr-embed>`;
          } else if (type === 'npub' || type === 'nprofile') {
            const htmlNode = node as unknown as Html;
            htmlNode.type = 'html';
            htmlNode.value = `<nostr-mention data-identifier="${identifier}"></nostr-mention>`;
          }
        } catch {
          // If decoding fails, leave the link as-is
        }
      }
    });

    // 2. Handle bare nostr:npub1… / nostr:nprofile1… in running text.
    //    remark-gfm only autolinks http/https/www, so bare nostr: URIs
    //    remain as text nodes. We split them into text + <nostr-mention>
    //    HTML nodes so rehype-raw can pick them up.
    visit(tree, 'text', (node: Text, index: number | undefined, parent: Parent | undefined) => {
      if (index === undefined || index === null || !parent) return;
      // Skip text inside code blocks or inline code
      if (parent.type === 'code' || parent.type === 'inlineCode') return;
      // Skip text that is a child of a link — the link visitor above
      // handles the URL, and the link's text children are the label.
      if (parent.type === 'link') return;

      const value: string = node.value;
      const re = /nostr:(npub1|nprofile1)([023456789acdefghjklmnpqrstuvwxyz]+)/gi;

      // Fast path: no mention in this text node.
      re.lastIndex = 0;
      if (!re.exec(value)) return;

      const newNodes: (Text | Html)[] = [];
      let lastIndex = 0;
      let match: RegExpExecArray | null;
      re.lastIndex = 0;

      while ((match = re.exec(value)) !== null) {
        // Preserve text before the mention
        if (match.index > lastIndex) {
          newNodes.push({ type: 'text', value: value.slice(lastIndex, match.index) });
        }
        const identifier = match[1] + match[2];
        newNodes.push({
          type: 'html',
          value: `<nostr-mention data-identifier="${identifier}"></nostr-mention>`,
        });
        lastIndex = match.index + match[0].length;
      }

      // Preserve trailing text
      if (lastIndex < value.length) {
        newNodes.push({ type: 'text', value: value.slice(lastIndex) });
      }

      if (newNodes.length > 0) {
        // Replace the single text node with the split nodes.
        // unist-util-visit v5 handles sibling-index adjustment
        // automatically when we splice the parent's children.
        parent.children.splice(index, 1, ...newNodes);
        // Skip past the newly inserted nodes to avoid revisiting them.
        return index + newNodes.length;
      }
    });
  };
}
