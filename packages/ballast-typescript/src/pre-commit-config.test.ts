import fs from 'fs';
import path from 'path';
import YAML from 'yaml';

type PreCommitHook = {
  id?: string;
  repo?: string;
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
  test('uses non-mutating whitespace checks', () => {
    const config = readPreCommitConfig();
    const hookIds = config.repos.flatMap((repo) =>
      (repo.hooks ?? []).map((hook) => hook.id).filter(Boolean)
    );

    expect(hookIds).not.toContain('trailing-whitespace');
    expect(hookIds).not.toContain('end-of-file-fixer');
    expect(hookIds).toContain('check-trailing-whitespace');
    expect(hookIds).toContain('check-end-of-file-newline');
  });
});
