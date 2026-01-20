const {
  buildOpenCodeFormat,
  buildCursorFormat,
  buildClaudeFormat,
  getContent,
  getTemplate
} = require('./index');

describe('core package', () => {
  describe('getContent', () => {
    test('returns content from src/content.md', () => {
      const content = getContent();
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Your Responsibilities');
    });
  });

  describe('getTemplate', () => {
    test('reads opencode frontmatter template', () => {
      const template = getTemplate('opencode-frontmatter.yaml');
      expect(template).toContain('mode: subagent');
      expect(template).toContain('model: anthropic/claude-sonnet-4-20250514');
    });

    test('reads cursor frontmatter template', () => {
      const template = getTemplate('cursor-frontmatter.yaml');
      expect(template).toContain('alwaysApply: false');
      expect(template).toContain('globs:');
    });

    test('reads claude header template', () => {
      const template = getTemplate('claude-header.md');
      expect(template).toContain('TypeScript Linting Rules');
    });
  });

  describe('buildOpenCodeFormat', () => {
    test('combines frontmatter with content', () => {
      const result = buildOpenCodeFormat();
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('mode: subagent');
      expect(result).toContain('## Your Responsibilities');
    });

    test('includes OpenCode-specific permissions', () => {
      const result = buildOpenCodeFormat();
      expect(result).toContain('permission:');
      expect(result).toContain("'npm *': allow");
    });
  });

  describe('buildCursorFormat', () => {
    test('combines frontmatter with content', () => {
      const result = buildCursorFormat();
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('alwaysApply: false');
      expect(result).toContain('## Your Responsibilities');
    });

    test('includes Cursor-specific globs', () => {
      const result = buildCursorFormat();
      expect(result).toContain('globs:');
      expect(result).toContain("'*.ts'");
    });
  });

  describe('buildClaudeFormat', () => {
    test('combines header with content', () => {
      const result = buildClaudeFormat();
      expect(result).toContain('# TypeScript Linting Rules');
      expect(result).toContain('## Your Responsibilities');
    });

    test('does not include OpenCode frontmatter', () => {
      const result = buildClaudeFormat();
      expect(result).not.toContain('mode: subagent');
      expect(result).not.toContain('permission:');
    });
  });

  describe('content consistency', () => {
    test('all formats contain the same core content', () => {
      const opencode = buildOpenCodeFormat();
      const cursor = buildCursorFormat();
      const claude = buildClaudeFormat();

      // All should contain the core responsibilities section
      const coreSection = '## Your Responsibilities';
      expect(opencode).toContain(coreSection);
      expect(cursor).toContain(coreSection);
      expect(claude).toContain(coreSection);

      // All should contain the implementation order
      const implSection = '## Implementation Order';
      expect(opencode).toContain(implSection);
      expect(cursor).toContain(implSection);
      expect(claude).toContain(implSection);
    });
  });
});
