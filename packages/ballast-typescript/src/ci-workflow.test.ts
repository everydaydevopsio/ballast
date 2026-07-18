import fs from 'fs';
import path from 'path';
import YAML from 'yaml';

type WorkflowConfig = {
  name?: string;
  on?: {
    push?: {
      branches?: string[];
    };
    pull_request?: {
      branches?: string[];
    };
  };
  concurrency?: {
    group?: string;
    'cancel-in-progress'?: boolean;
  };
  jobs?: Record<string, unknown>;
};

const repoRoot = path.resolve(__dirname, '../../..');

function readRepoFile(relativePath: string): string {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

function readWorkflowConfig(relativePath: string): WorkflowConfig {
  return YAML.parse(readRepoFile(relativePath)) as WorkflowConfig;
}

describe('CI workflow', () => {
  test('primary CI is consolidated into one parallel workflow', () => {
    const workflowPath = '.github/workflows/ci.yml';
    const workflow = readWorkflowConfig(workflowPath);
    const workflowContent = readRepoFile(workflowPath);
    const readme = readRepoFile('README.md');
    const jobs = workflow.jobs ?? {};

    expect(workflow.name).toBe('CI');
    expect(workflow.on?.push?.branches).toContain('main');
    expect(workflow.on?.pull_request?.branches).toContain('main');
    expect(workflow.concurrency?.group).toBe(
      '${{ github.workflow }}-${{ github.ref }}'
    );
    expect(workflow.concurrency?.['cancel-in-progress']).toBe(true);

    for (const jobName of [
      'typescript-lint',
      'typescript-tests',
      'typescript-coverage',
      'python-lint',
      'python-tests',
      'python-package',
      'go-pack-lint',
      'go-pack-tests',
      'go-package',
      'cli-lint',
      'cli-tests',
      'cli-package'
    ]) {
      expect(Object.prototype.hasOwnProperty.call(jobs, jobName)).toBe(true);
    }

    expect(workflowContent).toContain(
      'node-version: ${{ matrix.node-version }}'
    );
    expect(workflowContent).toContain(
      'working-directory: packages/ballast-python'
    );
    expect(workflowContent).toContain(
      'python -c "import ballast.cli; import ballast.__main__"'
    );
    expect(readme).toContain('actions/workflows/ci.yml/badge.svg');
    expect(
      fs.existsSync(path.join(repoRoot, '.github/workflows/lint.yaml'))
    ).toBe(false);
    expect(
      fs.existsSync(path.join(repoRoot, '.github/workflows/test.yml'))
    ).toBe(false);
    expect(
      fs.existsSync(path.join(repoRoot, '.github/workflows/language-packs.yml'))
    ).toBe(false);
  });
});
