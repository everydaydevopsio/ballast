import path from 'path';
import {
  getContent,
  getTemplate,
  listRuleSuffixes,
  buildCursorFormat,
  buildClaudeFormat,
  buildOpenCodeFormat,
  buildCodexFormat,
  buildContent,
  buildCodexAgentsMd,
  getCodexAgentsMdPath,
  getCodexRuleDescription,
  extractDescriptionFromFrontmatter,
  getDestination,
  listTargets
} from './build';

describe('build', () => {
  describe('listRuleSuffixes', () => {
    test('returns only main rule for linting (no content-*.md)', () => {
      expect(listRuleSuffixes('linting')).toEqual(['']);
    });

    test('returns only main rule for logging', () => {
      expect(listRuleSuffixes('logging')).toEqual(['']);
    });

    test('returns only main rule for testing', () => {
      expect(listRuleSuffixes('testing')).toEqual(['']);
    });

    test('returns env, mcp, license, and badges for local-dev', () => {
      expect(listRuleSuffixes('local-dev')).toContain('env');
      expect(listRuleSuffixes('local-dev')).toContain('mcp');
      expect(listRuleSuffixes('local-dev')).toContain('license');
      expect(listRuleSuffixes('local-dev')).toContain('badges');
      expect(listRuleSuffixes('local-dev').length).toBe(4);
    });

    test('throws for unknown agent', () => {
      expect(() => listRuleSuffixes('nonexistent')).toThrow(/content.md/);
    });
  });

  describe('getContent', () => {
    test('returns content for linting agent', () => {
      const content = getContent('linting');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Your Responsibilities');
    });

    test('returns content for logging agent', () => {
      const content = getContent('logging');
      expect(content).toContain('Centralized Logging Agent');
      expect(content).toContain('pino-browser');
      expect(content).toContain('/api/logs');
    });

    test('returns env content for local-dev with ruleSuffix env', () => {
      const content = getContent('local-dev', 'env');
      expect(content).toContain('Local Development Environment Agent');
    });

    test('returns mcp content for local-dev with ruleSuffix mcp', () => {
      const content = getContent('local-dev', 'mcp');
      expect(content).toContain('GitHub MCP');
      expect(content).toContain('Jira');
      expect(content).toContain('Linear');
    });

    test('throws for unknown agent', () => {
      expect(() => getContent('nonexistent')).toThrow(/content.md/);
    });

    test('throws for missing optional rule', () => {
      expect(() => getContent('linting', 'mcp')).toThrow(/content-mcp.md/);
    });
  });

  describe('getTemplate', () => {
    test('reads cursor frontmatter for linting', () => {
      const t = getTemplate('linting', 'cursor-frontmatter.yaml');
      expect(t).toContain('alwaysApply: false');
      expect(t).toContain('globs:');
    });

    test('reads rule-specific cursor frontmatter for local-dev mcp', () => {
      const t = getTemplate('local-dev', 'cursor-frontmatter.yaml', 'mcp');
      expect(t).toContain('GitHub MCP');
      expect(t).toContain('Jira/Linear/GitHub');
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

  describe('buildCodexFormat', () => {
    test('combines header with content for linting', () => {
      const result = buildCodexFormat('linting');
      expect(result).toContain('# TypeScript Linting Rules');
      expect(result).toContain('## Your Responsibilities');
    });
  });

  describe('getCodexRuleDescription', () => {
    test('reads description from cursor frontmatter', () => {
      const description = getCodexRuleDescription('linting');
      expect(description).toContain('TypeScript linting specialist');
    });
  });

  describe('extractDescriptionFromFrontmatter', () => {
    test('extracts single-line quoted description', () => {
      const frontmatter = `---
description: 'TypeScript linting specialist'
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe(
        'TypeScript linting specialist'
      );
    });

    test('extracts multi-line literal block (|) description', () => {
      const frontmatter = `---
description: |
  First line
  Second line
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe(
        'First line\nSecond line'
      );
    });

    test('extracts multi-line folded block (>) description', () => {
      const frontmatter = `---
description: >
  This is a folded
  block scalar
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe(
        'This is a folded block scalar'
      );
    });

    test('handles single-quoted string with escaped single quote', () => {
      const frontmatter = `---
description: 'It''s great'
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe("It's great");
    });
  });

  describe('buildCodexAgentsMd', () => {
    test('lists codex rule files with descriptions', () => {
      const content = buildCodexAgentsMd(['linting']);
      expect(content).toContain('# AGENTS.md');
      expect(content).toContain('`.codex/rules/linting.md`');
      expect(content).toContain('TypeScript linting specialist');
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

    test('codex returns header + content', () => {
      const result = buildContent('linting', 'codex');
      expect(result).toContain('# TypeScript Linting Rules');
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

    test('codex returns .codex/rules/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'codex', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.codex', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.codex', 'rules', 'linting.md')
      );
    });

    test('codex agents.md path returns project root AGENTS.md', () => {
      const agentsMd = getCodexAgentsMdPath(projectRoot);
      expect(agentsMd).toBe(path.join(projectRoot, 'AGENTS.md'));
    });

    test('cursor with ruleSuffix returns .cursor/rules/<agent>-<suffix>.mdc', () => {
      const { dir, file } = getDestination(
        'local-dev',
        'cursor',
        projectRoot,
        'env'
      );
      expect(dir).toBe(path.join(projectRoot, '.cursor', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.cursor', 'rules', 'local-dev-env.mdc')
      );
    });

    test('throws for unknown target', () => {
      expect(() =>
        getDestination('linting', 'unknown' as 'cursor', projectRoot)
      ).toThrow(/Unknown target/);
    });
  });

  describe('listTargets', () => {
    test('returns cursor, claude, opencode, codex', () => {
      expect(listTargets()).toEqual(['cursor', 'claude', 'opencode', 'codex']);
    });
  });
});
