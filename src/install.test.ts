import fs from 'fs';
import path from 'path';
import os from 'os';
import { install, resolveTargetAndAgents, runInstall } from './install';
import { getDestination } from './build';
import { findProjectRoot, saveConfig, loadConfig } from './config';

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
        target: 'cursor',
        agents: ['linting']
      });
      expect(result).toEqual({ target: 'cursor', agents: ['linting'] });
    });

    test('with --all expands to all agents', async () => {
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        target: 'claude',
        agents: 'all'
      });
      expect(result?.target).toBe('claude');
      expect(result?.agents).toContain('linting');
      expect(result?.agents).toContain('local-dev');
      expect(result?.agents).toContain('cicd');
      expect(result?.agents).toContain('observability');
      expect(result?.agents).toContain('logging');
      expect(result?.agents).toContain('testing');
    });

    test('with saved config returns config when no flags', async () => {
      saveConfig({ target: 'opencode', agents: ['linting', 'cicd'] }, tmpDir);
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir
      });
      expect(result).toEqual({
        target: 'opencode',
        agents: ['linting', 'cicd']
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
        target: 'cursor',
        agents: ['linting']
      });
      expect(result).toEqual({ target: 'cursor', agents: ['linting'] });
    });
  });

  describe('install', () => {
    test('writes files and returns installed list', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(result.errors).toHaveLength(0);
      const cursorFile = path.join(tmpDir, '.cursor', 'rules', 'linting.mdc');
      expect(fs.existsSync(cursorFile)).toBe(true);
      expect(fs.readFileSync(cursorFile, 'utf8')).toContain(
        'TypeScript linting specialist'
      );
    });

    test('skips when file exists and force is false', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(path.join(cursorDir, 'linting.mdc'), 'existing content');
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toHaveLength(0);
      expect(result.skipped).toContain('linting');
      expect(fs.readFileSync(path.join(cursorDir, 'linting.mdc'), 'utf8')).toBe(
        'existing content'
      );
    });

    test('overwrites when force is true', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(path.join(cursorDir, 'linting.mdc'), 'existing content');
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(
        fs.readFileSync(path.join(cursorDir, 'linting.mdc'), 'utf8')
      ).toContain('TypeScript linting specialist');
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
        target: 'claude',
        agents: ['linting', 'local-dev']
      });
    });

    test('installs multiple agents', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting', 'local-dev', 'cicd'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(result.installed).toContain('local-dev');
      expect(result.installed).toContain('cicd');
      expect(
        fs.existsSync(path.join(tmpDir, '.cursor', 'rules', 'linting.mdc'))
      ).toBe(true);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'local-dev-env.mdc')
        )
      ).toBe(true);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'local-dev-mcp.mdc')
        )
      ).toBe(true);
      expect(
        fs.existsSync(path.join(tmpDir, '.cursor', 'rules', 'cicd.mdc'))
      ).toBe(true);
      expect(result.installedRules).toContainEqual({
        agentId: 'local-dev',
        ruleSuffix: 'env'
      });
      expect(result.installedRules).toContainEqual({
        agentId: 'local-dev',
        ruleSuffix: 'mcp'
      });
      expect(result.installedRules).toContainEqual({
        agentId: 'local-dev',
        ruleSuffix: 'license'
      });
      expect(result.installedRules).toContainEqual({
        agentId: 'local-dev',
        ruleSuffix: 'badges'
      });
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

    test('adds to errors for unknown agent and continues with valid ones', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting', 'unknown-agent', 'cicd'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['linting', 'cicd']);
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
        expect(file).toBe(path.join(tmpDir, '.cursor', 'rules', 'linting.mdc'));
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
        expect(file).toBe(path.join(tmpDir, '.claude', 'rules', 'linting.md'));
        expect(dir).toBe(path.join(tmpDir, '.claude', 'rules'));
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
        expect(file).toBe(path.join(tmpDir, '.opencode', 'linting.md'));
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
        expect(file).toBe(path.join(tmpDir, '.codex', 'rules', 'linting.md'));
        expect(dir).toBe(path.join(tmpDir, '.codex', 'rules'));
        const agentsMd = path.join(tmpDir, 'AGENTS.md');
        expect(fs.existsSync(agentsMd)).toBe(true);
        expect(fs.readFileSync(agentsMd, 'utf8')).toContain(
          '`.codex/rules/linting.md`'
        );
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
      expect(file).toBe(path.join(tmpDir, '.claude', 'rules', 'linting.md'));
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
