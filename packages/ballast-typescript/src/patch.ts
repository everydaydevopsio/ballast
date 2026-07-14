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

interface MarkdownSectionRange {
  start: number;
  end: number;
}

interface PatchCodexAgentsMdOptions {
  replaceUnmanagedSections?: boolean;
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
    case 'gemini':
      return mergeMarkdownBodies(existing, canonical);
    default:
      return canonical;
  }
}

function findMarkdownSectionRanges(
  content: string,
  heading: string
): MarkdownSectionRange[] {
  const lines = content.split(/\r?\n/);
  const targetHeading = `## ${heading}`;
  let inFence = false;
  let offset = 0;
  const headings: Array<{ line: string; start: number }> = [];

  for (const line of lines) {
    if (line.startsWith('```')) {
      inFence = !inFence;
    }
    if (!inFence && line.startsWith('## ')) {
      headings.push({ line, start: offset });
    }
    offset += line.length + 1;
  }

  return headings
    .map((item, index) => ({
      heading: item.line,
      start: item.start,
      end:
        index + 1 < headings.length ? headings[index + 1].start : content.length
    }))
    .filter((range) => range.heading === targetHeading)
    .map(({ start, end }) => ({ start, end }));
}

function findMarkdownSectionRange(
  content: string,
  heading: string
): MarkdownSectionRange | null {
  return findMarkdownSectionRanges(content, heading)[0] ?? null;
}

function hasBallastManagedNotice(section: string): boolean {
  return (
    section.includes('Created by [Ballast]') &&
    section.includes('Do not edit this section.')
  );
}

export function patchCodexAgentsMd(
  existing: string,
  canonical: string,
  options: PatchCodexAgentsMdOptions = {}
): string {
  if (!existing.trim()) {
    return canonical;
  }

  const replaceUnmanagedSections = options.replaceUnmanagedSections ?? true;
  let next = existing;
  for (const heading of ['Installed agent rules', 'Installed skills']) {
    const canonicalRange = findMarkdownSectionRange(canonical, heading);
    if (!canonicalRange) {
      continue;
    }

    const canonicalSection = canonical
      .slice(canonicalRange.start, canonicalRange.end)
      .trimEnd();
    const existingRange =
      findMarkdownSectionRanges(next, heading).find((range) => {
        if (replaceUnmanagedSections) {
          return true;
        }
        return hasBallastManagedNotice(next.slice(range.start, range.end));
      }) ?? null;

    if (!existingRange) {
      next = `${next.trimEnd()}\n\n${canonicalSection}\n`;
      continue;
    }

    next =
      (
        `${next.slice(0, existingRange.start).trimEnd()}\n\n` +
        `${canonicalSection}\n\n` +
        `${next.slice(existingRange.end).trimStart()}`
      ).trimEnd() + '\n';
  }

  return next;
}
