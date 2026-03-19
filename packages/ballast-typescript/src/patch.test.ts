import { patchCodexAgentsMd, patchRuleContent } from './patch';

describe('patch', () => {
  describe('patchRuleContent', () => {
    test('preserves existing markdown sections and appends new canonical sections', () => {
      const existing = `# TypeScript Linting Rules

Custom intro for this repo.

## Your Responsibilities

User customized responsibilities.

## Team Overrides

Keep this section.
`;

      const canonical = `# TypeScript Linting Rules

Canonical intro.

## Your Responsibilities

Canonical responsibilities.

## When Completed

Canonical completion checklist.
`;

      const merged = patchRuleContent(existing, canonical, 'claude');
      expect(merged).toContain('Custom intro for this repo.');
      expect(merged).toContain('User customized responsibilities.');
      expect(merged).toContain('## When Completed');
      expect(merged).toContain('Canonical completion checklist.');
      expect(merged).toContain('## Team Overrides');
      expect(merged).toContain('Keep this section.');
    });

    test('merges frontmatter with user values taking precedence', () => {
      const existing = `---
description: User description
alwaysApply: true
tools:
  read: false
---

Intro.

## Existing

User content.
`;

      const canonical = `---
description: Canonical description
alwaysApply: false
globs:
  - '*.ts'
tools:
  read: true
  write: true
---

Intro.

## Existing

Canonical content.
`;

      const merged = patchRuleContent(existing, canonical, 'cursor');
      expect(merged).toContain('description: User description');
      expect(merged).toContain('alwaysApply: true');
      expect(merged).toContain('globs:');
      expect(merged).toContain('*.ts');
      expect(merged).toContain('  read: false');
      expect(merged).toContain('  write: true');
      expect(merged).toContain('User content.');
    });

    test('ignores dangerous yaml keys while merging frontmatter', () => {
      const existing = `---
description: User description
__proto__:
  polluted: yes
tools:
  read: false
---

## Existing

User content.
`;

      const canonical = `---
description: Canonical description
tools:
  read: true
  write: true
---

## Existing

Canonical content.
`;

      const merged = patchRuleContent(existing, canonical, 'cursor');
      expect(merged).toContain('description: User description');
      expect(merged).not.toContain('__proto__');
      expect(merged).toContain('  read: false');
      expect(merged).toContain('  write: true');
      expect(
        Object.prototype.hasOwnProperty.call(Object.prototype, 'polluted')
      ).toBe(false);
      expect(({} as { polluted?: string }).polluted).toBeUndefined();
    });

    test('falls back to canonical content when existing file is empty', () => {
      const canonical = `# Rule

## Section

Canonical content.
`;

      expect(patchRuleContent('', canonical, 'codex')).toBe(canonical);
    });
  });

  describe('patchCodexAgentsMd', () => {
    test('replaces the installed rules section and preserves user content outside it', () => {
      const existing = `# AGENTS.md

Intro line.

## Team Notes

Keep this note.

## Installed agent rules

Read and follow these rule files in \`.codex/rules/\` when they apply:

- \`.codex/rules/old.md\` — Old rule

## Local Notes

Keep this too.
`;

      const canonical = `# AGENTS.md

This file provides guidance to Codex (CLI and app) for working in this repository.

## Installed agent rules

Created by Ballast v9.9.9-test. Do not edit this section.

Read and follow these rule files in \`.codex/rules/\` when they apply:

- \`.codex/rules/typescript-linting.md\` — Linting rule
`;

      const merged = patchCodexAgentsMd(existing, canonical);
      expect(merged).toContain('## Team Notes');
      expect(merged).toContain('Keep this note.');
      expect(merged).toContain('## Local Notes');
      expect(merged).toContain('Keep this too.');
      expect(merged).toMatch(
        /Created by Ballast v[0-9A-Za-z._-]+\. Do not edit this section\./
      );
      expect(merged).toContain('`.codex/rules/typescript-linting.md`');
      expect(merged).not.toContain('`.codex/rules/old.md`');
    });
  });
});
