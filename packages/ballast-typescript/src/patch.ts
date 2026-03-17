import YAML from 'yaml';
import type { Target } from './config';

interface ParsedFrontmatterDocument {
  frontmatter: string | null;
  body: string;
}

interface MarkdownSection {
  heading: string;
  text: string;
}

interface ParsedMarkdownBody {
  intro: string;
  sections: MarkdownSection[];
}

const DANGEROUS_YAML_KEYS = new Set(['__proto__', 'constructor', 'prototype']);

function splitFrontmatterDocument(content: string): ParsedFrontmatterDocument {
  const match = content.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/);
  if (!match || match.index !== 0) {
    return { frontmatter: null, body: content.trimStart() };
  }

  return {
    frontmatter: match[0].trimEnd(),
    body: content.slice(match[0].length).trimStart()
  };
}

function getYamlContent(frontmatter: string): string | null {
  const match = frontmatter.match(/^---\r?\n([\s\S]*?)\r?\n---$/);
  return match ? match[1] : null;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function createSafeRecord(): Record<string, unknown> {
  return Object.create(null) as Record<string, unknown>;
}

function mergeYamlValues(
  canonical: Record<string, unknown>,
  existing: Record<string, unknown>
): Record<string, unknown> {
  const merged = createSafeRecord();

  for (const [key, canonicalValue] of Object.entries(canonical)) {
    if (DANGEROUS_YAML_KEYS.has(key)) {
      continue;
    }
    merged[key] = canonicalValue;
  }

  for (const [key, existingValue] of Object.entries(existing)) {
    if (DANGEROUS_YAML_KEYS.has(key)) {
      continue;
    }
    const canonicalValue = merged[key];
    merged[key] =
      isPlainObject(canonicalValue) && isPlainObject(existingValue)
        ? mergeYamlValues(canonicalValue, existingValue)
        : existingValue;
  }
  return merged;
}

function mergeFrontmatter(
  existingFrontmatter: string | null,
  canonicalFrontmatter: string | null
): string | null {
  if (!canonicalFrontmatter) return existingFrontmatter;
  if (!existingFrontmatter) return canonicalFrontmatter;

  const existingYaml = getYamlContent(existingFrontmatter);
  const canonicalYaml = getYamlContent(canonicalFrontmatter);
  if (!existingYaml || !canonicalYaml) {
    return existingFrontmatter;
  }

  try {
    const existingParsed = YAML.parse(existingYaml);
    const canonicalParsed = YAML.parse(canonicalYaml);
    if (!isPlainObject(existingParsed) || !isPlainObject(canonicalParsed)) {
      return existingFrontmatter;
    }

    const merged = mergeYamlValues(canonicalParsed, existingParsed);
    return `---\n${YAML.stringify(merged).trimEnd()}\n---`;
  } catch {
    return existingFrontmatter;
  }
}

function parseMarkdownBody(content: string): ParsedMarkdownBody {
  const headingRegex = /^## .*(?:\r?\n|$)/gm;
  const matches = [...content.matchAll(headingRegex)];

  if (matches.length === 0) {
    return { intro: content.trim(), sections: [] };
  }

  const firstMatch = matches[0];
  const firstIndex = firstMatch.index ?? 0;
  const intro = content.slice(0, firstIndex).trim();
  const sections: MarkdownSection[] = [];

  for (let i = 0; i < matches.length; i++) {
    const start = matches[i].index ?? 0;
    const end =
      i + 1 < matches.length
        ? (matches[i + 1].index ?? content.length)
        : content.length;
    const text = content.slice(start, end).trim();
    const heading = text.split(/\r?\n/, 1)[0];
    sections.push({ heading, text });
  }

  return { intro, sections };
}

function mergeMarkdownBodies(existing: string, canonical: string): string {
  if (!existing.trim()) {
    return canonical.trimEnd() + '\n';
  }

  const existingParsed = parseMarkdownBody(existing);
  const canonicalParsed = parseMarkdownBody(canonical);
  const canonicalHeadings = new Set(
    canonicalParsed.sections.map((section) => section.heading)
  );
  const existingByHeading = new Map(
    existingParsed.sections.map((section) => [section.heading, section.text])
  );

  const parts: string[] = [];
  const intro = existingParsed.intro || canonicalParsed.intro;
  if (intro) {
    parts.push(intro);
  }

  for (const section of canonicalParsed.sections) {
    parts.push(existingByHeading.get(section.heading) ?? section.text);
  }

  for (const section of existingParsed.sections) {
    if (!canonicalHeadings.has(section.heading)) {
      parts.push(section.text);
    }
  }

  return parts.join('\n\n').trimEnd() + '\n';
}

export function patchRuleContent(
  existing: string,
  canonical: string,
  target: Target
): string {
  if (!existing.trim()) {
    return canonical;
  }

  switch (target) {
    case 'cursor':
    case 'opencode': {
      const existingDoc = splitFrontmatterDocument(existing);
      const canonicalDoc = splitFrontmatterDocument(canonical);
      const frontmatter = mergeFrontmatter(
        existingDoc.frontmatter,
        canonicalDoc.frontmatter
      );
      const body = mergeMarkdownBodies(existingDoc.body, canonicalDoc.body);
      return frontmatter ? `${frontmatter}\n\n${body}` : body;
    }
    case 'claude':
    case 'codex':
      return mergeMarkdownBodies(existing, canonical);
    default:
      return canonical;
  }
}

function findMarkdownSectionRange(
  content: string,
  heading: string
): { start: number; end: number } | null {
  const headingRegex = new RegExp(
    `^## ${heading.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`,
    'm'
  );
  const headingMatch = headingRegex.exec(content);
  if (!headingMatch || headingMatch.index == null) {
    return null;
  }

  const afterHeading = content.slice(
    headingMatch.index + headingMatch[0].length
  );
  const nextHeadingMatch = /\n## .*/.exec(afterHeading);
  const end = nextHeadingMatch
    ? headingMatch.index + headingMatch[0].length + nextHeadingMatch.index + 1
    : content.length;

  return { start: headingMatch.index, end };
}

export function patchCodexAgentsMd(
  existing: string,
  canonical: string
): string {
  if (!existing.trim()) {
    return canonical;
  }

  const canonicalRange = findMarkdownSectionRange(
    canonical,
    'Installed agent rules'
  );
  if (!canonicalRange) {
    return existing;
  }

  const canonicalSection = canonical
    .slice(canonicalRange.start, canonicalRange.end)
    .trimEnd();
  const existingRange = findMarkdownSectionRange(
    existing,
    'Installed agent rules'
  );

  if (!existingRange) {
    return `${existing.trimEnd()}\n\n${canonicalSection}\n`;
  }

  return (
    (
      `${existing.slice(0, existingRange.start).trimEnd()}\n\n` +
      `${canonicalSection}\n\n` +
      `${existing.slice(existingRange.end).trimStart()}`
    ).trimEnd() + '\n'
  );
}
