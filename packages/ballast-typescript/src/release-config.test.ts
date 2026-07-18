import fs from 'fs';
import path from 'path';
import YAML from 'yaml';

type GoReleaserConfig = {
  checksum?: {
    name_template?: string;
  };
};

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

function readGoReleaserConfig(relativePath: string): GoReleaserConfig {
  const configPath = path.join(repoRoot, relativePath);
  const raw = fs.readFileSync(configPath, 'utf8');
  return YAML.parse(raw) as GoReleaserConfig;
}

function readRepoFile(relativePath: string): string {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

function readWorkflowConfig(relativePath: string): WorkflowConfig {
  return YAML.parse(readRepoFile(relativePath)) as WorkflowConfig;
}

describe('release config', () => {
  test('Go and CLI releases publish distinct checksum asset names', () => {
    const goConfig = readGoReleaserConfig(
      'packages/ballast-go/.goreleaser.yaml'
    );
    const cliConfig = readGoReleaserConfig('cli/ballast/.goreleaser.yaml');

    expect(goConfig.checksum?.name_template).toBeTruthy();
    expect(cliConfig.checksum?.name_template).toBeTruthy();
    expect(goConfig.checksum?.name_template).not.toBe(
      cliConfig.checksum?.name_template
    );
  });

  test('publish workflows pin GoReleaser to an explicit stable release', () => {
    for (const workflowPath of [
      '.github/workflows/publish.yml',
      '.github/workflows/publish-go.yml',
      '.github/workflows/publish-cli.yml'
    ]) {
      const workflow = readRepoFile(workflowPath);
      const goreleaserUses =
        workflow.match(/uses:\s*goreleaser\/goreleaser-action@[^ \n]+/g) ?? [];
      const pinnedVersions = workflow.match(/version:\s*'v2\.14\.0'/g) ?? [];

      expect(goreleaserUses.length).toBeGreaterThan(0);
      expect(workflow).not.toContain("version: '~> v2'");
      expect(pinnedVersions.length).toBe(goreleaserUses.length);
    }
  });

  test('primary CI is consolidated into one parallel workflow', () => {
    const workflow = readWorkflowConfig('.github/workflows/ci.yml');
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

    expect(readRepoFile('.github/workflows/ci.yml')).toContain(
      'node-version: ${{ matrix.node-version }}'
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
