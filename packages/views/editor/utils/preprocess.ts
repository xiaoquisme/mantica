import { preprocessLinks } from "@multica/ui/markdown";
import { preprocessMentionShortcodes } from "@multica/ui/markdown";
import { isFileCardUrl } from "../extensions/file-card";

/**
 * Preprocess a markdown string before loading into Tiptap via contentType: 'markdown'.
 *
 * This is the ONLY transform applied before @tiptap/markdown parses the content.
 * It does NOT convert to HTML — that was the old markdownToHtml.ts pipeline which
 * was deleted in the April 2026 refactor.
 *
 * Three string→string transforms on raw Markdown:
 * 1. Legacy mention shortcodes [@ id="..." label="..."] → [@Label](mention://member/id)
 *    (old serialization format in database, migrated on read)
 * 2. Raw URLs → markdown links via linkify-it (so they render as clickable Link nodes)
 * 3. CDN file links on their own line → HTML div for fileCard node parsing
 */
export function preprocessMarkdown(markdown: string): string {
  if (!markdown) return "";
  const step1 = preprocessMentionShortcodes(markdown);
  const step2 = preprocessLinks(step1);
  const step3 = preprocessFileCards(step2);
  const step4 = stripUnknownHtmlTags(step3);
  return step4;
}

/**
 * Convert standalone `[name](cdnUrl)` lines into HTML that Tiptap's fileCard
 * parseHTML can recognise. Only matches non-image CDN URLs on their own line.
 *
 * Input:  `[report.pdf](https://multica-static.copilothub.ai/xxx.pdf)`
 * Output: `<div data-type="fileCard" data-href="url" data-filename="report.pdf"></div>`
 */
const FILE_LINK_LINE = /^\[([^\]]+)\]\((https?:\/\/[^)]+)\)$/;

function preprocessFileCards(markdown: string): string {
  return markdown
    .split("\n")
    .map((line) => {
      const trimmed = line.trim();
      const match = trimmed.match(FILE_LINK_LINE);
      if (!match) return line;
      const filename = match[1]!;
      const url = match[2]!;
      if (!isFileCardUrl(url)) return line;
      return `<div data-type="fileCard" data-href="${url}" data-filename="${filename}"></div>`;
    })
    .join("\n");
}

/**
 * Strip non-standard HTML/XML tags that agents may include in their output
 * (e.g. <concise>, <thinking>, <artifact>). These cause React warnings
 * when rehype-raw passes them through as DOM elements.
 *
 * Allows standard HTML tags, mentions, and data-type divs through.
 */
const STANDARD_HTML_TAGS = new Set([
  "a", "abbr", "address", "article", "aside", "audio", "b", "bdi", "bdo",
  "blockquote", "br", "button", "canvas", "caption", "cite", "code", "col",
  "colgroup", "data", "dd", "del", "details", "dfn", "dialog", "div", "dl",
  "dt", "em", "fieldset", "figcaption", "figure", "footer", "form", "h1",
  "h2", "h3", "h4", "h5", "h6", "header", "hr", "i", "iframe", "img",
  "input", "ins", "kbd", "label", "legend", "li", "main", "mark", "menu",
  "meter", "nav", "ol", "optgroup", "option", "output", "p", "picture",
  "pre", "progress", "q", "rp", "rt", "ruby", "s", "samp", "section",
  "select", "small", "source", "span", "strong", "sub", "summary", "sup",
  "svg", "table", "tbody", "td", "template", "textarea", "tfoot", "th",
  "thead", "time", "tr", "track", "u", "ul", "var", "video", "wbr",
]);

function stripUnknownHtmlTags(markdown: string): string {
  // Match opening and closing HTML tags: <tagname ...> or </tagname>
  return markdown.replace(/<\/?([a-zA-Z][a-zA-Z0-9_-]*)\b[^>]*\/?>/g, (match, tagName: string) => {
    const lower = tagName.toLowerCase();
    if (STANDARD_HTML_TAGS.has(lower)) return match;
    // Escape angle brackets so it renders as plain text.
    return match.replace(/</g, "&lt;").replace(/>/g, "&gt;");
  });
}
