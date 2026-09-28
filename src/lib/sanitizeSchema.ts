/**
 * Shared rehype-sanitize schema for rendering Markdown with raw HTML support.
 *
 * Extends the default GitHub-style schema to allow the custom `<nostr-embed>`
 * element (produced by remarkNostrEmbed) with its `data-identifier` attribute.
 *
 * Used by MarkdownWithEventEmbeds and PageContent to sanitize untrusted HTML
 * in Markdown content as a defense-in-depth layer, even though CMS content is
 * gated to master/publisher authors.
 */

import { defaultSchema } from 'rehype-sanitize';
import type { Schema } from 'hast-util-sanitize';

export const sanitizeSchema: Schema = {
  ...defaultSchema,
  // Allow the custom <nostr-embed> element produced by remarkNostrEmbed
  tagNames: [...(defaultSchema.tagNames ?? []), 'nostr-embed'],
  attributes: {
    ...defaultSchema.attributes,
    'nostr-embed': ['data-identifier'],
  },
};
