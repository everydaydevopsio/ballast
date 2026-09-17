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
  test('source builds of the Go backend carry a version stamp', () => {
    // The wrapper prefers a `ballast-go` sitting next to it over the installer
    // path, so an unstamped source build emits rules marked `dev`. The wrapper
    // itself must NOT be stamped: a wrapper reporting a release version
    // installs published backends instead of building from the tree.
    const sources: Array<{ path: string; content: string }> = [
      { path: 'Makefile', content: readRepoFile('Makefile') },
      ...fs
        .readdirSync(path.join(repoRoot, '.github/workflows'))
        .filter((entry) => entry.endsWith('.yml') || entry.endsWith('.yaml'))
        .map((entry) => {
          const relative = `.github/workflows/${entry}`;
          return { path: relative, content: readRepoFile(relative) };
        })
    ];

    const unstamped: string[] = [];
    for (const source of sources) {
      for (const line of source.content.split('\n')) {
        if (!/go build\b/.test(line)) continue;
        if (!/ballast-go/.test(line)) continue;
        // Verification builds discard their output or drop it in /tmp; they
        // never become the backend the wrapper runs, so a stamp is moot.
        const output = line.match(/-o\s+("?)([^\s"]+)\1/);
        if (!output || output[2].includes('/tmp/')) continue;
        if (!line.includes('-X main.ballastVersion=')) {
          unstamped.push(`${source.path}: ${line.trim()}`);
        }
      }
    }

    expect(unstamped).toEqual([]);
  });

  test('the cross-language gate is wired into every publishing workflow', () => {
    const gate = '.github/workflows/cross-language-validate.yml';
    const publishing = fs
      .readdirSync(path.join(repoRoot, '.github/workflows'))
      .filter((entry) => entry.startsWith('publish'))
      .map((entry) => `.github/workflows/${entry}`);

    expect(publishing.length).toBeGreaterThan(0);
    const missing = publishing.filter(
      (workflowPath) =>
        !readRepoFile(workflowPath).includes(
          gate.replace('.github/workflows/', '')
        )
    );

    expect(missing).toEqual([]);
  });
});
