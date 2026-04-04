import fs from 'fs';
import path from 'path';
import os from 'os';
import readline from 'readline';
import { install, resolveTargetAndAgents, runInstall } from './install';
import { getClaudeMdPath, getDestination } from './build';
import { findProjectRoot, saveConfig, loadConfig } from './config';
import { BALLAST_VERSION } from './version';

describe('install', () => {
  let tmpDir: string;
  const origEnv = process.env;

  beforeEach(() => {
    tmpDir = path.join(os.tmpdir(), `ballast-install-${Date.now()}`);
    fs.mkdirSync(tmpDir, { recursive: true });
    process.env = { ...origEnv };
    delete process.env.CI;
    delete process.env.GITHUB_ACTIONS;
  });

  afterEach(() => {
    if (tmpDir && fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true });
    }
    process.env = origEnv;
  });

  describe('resolveTargetAndAgents', () => {
    test('with target and agents from options returns them', async () => {
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['cursor'],
        agents: ['linting']
      });
      expect(result).toEqual({
        targets: ['cursor'],
        agents: ['linting', 'git-hooks'],
        skills: []
      });
    });

    test('with --all expands to all agents', async () => {
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['claude'],
        agents: 'all'
      });
      expect(result?.targets).toEqual(['claude']);
      expect(result?.agents).toContain('linting');
      expect(result?.agents).toContain('local-dev');
      expect(result?.agents).toContain('docs');
      expect(result?.agents).toContain('cicd');
      expect(result?.agents).toContain('observability');
      expect(result?.agents).toContain('publishing');
      expect(result?.agents).toContain('git-hooks');
      expect(result?.agents).toContain('logging');
      expect(result?.agents).toContain('testing');
      expect(result?.skills).toEqual([]);
    });

    test('with saved config returns config when no flags', async () => {
      saveConfig(
        {
          targets: ['opencode'],
          agents: ['linting', 'cicd'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir
      });
      expect(result).toEqual({
        targets: ['opencode'],
        agents: ['linting', 'cicd', 'git-hooks'],
        skills: ['owasp-security-scan']
      });
    });

    test('in CI with no config and no args returns null', async () => {
      process.env.CI = 'true';
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        yes: true
      });
      expect(result).toBeNull();
    });

    test('in CI with target and agents returns them', async () => {
      process.env.CI = 'true';
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        yes: true,
        targets: ['cursor'],
        agents: ['linting']
      });
      expect(result).toEqual({
        targets: ['cursor'],
        agents: ['linting', 'git-hooks'],
        skills: []
      });
    });

    test('supports multi-target flags and saved config', async () => {
      saveConfig(
        {
          targets: ['cursor', 'claude'],
          agents: ['linting'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir
      });
      expect(result).toEqual({
        targets: ['cursor', 'claude'],
        agents: ['linting', 'git-hooks'],
        skills: ['owasp-security-scan']
      });
    });

    test('interactive selection normalizes implicit git-hooks before returning', async () => {
      const answers = ['cursor', 'linting', ''];
      const createInterfaceSpy = jest
        .spyOn(readline, 'createInterface')
        .mockImplementation(
          () =>
            ({
              question: (_prompt: string, cb: (answer: string) => void) =>
                cb(answers.shift() ?? ''),
              close: () => {}
            }) as unknown as readline.Interface
        );
      try {
        const result = await resolveTargetAndAgents({
          projectRoot: tmpDir
        });

        expect(result).toEqual({
          targets: ['cursor'],
          agents: ['linting', 'git-hooks'],
          skills: []
        });
      } finally {
        createInterfaceSpy.mockRestore();
      }
    });

    test('rejects invalid target flags instead of ignoring them', async () => {
      await expect(
        resolveTargetAndAgents({
          projectRoot: tmpDir,
          targets: ['cursor', 'bogus'],
          agents: ['linting']
        })
      ).rejects.toThrow(
        'Invalid --target. Use: cursor, claude, opencode, codex'
      );
    });
  });

  describe('install', () => {
    test('writes files and returns installed list', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        skills: ['owasp-security-scan'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(result.installedSkills).toContain('owasp-security-scan');
      expect(result.errors).toHaveLength(0);
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      expect(fs.readFileSync(cursorFile, 'utf8')).toContain(
        'TypeScript linting specialist'
      );
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'owasp-security-scan.mdc')
        )
      ).toBe(true);
    });

    test('writes ansible language rules when requested', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        language: 'ansible',
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('linting');
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'ansible-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      expect(fs.readFileSync(cursorFile, 'utf8')).toContain(
        'Ansible linting specialist'
      );
    });

    test('writes terraform language rules when requested', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        language: 'terraform',
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('linting');
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'terraform-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).toContain('Terraform linting specialist');
      expect(content).toContain('.terraform-version');
      expect(content).toContain('tfenv install');
      expect(content).toContain('terraform fmt -check -recursive');
      expect(content).toContain('tfsec');
    });

    test('uses pre-commit guidance for standalone typescript installs', () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          languages: ['typescript']
        },
        tmpDir
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });

      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).not.toContain('.pre-commit-config.yaml');
      expect(content).not.toContain('pre-commit install');
      expect(content).not.toContain('pre-commit install --hook-type pre-push');
      expect(content).not.toContain('Use Husky for this monorepo.');

      const gitHooksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'git-hooks.mdc'
      );
      expect(fs.existsSync(gitHooksFile)).toBe(true);
      const gitHooksContent = fs.readFileSync(gitHooksFile, 'utf8');
      expect(gitHooksContent).toContain('.pre-commit-config.yaml');
      expect(gitHooksContent).toContain(
        'pre-commit install --hook-type pre-push'
      );
    });

    test('uses husky guidance for monorepo typescript installs', () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          languages: ['typescript', 'python', 'go', 'ansible', 'terraform']
        },
        tmpDir
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });

      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).not.toContain('Use Husky for this monorepo.');
      expect(content).not.toContain('lint-staged');
      expect(content).not.toContain('.husky/pre-push');
      expect(content).not.toContain('pre-commit install');

      const gitHooksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'git-hooks.mdc'
      );
      expect(fs.existsSync(gitHooksFile)).toBe(true);
      const gitHooksContent = fs.readFileSync(gitHooksFile, 'utf8');
      expect(gitHooksContent).toContain('Use Husky for this monorepo.');
      expect(gitHooksContent).toContain('lint-staged');
      expect(gitHooksContent).toContain('.husky/pre-push');
    });

    test('uses husky guidance for typescript workspace monorepos even with one language', () => {
      fs.writeFileSync(
        path.join(tmpDir, 'package.json'),
        JSON.stringify({ name: 'workspace-root', private: true }, null, 2)
      );
      fs.writeFileSync(
        path.join(tmpDir, 'pnpm-workspace.yaml'),
        'packages:\n  - apps/*\n  - packages/*\n',
        'utf8'
      );
      fs.mkdirSync(path.join(tmpDir, 'apps', 'web'), { recursive: true });
      fs.writeFileSync(
        path.join(tmpDir, 'apps', 'web', 'package.json'),
        JSON.stringify({ name: '@app/web', private: true }, null, 2)
      );
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          languages: ['typescript']
        },
        tmpDir
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });

      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).not.toContain('Set Up Git Hooks with Husky');
      expect(content).not.toContain('pre-commit install');

      const gitHooksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'git-hooks.mdc'
      );
      expect(fs.readFileSync(gitHooksFile, 'utf8')).toContain(
        'Use Husky for this monorepo.'
      );
    });

    test('writes python language files and uses python-specific content', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        language: 'python',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'python-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      expect(fs.readFileSync(cursorFile, 'utf8')).toContain(
        'Python linting specialist'
      );
    });

    test('skips when file exists and force is false', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'existing content'
      );
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['git-hooks']);
      expect(result.skipped).toContain('linting');
      expect(
        fs.readFileSync(path.join(cursorDir, 'typescript-linting.mdc'), 'utf8')
      ).toBe('existing content');
    });

    test('overwrites when force is true', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'existing content'
      );
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(
        fs.readFileSync(path.join(cursorDir, 'typescript-linting.mdc'), 'utf8')
      ).toContain('TypeScript linting specialist');
    });

    test('patches an existing rule file when patch is true', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        `---
description: Team customized linting rules
alwaysApply: true
---

User intro.

## Your Responsibilities

Keep my custom responsibilities.

## Team Overrides

Do not remove this note.
`
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('linting');
      const content = fs.readFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'utf8'
      );
      expect(content).toContain('description: Team customized linting rules');
      expect(content).toContain('alwaysApply: true');
      expect(content).toContain('Keep my custom responsibilities.');
      expect(content).toContain('## Team Overrides');
      expect(content).toContain('## When Completed');
    });

    test('force wins over patch when both flags are true', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        `---
description: Team customized linting rules
alwaysApply: true
---

User intro.

## Your Responsibilities

Keep my custom responsibilities.
`
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        patch: true,
        force: true,
        saveConfig: false
      });

      const content = fs.readFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'utf8'
      );
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('alwaysApply: false');
      expect(content).not.toContain('Keep my custom responsibilities.');
    });

    test('saves config when saveConfig is true', () => {
      install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });
      const config = loadConfig(tmpDir);
      expect(config).toEqual({
        targets: ['claude'],
        agents: ['linting', 'local-dev', 'git-hooks'],
        ballastVersion: BALLAST_VERSION,
        languages: ['typescript'],
        paths: { typescript: ['.'] }
      });
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.languages).toEqual(['typescript']);
      expect(raw.ballastVersion).toBe(BALLAST_VERSION);
      expect(raw.paths).toEqual({ typescript: ['.'] });
    });

    test('saves shared .rulesrc.json for go installs', () => {
      install({
        projectRoot: tmpDir,
        target: 'claude',
        language: 'go',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });
      const config = loadConfig(tmpDir, 'go');
      expect(config).toEqual({
        targets: ['claude'],
        agents: ['linting', 'local-dev', 'git-hooks'],
        ballastVersion: BALLAST_VERSION,
        languages: ['go'],
        paths: { go: ['.'] }
      });
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.languages).toEqual(['go']);
      expect(raw.ballastVersion).toBe(BALLAST_VERSION);
      expect(raw.paths).toEqual({ go: ['.'] });
    });

    test('manual language installs accumulate languages in shared config', () => {
      install({
        projectRoot: tmpDir,
        target: 'claude',
        language: 'typescript',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });
      install({
        projectRoot: tmpDir,
        target: 'claude',
        language: 'go',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });

      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.ballastVersion).toBe(BALLAST_VERSION);
      expect(raw.languages).toEqual(['typescript', 'go']);
      expect(raw.paths).toEqual({
        typescript: ['.'],
        go: ['.']
      });
    });

    test('adds .ballast/ to .gitignore during install', () => {
      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(
        fs.readFileSync(path.join(tmpDir, '.gitignore'), 'utf8')
      ).toContain('.ballast/');
    });

    test('does not duplicate .ballast/ in an existing .gitignore', () => {
      fs.writeFileSync(path.join(tmpDir, '.gitignore'), '.ballast/\n', 'utf8');

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(fs.readFileSync(path.join(tmpDir, '.gitignore'), 'utf8')).toBe(
        '.ballast/\n'
      );
    });

    test('appends .ballast/ to an existing .gitignore without trailing newline', () => {
      fs.writeFileSync(path.join(tmpDir, '.gitignore'), 'node_modules', 'utf8');

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(fs.readFileSync(path.join(tmpDir, '.gitignore'), 'utf8')).toBe(
        'node_modules\n.ballast/\n'
      );
    });

    test('records a gitignore error and continues install when .gitignore is unreadable', () => {
      fs.mkdirSync(path.join(tmpDir, '.gitignore'));

      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(result.errors).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ agent: 'gitignore' })
        ])
      );
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        )
      ).toBe(true);
    });

    test('installs multiple agents', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting', 'docs', 'cicd'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(result.installed).toContain('docs');
      expect(result.installed).toContain('cicd');
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        )
      ).toBe(true);
      expect(
        fs.existsSync(path.join(tmpDir, '.cursor', 'rules', 'docs.mdc'))
      ).toBe(true);
      expect(
        fs.existsSync(path.join(tmpDir, '.cursor', 'rules', 'cicd.mdc'))
      ).toBe(true);
      expect(result.installedRules).toContainEqual({
        agentId: 'docs',
        ruleSuffix: ''
      });
    });

    test('installs docs rule', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['docs'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['docs']);
      expect(result.installedRules).toEqual([
        {
          agentId: 'docs',
          ruleSuffix: ''
        }
      ]);
      const docsFile = path.join(tmpDir, '.cursor', 'rules', 'docs.mdc');
      expect(fs.existsSync(docsFile)).toBe(true);
      expect(fs.readFileSync(docsFile, 'utf8')).toContain(
        'Documentation Agent'
      );
      expect(fs.readFileSync(docsFile, 'utf8')).toContain('publish-docs');
    });

    test('installs all rules for agent with multiple rules (local-dev)', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['local-dev'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['local-dev']);
      expect(result.installedRules.length).toBe(4);
      const envFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-env.mdc'
      );
      const mcpFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-mcp.mdc'
      );
      const licenseFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-license.mdc'
      );
      const badgesFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-badges.mdc'
      );
      expect(fs.existsSync(envFile)).toBe(true);
      expect(fs.existsSync(mcpFile)).toBe(true);
      expect(fs.existsSync(licenseFile)).toBe(true);
      expect(fs.existsSync(badgesFile)).toBe(true);
      expect(fs.readFileSync(envFile, 'utf8')).toContain(
        'Local Development Environment Agent'
      );
      expect(fs.readFileSync(mcpFile, 'utf8')).toContain('GitHub MCP');
      expect(fs.readFileSync(licenseFile, 'utf8')).toContain('LICENSE');
      expect(fs.readFileSync(badgesFile, 'utf8')).toContain('README Badges');
    });

    test('installs all rules for agent with multiple rules (publishing)', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['publishing'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['publishing']);
      expect(result.installedRules.length).toBe(3);
      const librariesFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-libraries.mdc'
      );
      const sdksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-sdks.mdc'
      );
      const appsFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-apps.mdc'
      );
      expect(fs.existsSync(librariesFile)).toBe(true);
      expect(fs.existsSync(sdksFile)).toBe(true);
      expect(fs.existsSync(appsFile)).toBe(true);
      expect(fs.readFileSync(librariesFile, 'utf8')).toContain(
        'Publishing Libraries Agent'
      );
      expect(fs.readFileSync(sdksFile, 'utf8')).toContain(
        'Publishing SDKs Agent'
      );
      expect(fs.readFileSync(appsFile, 'utf8')).toContain(
        'Publishing Apps Agent'
      );
    });

    test('adds to errors for unknown agent and continues with valid ones', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting', 'unknown-agent', 'cicd'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['linting', 'cicd', 'git-hooks']);
      expect(result.errors).toHaveLength(1);
      expect(result.errors[0]).toEqual({
        agent: 'unknown-agent',
        error: 'Unknown agent'
      });
    });

    describe('file locations', () => {
      test('cursor: writes to .cursor/rules/<agent>.mdc', () => {
        install({
          projectRoot: tmpDir,
          target: 'cursor',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'cursor', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        );
        expect(dir).toBe(path.join(tmpDir, '.cursor', 'rules'));
      });

      test('claude: writes to .claude/rules/<agent>.md', () => {
        install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'claude', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.claude', 'rules', 'typescript-linting.md')
        );
        expect(dir).toBe(path.join(tmpDir, '.claude', 'rules'));
        const claudeMd = getClaudeMdPath(tmpDir);
        expect(fs.existsSync(claudeMd)).toBe(true);
        expect(fs.readFileSync(claudeMd, 'utf8')).toContain(
          '`.claude/rules/typescript-linting.md`'
        );
      });

      test('claude skips existing CLAUDE.md unless patch is approved', () => {
        fs.writeFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          `# CLAUDE.md

## Team Notes

Keep this section.
`
        );

        const result = install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });

        expect(result.skippedSupportFiles).toContain(
          path.join(tmpDir, 'CLAUDE.md')
        );
        expect(
          fs.readFileSync(path.join(tmpDir, 'CLAUDE.md'), 'utf8')
        ).toContain('## Team Notes');
      });

      test('claude patch updates installed rules section without removing user notes', () => {
        const claudeRulesDir = path.join(tmpDir, '.claude', 'rules');
        fs.mkdirSync(claudeRulesDir, { recursive: true });
        fs.writeFileSync(
          path.join(claudeRulesDir, 'typescript-linting.md'),
          `# TypeScript Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
`
        );
        fs.writeFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          `# CLAUDE.md

## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in \`.claude/rules/\` when they apply:

- \`.claude/rules/old.md\` — Old rule
`
        );

        install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          patchClaudeMd: true,
          force: false,
          saveConfig: false
        });

        const claudeMd = fs.readFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          'utf8'
        );
        expect(claudeMd).toContain('## Team Notes');
        expect(claudeMd).toContain('Keep this section.');
        expect(claudeMd).toContain('`.claude/rules/typescript-linting.md`');
        expect(claudeMd).not.toContain('`.claude/rules/old.md`');
      });

      test('claude --patch updates installed rules section without prompt approval', () => {
        fs.writeFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          `# CLAUDE.md

## Installed agent rules

Read and follow these rule files in \`.claude/rules/\` when they apply:

- \`.claude/rules/old.md\` — Old rule
`
        );

        const result = install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          patch: true,
          force: false,
          saveConfig: false
        });

        expect(result.installedSupportFiles).toContain(
          path.join(tmpDir, 'CLAUDE.md')
        );
        const claudeMd = fs.readFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          'utf8'
        );
        expect(claudeMd).toContain('`.claude/rules/typescript-linting.md`');
        expect(claudeMd).not.toContain('`.claude/rules/old.md`');
      });

      test('opencode: writes to .opencode/<agent>.md', () => {
        install({
          projectRoot: tmpDir,
          target: 'opencode',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'opencode', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.opencode', 'typescript-linting.md')
        );
        expect(dir).toBe(path.join(tmpDir, '.opencode'));
      });

      test('codex: writes to .codex/rules/<agent>.md and AGENTS.md', () => {
        install({
          projectRoot: tmpDir,
          target: 'codex',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'codex', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.codex', 'rules', 'typescript-linting.md')
        );
        expect(dir).toBe(path.join(tmpDir, '.codex', 'rules'));
        const agentsMd = path.join(tmpDir, 'AGENTS.md');
        expect(fs.existsSync(agentsMd)).toBe(true);
        expect(fs.readFileSync(agentsMd, 'utf8')).toContain(
          '`.codex/rules/typescript-linting.md`'
        );
      });

      test('codex patch updates installed rules section without removing user notes', () => {
        const codexDir = path.join(tmpDir, '.codex', 'rules');
        fs.mkdirSync(codexDir, { recursive: true });
        fs.writeFileSync(
          path.join(codexDir, 'typescript-linting.md'),
          `# TypeScript Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
`
        );
        fs.writeFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          `# AGENTS.md

## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in \`.codex/rules/\` when they apply:

- \`.codex/rules/old.md\` — Old rule
`
        );

        install({
          projectRoot: tmpDir,
          target: 'codex',
          agents: ['linting'],
          patch: true,
          force: false,
          saveConfig: false
        });

        const codexRule = fs.readFileSync(
          path.join(codexDir, 'typescript-linting.md'),
          'utf8'
        );
        expect(codexRule).toContain('Keep my custom rule text.');
        expect(codexRule).toContain('## When Completed');

        const agentsMd = fs.readFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          'utf8'
        );
        expect(agentsMd).toContain('## Team Notes');
        expect(agentsMd).toContain('Keep this section.');
        expect(agentsMd).toContain('`.codex/rules/typescript-linting.md`');
        expect(agentsMd).not.toContain('`.codex/rules/old.md`');
      });

      test('written path matches getDestination for each target', () => {
        const targets: Array<{
          target: 'cursor' | 'claude' | 'opencode' | 'codex';
          ext: string;
        }> = [
          { target: 'cursor', ext: 'mdc' },
          { target: 'claude', ext: 'md' },
          { target: 'opencode', ext: 'md' },
          { target: 'codex', ext: 'md' }
        ];
        for (const { target, ext } of targets) {
          const subDir = path.join(tmpDir, target);
          fs.mkdirSync(subDir, { recursive: true });
          install({
            projectRoot: subDir,
            target,
            agents: ['linting'],
            force: false,
            saveConfig: false
          });
          const { file: expectedFile } = getDestination(
            'linting',
            target,
            subDir
          );
          expect(fs.existsSync(expectedFile)).toBe(true);
          expect(path.extname(expectedFile)).toBe(
            ext === 'mdc' ? '.mdc' : '.md'
          );
        }
      });
    });
  });

  describe('runInstall', () => {
    test('writes files for every requested target in one run', async () => {
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        targets: ['cursor', 'claude'],
        agents: ['linting'],
        yes: true
      });
      expect(exitCode).toBe(0);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        )
      ).toBe(true);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.claude', 'rules', 'typescript-linting.md')
        )
      ).toBe(true);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.targets).toEqual(['cursor', 'claude']);
    });

    test('writes files to correct locations for given target', async () => {
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'claude',
        agents: ['linting'],
        yes: true
      });
      expect(exitCode).toBe(0);
      const { file } = getDestination('linting', 'claude', tmpDir);
      expect(fs.existsSync(file)).toBe(true);
      expect(file).toBe(
        path.join(tmpDir, '.claude', 'rules', 'typescript-linting.md')
      );
      expect(fs.existsSync(path.join(tmpDir, '.rulesrc.json'))).toBe(true);
    });

    test('uses saved config when CLI passes empty agent and skill arrays', async () => {
      saveConfig(
        {
          targets: ['codex'],
          agents: ['linting'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        agents: [],
        skills: [],
        yes: true
      });

      expect(exitCode).toBe(0);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.codex', 'rules', 'typescript-linting.md')
        )
      ).toBe(true);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.codex', 'rules', 'owasp-security-scan.md')
        )
      ).toBe(true);
    });

    test('returns 1 when CI and no config and no target/agents', async () => {
      process.env.CI = 'true';
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        yes: true
      });
      expect(exitCode).toBe(1);
    });
  });
});
