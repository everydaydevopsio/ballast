import path from 'path';
import {
  getContent,
  getTemplate,
  buildCursorFormat,
  buildClaudeFormat,
  buildOpenCodeFormat,
  buildContent,
  getDestination,
  listTargets
} from './build';

describe('build', () => {
  describe('getContent', () => {
    test('returns content for linting agent', () => {
      const content = getContent('linting');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Your Responsibilities');
    });

    test('throws for unknown agent', () => {
      expect(() => getContent('nonexistent')).toThrow(/content.md/);
    });
  });

  describe('getTemplate', () => {
    test('reads cursor frontmatter for linting', () => {
      const t = getTemplate('linting', 'cursor-frontmatter.yaml');
      expect(t).toContain('alwaysApply: false');
      expect(t).toContain('globs:');
    });

    test('reads claude header for linting', () => {
      const t = getTemplate('linting', 'claude-header.md');
      expect(t).toContain('TypeScript Linting Rules');
    });

    test('reads opencode frontmatter for linting', () => {
      const t = getTemplate('linting', 'opencode-frontmatter.yaml');
      expect(t).toContain('mode: subagent');
    });
  });

  describe('buildCursorFormat', () => {
    test('combines frontmatter with content for linting', () => {
      const result = buildCursorFormat('linting');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('alwaysApply: false');
      expect(result).toContain('## Your Responsibilities');
    });
  });

  describe('buildClaudeFormat', () => {
    test('combines header with content for linting', () => {
      const result = buildClaudeFormat('linting');
      expect(result).toContain('# TypeScript Linting Rules');
      expect(result).toContain('## Your Responsibilities');
      expect(result).not.toContain('mode: subagent');
    });
  });

  describe('buildOpenCodeFormat', () => {
    test('combines frontmatter with content for linting', () => {
      const result = buildOpenCodeFormat('linting');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('mode: subagent');
      expect(result).toContain('## Your Responsibilities');
    });
  });

  describe('buildContent', () => {
    test('cursor returns mdc-style content', () => {
      const result = buildContent('linting', 'cursor');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('globs:');
    });

    test('claude returns header + content', () => {
      const result = buildContent('linting', 'claude');
      expect(result).toContain('# TypeScript Linting Rules');
    });

    test('opencode returns yaml frontmatter + content', () => {
      const result = buildContent('linting', 'opencode');
      expect(result).toContain('permission:');
    });

    test('throws for unknown target', () => {
      expect(() => buildContent('linting', 'unknown' as 'cursor')).toThrow(
        /Unknown target/
      );
    });
  });

  describe('getDestination', () => {
    const projectRoot = path.join(__dirname, '..', 'fixtures', 'project');

    test('cursor returns .cursor/rules/<agent>.mdc', () => {
      const { dir, file } = getDestination('linting', 'cursor', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.cursor', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.cursor', 'rules', 'linting.mdc')
      );
    });

    test('claude returns .claude/rules/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'claude', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.claude', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.claude', 'rules', 'linting.md')
      );
    });

    test('opencode returns .opencode/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'opencode', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.opencode'));
      expect(file).toBe(path.join(projectRoot, '.opencode', 'linting.md'));
    });

    test('throws for unknown target', () => {
      expect(() =>
        getDestination('linting', 'unknown' as 'cursor', projectRoot)
      ).toThrow(/Unknown target/);
    });
  });

  describe('listTargets', () => {
    test('returns cursor, claude, opencode', () => {
      expect(listTargets()).toEqual(['cursor', 'claude', 'opencode']);
    });
  });
});
