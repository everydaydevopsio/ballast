import fs from 'fs';
import path from 'path';
import os from 'os';
import {
  findProjectRoot,
  loadConfig,
  saveConfig,
  isCiMode,
  RULESRC_FILENAME,
  getLegacyRulesrcFilename
} from './config';

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
      expect(findProjectRoot(cwd)).toBe(path.resolve(cwd));
    });

    test('returns dir containing package.json', () => {
      fs.writeFileSync(path.join(tmpDir, 'package.json'), '{}');
      const sub = path.join(tmpDir, 'sub', 'deep');
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
  });

  describe('loadConfig', () => {
    test('returns null when file does not exist', () => {
      expect(loadConfig(tmpDir)).toBeNull();
    });

    test('returns parsed config when valid', () => {
      const config = {
        target: 'claude' as const,
        agents: ['linting', 'local-dev']
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
  });

  describe('saveConfig', () => {
    test('writes .rulesrc.json', () => {
      const config = { target: 'opencode' as const, agents: ['cicd'] };
      saveConfig(config, tmpDir);
      const file = path.join(tmpDir, RULESRC_FILENAME);
      expect(fs.existsSync(file)).toBe(true);
      const parsed = JSON.parse(fs.readFileSync(file, 'utf8'));
      expect(parsed).toEqual(config);
    });

    test('accumulates languages and default paths in shared config', () => {
      saveConfig(
        {
          target: 'claude',
          agents: ['linting'],
          languages: ['typescript']
        },
        tmpDir
      );
      saveConfig(
        {
          target: 'claude',
          agents: ['linting'],
          languages: ['go']
        },
        tmpDir
      );

      const parsed = JSON.parse(
        fs.readFileSync(path.join(tmpDir, RULESRC_FILENAME), 'utf8')
      );
      expect(parsed).toEqual({
        target: 'claude',
        agents: ['linting'],
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
      expect(loadConfig(tmpDir)).toEqual(config);
    });

    test('loads legacy language-specific rulesrc for python', () => {
      const config = { target: 'opencode' as const, agents: ['linting'] };
      fs.writeFileSync(
        path.join(tmpDir, getLegacyRulesrcFilename('python')),
        JSON.stringify(config)
      );
      expect(loadConfig(tmpDir, 'python')).toEqual(config);
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
