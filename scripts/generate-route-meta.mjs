import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const distDir = path.resolve(process.cwd(), 'dist');
const indexPath = path.join(distDir, 'index.html');

const SEO_META_START = '<!-- SEO_META_START -->';
const SEO_META_END = '<!-- SEO_META_END -->';

const siteUrl = (process.env.VITE_SITE_URL || '').replace(/\/$/, '');
const defaultOgImage = process.env.VITE_OG_IMAGE || '';

const routes = [
  {
    path: '/',
    title: 'Community Meetup Site',
    description: 'Join us for amazing meetups and events',
  },
  {
    path: '/blog',
    title: 'Blog - Community Meetup',
    description: 'Read our latest blog posts and community updates.',
  },
  {
    path: '/events',
    title: 'Events - Community Meetup',
    description: 'Browse upcoming and past community events and meetups.',
  },
];

function escapeHtml(value) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function toAbsoluteUrl(value) {
  if (!value) return '';
  if (/^https?:\/\//i.test(value)) return value;
  if (!siteUrl) return value;
  const normalizedPath = value.startsWith('/') ? value : `/${value}`;
  return `${siteUrl}${normalizedPath}`;
}

function buildSeoMetaBlock(route) {
  const title = escapeHtml(route.title);
  const description = escapeHtml(route.description);
  const ogUrl = toAbsoluteUrl(route.path);
  const ogImage = toAbsoluteUrl(defaultOgImage);

  const lines = [
    `    <title>${title}</title>`,
    `    <meta name="description" content="${description}" />`,
    `    <meta property="og:title" content="${title}" />`,
    `    <meta property="og:description" content="${description}" />`,
    '    <meta property="og:type" content="website" />',
    '    <meta property="twitter:card" content="summary_large_image" />',
    `    <meta property="twitter:title" content="${title}" />`,
    `    <meta property="twitter:description" content="${description}" />`,
  ];

  if (ogUrl) {
    lines.push(`    <meta property="og:url" content="${escapeHtml(ogUrl)}" />`);
  }

  if (ogImage) {
    lines.push(`    <meta property="og:image" content="${escapeHtml(ogImage)}" />`);
    lines.push(`    <meta property="twitter:image" content="${escapeHtml(ogImage)}" />`);
  }

  return lines.join('\n');
}

function outputPathForRoute(routePath) {
  if (routePath === '/') {
    return path.join(distDir, 'index.html');
  }

  return path.join(distDir, routePath.replace(/^\//, ''), 'index.html');
}

async function generateRouteMetaHtml() {
  const sourceHtml = await readFile(indexPath, 'utf8');

  if (!sourceHtml.includes(SEO_META_START) || !sourceHtml.includes(SEO_META_END)) {
    throw new Error('SEO markers not found in index.html.');
  }

  const seoBlockRegex = /<!-- SEO_META_START -->[\s\S]*?<!-- SEO_META_END -->/;

  for (const route of routes) {
    const routeMetaBlock = `${SEO_META_START}\n${buildSeoMetaBlock(route)}\n    ${SEO_META_END}`;
    const routeHtml = sourceHtml.replace(seoBlockRegex, routeMetaBlock);
    const outputPath = outputPathForRoute(route.path);

    await mkdir(path.dirname(outputPath), { recursive: true });
    await writeFile(outputPath, routeHtml, 'utf8');
    console.log(`[seo] generated ${path.relative(distDir, outputPath)}`);
  }
}

generateRouteMetaHtml().catch((error) => {
  console.error('[seo] failed to generate route metadata files:', error);
  process.exitCode = 1;
});
