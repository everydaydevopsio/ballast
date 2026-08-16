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
