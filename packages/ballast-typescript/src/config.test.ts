import fs from 'fs';
import path from 'path';
import os from 'os';
import {
  findProjectRoot,
  loadConfig,
  saveConfig,
  isCiMode,
  RULESRC_FILENAME,
  getLegacyRulesrcFilename,
  parseTargets,
  TASK_SYSTEMS,
  DEFAULT_TASK_SYSTEM,
  DEPLOYMENT_MODELS,
  DEFAULT_DEPLOYMENT_MODEL
} from './config';
import { BALLAST_VERSION } from './version';

function makeGitBoundary(dir: string): void {
  const gitDir = path.join(dir, '.git');
  fs.mkdirSync(gitDir, { recursive: true });
  fs.writeFileSync(path.join(gitDir, 'HEAD'), 'ref: refs/heads/main\n');
}

describe('config', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = path.join(os.tmpdir(), `ballast-config-${Date.now()}`);
    fs.mkdirSync(tmpDir, { recursive: true });
  });

  afterEach(() => {
    if (tmpDir && fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true });
    }
  });

  describe('findProjectRoot', () => {
    test('returns cwd when no config or package.json in cwd', () => {
      const cwd = tmpDir;
      makeGitBoundary(cwd);
      expect(findProjectRoot(cwd)).toBe(path.resolve(cwd));
    });

    test('returns dir containing package.json', () => {
      fs.writeFileSync(path.join(tmpDir, 'package.json'), '{}');
      const sub = path.join(tmpDir, 'sub', 'deep');
      fs.mkdirSync(sub, { recursive: true });
      expect(findProjectRoot(sub)).toBe(path.resolve(tmpDir));
    });

    test('returns dir containing ansible project markers', () => {
      fs.writeFileSync(path.join(tmpDir, 'ansible.cfg'), '[defaults]\n');
      const sub = path.join(tmpDir, 'roles', 'novnc');
      fs.mkdirSync(sub, { recursive: true });
      expect(findProjectRoot(sub)).toBe(path.resolve(tmpDir));
    });

    test('returns dir containing ansible requirements.yaml marker', () => {
      fs.writeFileSync(path.join(tmpDir, 'requirements.yaml'), '---\n');
      const sub = path.join(tmpDir, 'roles', 'novnc');
      fs.mkdirSync(sub, { recursive: true });
      expect(findProjectRoot(sub)).toBe(path.resolve(tmpDir));
    });

    test('returns dir containing terraform project markers', () => {
      fs.writeFileSync(path.join(tmpDir, '.terraform-version'), '1.8.5\n');
      fs.writeFileSync(
        path.join(tmpDir, 'versions.tf'),
        'terraform { required_version = "~> 1.8.0" }\n'
      );
      const sub = path.join(tmpDir, 'modules', 'network');
      fs.mkdirSync(sub, { recursive: true });
      expect(findProjectRoot(sub)).toBe(path.resolve(tmpDir));
    });

    test('returns dir containing .rulesrc.json', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({ target: 'cursor', agents: ['linting'] })
      );
      const sub = path.join(tmpDir, 'sub');
      fs.mkdirSync(sub, { recursive: true });
      expect(findProjectRoot(sub)).toBe(path.resolve(tmpDir));
    });

    test('does not cross git repository boundary', () => {
      // parent has a project marker
      fs.writeFileSync(path.join(tmpDir, 'playbook.yml'), '---\n');
      // child is its own git repo with no project markers
      const child = path.join(tmpDir, 'child-project');
      makeGitBoundary(child);
      expect(findProjectRoot(child)).toBe(path.resolve(child));
    });

    test('returns child repo root for nested cwd inside unmarked git repo', () => {
      fs.writeFileSync(path.join(tmpDir, 'playbook.yml'), '---\n');
      const child = path.join(tmpDir, 'child-project');
      const nested = path.join(child, 'subdir');
      makeGitBoundary(child);
      fs.mkdirSync(nested, { recursive: true });
      expect(findProjectRoot(nested)).toBe(path.resolve(child));
    });
  });

  describe('loadConfig', () => {
    test('returns null when file does not exist', () => {
      expect(loadConfig(tmpDir)).toBeNull();
    });

    test('returns parsed config when valid', () => {
      const config = {
        targets: ['claude'] as const,
        agents: ['linting', 'local-dev'],
        skills: ['owasp-security-scan']
      };
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify(config)
      );
      expect(loadConfig(tmpDir)).toEqual(config);
    });

    test('returns ballastVersion when present', () => {
      const config = {
        targets: ['claude'] as const,
        agents: ['linting', 'local-dev'],
        skills: ['owasp-security-scan'],
        ballastVersion: BALLAST_VERSION
      };
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify(config)
      );
      expect(loadConfig(tmpDir)).toEqual(config);
    });

    test('returns null when invalid (missing target)', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({ agents: ['linting'] })
      );
      expect(loadConfig(tmpDir)).toBeNull();
    });

    test('returns null when invalid (agents not array)', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({ target: 'cursor', agents: 'linting' })
      );
      expect(loadConfig(tmpDir)).toBeNull();
    });

    test('reads legacy single-target config as targets array', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({ target: 'cursor', agents: ['linting'] })
      );
      expect(loadConfig(tmpDir)).toEqual({
        targets: ['cursor'],
        agents: ['linting']
      });
    });
  });

  describe('parseTargets', () => {
    test('returns normalized targets and invalid tokens separately', () => {
      expect(parseTargets(['cursor,claude', 'bogus', 'codex'])).toEqual({
        targets: ['cursor', 'claude', 'codex'],
        invalidTargets: ['bogus']
      });
    });
  });

  describe('saveConfig', () => {
    test('writes .rulesrc.json', () => {
      const config = {
        targets: ['opencode' as const],
        agents: ['cicd'],
        skills: ['owasp-security-scan'],
        ballastVersion: BALLAST_VERSION
      };
      saveConfig(config, tmpDir);
      const file = path.join(tmpDir, RULESRC_FILENAME);
      expect(fs.existsSync(file)).toBe(true);
      const parsed = JSON.parse(fs.readFileSync(file, 'utf8'));
      expect(parsed).toEqual(config);
    });

    test('accumulates languages and default paths in shared config', () => {
      saveConfig(
        {
          targets: ['claude'],
          agents: ['linting'],
          ballastVersion: BALLAST_VERSION,
          languages: ['typescript']
        },
        tmpDir
      );
      saveConfig(
        {
          targets: ['claude'],
          agents: ['linting'],
          ballastVersion: BALLAST_VERSION,
          languages: ['go']
        },
        tmpDir
      );

      const parsed = JSON.parse(
        fs.readFileSync(path.join(tmpDir, RULESRC_FILENAME), 'utf8')
      );
      expect(parsed).toEqual({
        targets: ['claude'],
        agents: ['linting'],
        ballastVersion: BALLAST_VERSION,
        languages: ['typescript', 'go'],
        paths: {
          typescript: ['.'],
          go: ['.']
        }
      });
    });

    test('loads legacy .rulesrc.ts.json for typescript', () => {
      const config = { target: 'cursor' as const, agents: ['linting'] };
      fs.writeFileSync(
        path.join(tmpDir, getLegacyRulesrcFilename('typescript')),
        JSON.stringify(config)
      );
      expect(loadConfig(tmpDir)).toEqual({
        targets: ['cursor'],
        agents: ['linting']
      });
    });

    test('loads legacy language-specific rulesrc for python', () => {
      const config = { target: 'opencode' as const, agents: ['linting'] };
      fs.writeFileSync(
        path.join(tmpDir, getLegacyRulesrcFilename('python')),
        JSON.stringify(config)
      );
      expect(loadConfig(tmpDir, 'python')).toEqual({
        targets: ['opencode'],
        agents: ['linting']
      });
    });

    test('normalizes saved config to targets array', () => {
      saveConfig(
        {
          targets: ['cursor', 'claude'],
          agents: ['linting'],
          ballastVersion: BALLAST_VERSION
        },
        tmpDir
      );

      const parsed = JSON.parse(
        fs.readFileSync(path.join(tmpDir, RULESRC_FILENAME), 'utf8')
      );
      expect(parsed.targets).toEqual(['cursor', 'claude']);
      expect(parsed.target).toBeUndefined();
    });
  });

  describe('taskSystem', () => {
    test('TASK_SYSTEMS contains github, jira, linear', () => {
      expect(TASK_SYSTEMS).toContain('github');
      expect(TASK_SYSTEMS).toContain('jira');
      expect(TASK_SYSTEMS).toContain('linear');
    });

    test('DEFAULT_TASK_SYSTEM is github', () => {
      expect(DEFAULT_TASK_SYSTEM).toBe('github');
    });

    test('saves and loads taskSystem', () => {
      saveConfig(
        {
          targets: ['claude'],
          agents: ['tasks'],
          ballastVersion: BALLAST_VERSION,
          taskSystem: 'linear'
        },
        tmpDir
      );
      const loaded = loadConfig(tmpDir);
      expect(loaded?.taskSystem).toBe('linear');
    });

    test('loads config without taskSystem field', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({ targets: ['claude'], agents: ['local-dev'] })
      );
      const loaded = loadConfig(tmpDir);
      expect(loaded?.taskSystem).toBeUndefined();
    });

    test('ignores invalid taskSystem value', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({
          targets: ['claude'],
          agents: ['tasks'],
          taskSystem: 'notion'
        })
      );
      const loaded = loadConfig(tmpDir);
      expect(loaded?.taskSystem).toBeUndefined();
    });

    test('preserves existing taskSystem when saving without one', () => {
      saveConfig(
        { targets: ['claude'], agents: ['tasks'], taskSystem: 'jira' },
        tmpDir
      );
      saveConfig({ targets: ['cursor'], agents: ['tasks'] }, tmpDir);
      const loaded = loadConfig(tmpDir);
      expect(loaded?.taskSystem).toBe('jira');
    });
  });

  describe('deploymentModel', () => {
    test('DEPLOYMENT_MODELS contains supported deployment models', () => {
      expect(DEPLOYMENT_MODELS).toEqual([
        'none',
        'kubernetes',
        'serverless',
        'server',
        'hosted'
      ]);
    });

    test('DEFAULT_DEPLOYMENT_MODEL is none', () => {
      expect(DEFAULT_DEPLOYMENT_MODEL).toBe('none');
    });

    test('saves and loads deploymentModel', () => {
      saveConfig(
        {
          targets: ['claude'],
          agents: ['publishing'],
          ballastVersion: BALLAST_VERSION,
          deploymentModel: 'kubernetes'
        },
        tmpDir
      );
      const loaded = loadConfig(tmpDir);
      expect(loaded?.deploymentModel).toBe('kubernetes');
    });

    test('loads config without deploymentModel field', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({ targets: ['claude'], agents: ['local-dev'] })
      );
      const loaded = loadConfig(tmpDir);
      expect(loaded?.deploymentModel).toBeUndefined();
    });

    test('ignores invalid deploymentModel value', () => {
      fs.writeFileSync(
        path.join(tmpDir, RULESRC_FILENAME),
        JSON.stringify({
          targets: ['claude'],
          agents: ['publishing'],
          deploymentModel: 'kuberntes'
        })
      );
      const loaded = loadConfig(tmpDir);
      expect(loaded?.deploymentModel).toBeUndefined();
    });

    test('preserves existing deploymentModel when saving without one', () => {
      saveConfig(
        {
          targets: ['claude'],
          agents: ['publishing'],
          deploymentModel: 'hosted'
        },
        tmpDir
      );
      saveConfig({ targets: ['cursor'], agents: ['publishing'] }, tmpDir);
      const loaded = loadConfig(tmpDir);
      expect(loaded?.deploymentModel).toBe('hosted');
    });
  });

  describe('isCiMode', () => {
    const orig = process.env;

    afterEach(() => {
      process.env = { ...orig };
    });

    test('returns true when CI=true', () => {
      process.env.CI = 'true';
      expect(isCiMode()).toBe(true);
    });

    test('returns true when GITHUB_ACTIONS=true', () => {
      process.env.CI = undefined;
      process.env.GITHUB_ACTIONS = 'true';
      expect(isCiMode()).toBe(true);
    });

    test('returns false when no CI env', () => {
      delete process.env.CI;
      delete process.env.GITHUB_ACTIONS;
      delete process.env.GITLAB_CI;
      delete process.env.TF_BUILD;
      expect(isCiMode()).toBe(false);
    });
  });
});
