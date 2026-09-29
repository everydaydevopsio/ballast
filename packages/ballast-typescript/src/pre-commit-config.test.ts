import fs from 'fs';
import path from 'path';
import YAML from 'yaml';

type PreCommitHook = {
  id?: string;
  entry?: string;
  stages?: string[];
};

type PreCommitRepo = {
  repo: string;
  hooks?: PreCommitHook[];
};

type PreCommitConfig = {
  repos: PreCommitRepo[];
};

const repoRoot = path.resolve(__dirname, '../../..');

function readPreCommitConfig(): PreCommitConfig {
  const content = fs.readFileSync(
    path.join(repoRoot, '.pre-commit-config.yaml'),
    'utf8'
  );
  return YAML.parse(content) as PreCommitConfig;
}

describe('root pre-commit config', () => {
  test('uses auto-fixing whitespace hooks', () => {
    const config = readPreCommitConfig();
    const hookIds = config.repos.flatMap((repo) =>
      (repo.hooks ?? []).map((hook) => hook.id).filter(Boolean)
    );

    expect(hookIds).toContain('trailing-whitespace');
    expect(hookIds).toContain('end-of-file-fixer');
  });

  test('runs all package and cli unit tests at pre-push', () => {
    const config = readPreCommitConfig();
    const hooks = config.repos.flatMap((repo) => repo.hooks ?? []);
    const unitTestHook = hooks.find(
      (hook) => hook.id === 'ballast-unit-tests-pre-push'
    );

    expect(unitTestHook).toBeTruthy();
    expect(unitTestHook?.entry).toBe('scripts/run-unit-tests-pre-push.sh');
    expect(unitTestHook?.stages).toContain('pre-push');
  });

  test('no second hook system can hijack core.hooksPath', () => {
    // `.husky/` plus `"prepare": "husky"` in the TypeScript package set
    // core.hooksPath to .husky/_, so git ran husky's `npx lint-staged` and
    // never ran this config at all -- gitleaks and detect-private-key included.
    // The husky hook then failed to spawn tsc-files from the repo root, so
    // every TypeScript commit was made with --no-verify, which is how the
    // suppression stayed invisible. pre-commit owns the hooks; keep it sole.
    expect(fs.existsSync(path.join(repoRoot, '.husky'))).toBe(false);

    const manifests = [
      'package.json',
      'packages/ballast-typescript/package.json',
      'packages/ballast-python/package.json',
      'packages/ballast-go/package.json'
    ].filter((rel) => fs.existsSync(path.join(repoRoot, rel)));

    for (const rel of manifests) {
      const manifest = JSON.parse(
        fs.readFileSync(path.join(repoRoot, rel), 'utf8')
      ) as {
        scripts?: Record<string, string>;
        devDependencies?: Record<string, string>;
        'lint-staged'?: unknown;
      };

      expect({
        file: rel,
        prepare: manifest.scripts?.prepare ?? null,
        husky: manifest.devDependencies?.husky ?? null,
        lintStaged: manifest['lint-staged'] ?? null
      }).toEqual({
        file: rel,
        prepare: null,
        husky: null,
        lintStaged: null
      });
    }
  });

  test('declares gitleaks as a pre-commit hook instead of a local script', () => {
    const config = readPreCommitConfig();
    const gitleaksRepo = config.repos.find(
      (repo) => repo.repo === 'https://github.com/gitleaks/gitleaks'
    );
    const hooks = config.repos.flatMap((repo) => repo.hooks ?? []);

    expect(gitleaksRepo).toBeTruthy();
    expect(gitleaksRepo?.hooks?.some((hook) => hook.id === 'gitleaks')).toBe(
      true
    );
    expect(
      hooks.some((hook) => hook.entry === 'scripts/check-no-secrets.sh')
    ).toBe(false);
  });
});
