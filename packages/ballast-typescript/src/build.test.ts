import path from 'path';
import {
  getContent,
  getTemplate,
  getSkillContent,
  listRuleSuffixes,
  buildCursorFormat,
  buildClaudeFormat,
  buildOpenCodeFormat,
  buildCodexFormat,
  buildContent,
  buildClaudeSkill,
  buildCursorSkillFormat,
  buildSkillMarkdown,
  buildClaudeMd,
  buildCodexAgentsMd,
  getClaudeMdPath,
  getCodexAgentsMdPath,
  getCodexRuleDescription,
  getSkillDescription,
  extractDescriptionFromFrontmatter,
  getDestination,
  getSkillDestination,
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

    test('returns only main rule for docs', () => {
      expect(listRuleSuffixes('docs')).toEqual(['']);
    });

    test('returns libraries, sdks, and apps for publishing', () => {
      expect(listRuleSuffixes('publishing')).toContain('libraries');
      expect(listRuleSuffixes('publishing')).toContain('sdks');
      expect(listRuleSuffixes('publishing')).toContain('apps');
      expect(listRuleSuffixes('publishing').length).toBe(3);
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
      expect(content).toContain('docker-compose.local.yaml');
      expect(content).toContain('Makefile');
      expect(content).toContain('make up-local');
    });

    test('returns mcp content for local-dev with ruleSuffix mcp', () => {
      const content = getContent('local-dev', 'mcp');
      expect(content).toContain('GitHub MCP');
      expect(content).toContain('Jira');
      expect(content).toContain('Linear');
    });

    test('returns docs content', () => {
      const content = getContent('docs');
      expect(content).toContain('Documentation Agent');
      expect(content).toContain('Default to a GitHub-readable Markdown');
      expect(content).toContain('publish-docs');
      expect(content).toContain('Mermaid');
    });

    test('returns publishing libraries content', () => {
      const content = getContent('publishing', 'libraries');
      expect(content).toContain('Publishing Libraries Agent');
      expect(content).toContain('release_type');
      expect(content).toContain('patch');
      expect(content).toContain('minor');
      expect(content).toContain('major');
      expect(content).toContain('bump_and_tag');
      expect(content).toContain(
        'WyriHaximus/github-action-get-previous-tag@v2'
      );
      expect(content).toContain('WyriHaximus/github-action-next-semvers');
      expect(content).toContain('npm publish --access public --provenance');
      expect(content).toContain('PyPI');
      expect(content).toContain('GitHub Releases');
    });

    test('returns publishing apps content for web app containers and helm repos', () => {
      const content = getContent('publishing', 'apps');
      expect(content).toContain('release_type');
      expect(content).toContain('v<version>');
      expect(content).toContain(
        'WyriHaximus/github-action-get-previous-tag@v2'
      );
      expect(content).toContain('WyriHaximus/github-action-next-semvers');
      expect(content).toContain('ghcr.io');
      expect(content).toContain('Docker Hub');
      expect(content).toContain('Helm chart repository');
      expect(content).toContain('image digest');
    });

    test('throws for unknown agent', () => {
      expect(() => getContent('nonexistent')).toThrow(/content.md/);
    });

    test('throws for missing optional rule', () => {
      expect(() => getContent('linting', 'mcp')).toThrow(/content-mcp.md/);
    });

    test('returns python-specific linting content without hook guidance when language is python', () => {
      const content = getContent('linting', undefined, 'python');
      expect(content).toContain('Python linting specialist');
      expect(content).toContain('Ruff');
      expect(content).not.toContain('.pre-commit-config.yaml');
      expect(content).not.toContain('pre-commit install');
      expect(content).not.toContain('pre-commit install --hook-type pre-push');
      expect(content).not.toContain('pre-commit autoupdate');
    });

    test('returns go-specific linting content without hook guidance', () => {
      const content = getContent('linting', undefined, 'go');
      expect(content).toContain('Go linting specialist');
      expect(content).not.toContain('.pre-commit-config.yaml');
      expect(content).not.toContain('sub-pre-commit');
      expect(content).not.toContain('pre-commit install --hook-type pre-push');
      expect(content).not.toContain('pre-commit autoupdate');
    });

    test('returns standalone git-hooks content for python', () => {
      const content = getContent('git-hooks', undefined, 'python');
      expect(content).toContain('Git hook specialist');
      expect(content).toContain('.pre-commit-config.yaml');
      expect(content).toContain('pre-commit install --hook-type pre-push');
      expect(content).toContain('pre-commit autoupdate');
    });

    test('returns monorepo git-hooks content for typescript', () => {
      const content = getContent('git-hooks', undefined, 'typescript', {
        hookMode: 'monorepo'
      });
      expect(content).toContain('Use Husky for this monorepo.');
      expect(content).toContain('.husky/pre-push');
      expect(content).not.toContain('pre-commit install');
    });

    test('returns go-specific testing content when language is go', () => {
      const content = getContent('testing', undefined, 'go');
      expect(content).toContain('Go testing specialist');
      expect(content).toContain('go test ./...');
    });
  });

  describe('skills', () => {
    test('reads skill content', () => {
      const content = getSkillContent('owasp-security-scan');
      expect(content).toContain('name: owasp-security-scan');
      expect(content).toContain('# OWASP Security Scan Skill');
    });

    test('reads aws health skill content', () => {
      const content = getSkillContent('aws-health-review');
      expect(content).toContain('name: aws-health-review');
      expect(content).toContain('# AWS Health Review');
    });

    test('builds cursor skill format', () => {
      const content = buildCursorSkillFormat('owasp-security-scan');
      expect(content).toContain('alwaysApply: false');
      expect(content).toContain('# OWASP Security Scan Skill');
    });

    test('builds markdown skill format', () => {
      const content = buildSkillMarkdown('owasp-security-scan');
      expect(content).toContain('# OWASP Security Scan Skill');
      expect(content).not.toContain('name: owasp-security-scan');
    });

    test('builds claude skill zip with references', () => {
      const archive = buildClaudeSkill('owasp-security-scan');
      expect(archive.subarray(0, 4).toString('hex')).toBe('504b0304');
      expect(archive.includes(Buffer.from('SKILL.md'))).toBe(true);
      expect(archive.includes(Buffer.from('references/owasp-mapping.md'))).toBe(
        true
      );
    });

    test('gets skill description and destination', () => {
      expect(getSkillDescription('owasp-security-scan')).toContain(
        'Run OWASP-aligned security scans'
      );
      expect(
        getSkillDestination('owasp-security-scan', 'claude', '/tmp/project')
      ).toEqual({
        dir: path.join('/tmp/project', '.claude', 'skills'),
        file: path.join(
          '/tmp/project',
          '.claude',
          'skills',
          'owasp-security-scan.skill'
        )
      });
    });

    test('gets aws live health skill description', () => {
      expect(getSkillDescription('aws-live-health-review')).toContain(
        'Run a read-only AWS live health review'
      );
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

    test('reads rule-specific cursor frontmatter for publishing sdks', () => {
      const t = getTemplate('publishing', 'cursor-frontmatter.yaml', 'sdks');
      expect(t).toContain('SDK publishing specialist');
    });

    test('reads cursor frontmatter for docs', () => {
      const t = getTemplate('docs', 'cursor-frontmatter.yaml');
      expect(t).toContain('Documentation specialist');
      expect(t).toContain('docusaurus.config.*');
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

    test('wraps plain yaml frontmatter for publishing templates', () => {
      const result = buildCursorFormat('publishing', 'libraries');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain("description: 'Library publishing specialist");
      expect(result).toContain('\n---\n# Publishing Libraries Agent');
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

    test('wraps plain yaml frontmatter for publishing templates', () => {
      const result = buildOpenCodeFormat('publishing', 'apps');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('mode: subagent');
      expect(result).toContain('\n---\n# Publishing Apps Agent');
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
      const content = buildCodexAgentsMd(['linting'], ['owasp-security-scan']);
      expect(content).toContain('# AGENTS.md');
      expect(content).toContain('## Repository Facts');
      expect(content).toContain('Canonical GitHub repo: `<OWNER/REPO>`');
      expect(content).toContain(
        'Prefer facts stored here over re-deriving them with shell commands on every task.'
      );
      expect(content).toMatch(
        /Created by \[Ballast]\(https:\/\/github\.com\/everydaydevopsio\/ballast\) v[0-9A-Za-z._-]+\. Do not edit this section\./
      );
      expect(content).toContain('`.codex/rules/typescript-linting.md`');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Installed skills');
      expect(content).toContain('`.codex/rules/owasp-security-scan.md`');
    });
  });

  describe('buildClaudeMd', () => {
    test('lists claude rule files with descriptions', () => {
      const content = buildClaudeMd(['linting'], ['owasp-security-scan']);
      expect(content).toContain('# CLAUDE.md');
      expect(content).toContain('## Repository Facts');
      expect(content).toContain('Primary CI workflows: `<workflow filenames>`');
      expect(content).toMatch(
        /Created by \[Ballast]\(https:\/\/github\.com\/everydaydevopsio\/ballast\) v[0-9A-Za-z._-]+\. Do not edit this section\./
      );
      expect(content).toContain('`.claude/rules/typescript-linting.md`');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Installed skills');
      expect(content).toContain('`.claude/skills/owasp-security-scan.skill`');
    });
  });

  describe('buildContent', () => {
    test('cursor returns mdc-style content', () => {
      const result = buildContent('linting', 'cursor');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('globs:');
    });

    test('standalone typescript linting content no longer includes hook guidance', () => {
      const result = buildContent('linting', 'codex', undefined, 'typescript', {
        hookMode: 'standalone'
      });
      expect(result).not.toContain(
        'Use `pre-commit` for this repository layout.'
      );
      expect(result).not.toContain('Install hooks with `pre-commit install`.');
      expect(result).not.toContain(
        'Install the pre-push hook with `pre-commit install --hook-type pre-push`.'
      );
      expect(result).not.toContain('Set Up Git Hooks with Husky');
      expect(result).not.toContain('Use Husky for this monorepo.');
      expect(result).not.toContain('Configure lint-staged');
    });

    test('monorepo typescript git-hooks content is husky based', () => {
      const result = buildContent(
        'git-hooks',
        'codex',
        undefined,
        'typescript',
        {
          hookMode: 'monorepo'
        }
      );
      expect(result).toContain('Use Husky for this monorepo.');
      expect(result).toContain('npx lint-staged');
      expect(result).toContain('.husky/pre-push');
      expect(result).not.toContain('pre-commit install');
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
        path.join(projectRoot, '.cursor', 'rules', 'typescript-linting.mdc')
      );
    });

    test('claude returns .claude/rules/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'claude', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.claude', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.claude', 'rules', 'typescript-linting.md')
      );
    });

    test('opencode returns .opencode/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'opencode', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.opencode'));
      expect(file).toBe(
        path.join(projectRoot, '.opencode', 'typescript-linting.md')
      );
    });

    test('codex returns .codex/rules/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'codex', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.codex', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.codex', 'rules', 'typescript-linting.md')
      );
    });

    test('codex agents.md path returns project root AGENTS.md', () => {
      const agentsMd = getCodexAgentsMdPath(projectRoot);
      expect(agentsMd).toBe(path.join(projectRoot, 'AGENTS.md'));
    });

    test('claude md path returns project root CLAUDE.md', () => {
      const claudeMd = getClaudeMdPath(projectRoot);
      expect(claudeMd).toBe(path.join(projectRoot, 'CLAUDE.md'));
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

    test('rejects invalid BALLAST_RULE_SUBDIR values', () => {
      process.env.BALLAST_RULE_SUBDIR = '../escape';
      expect(() => getDestination('linting', 'codex', projectRoot)).toThrow(
        /Invalid BALLAST_RULE_SUBDIR/
      );
      delete process.env.BALLAST_RULE_SUBDIR;
    });
  });

  describe('listTargets', () => {
    test('returns cursor, claude, opencode, codex', () => {
      expect(listTargets()).toEqual(['cursor', 'claude', 'opencode', 'codex']);
    });
  });
});
