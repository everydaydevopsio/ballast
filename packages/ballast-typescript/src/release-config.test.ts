import fs from 'fs';
import path from 'path';
import YAML from 'yaml';

type GoReleaserConfig = {
  checksum?: {
    name_template?: string;
  };
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

      expect(workflow).not.toContain("version: '~> v2'");
      expect(workflow).toContain("version: 'v2.14.0'");
    }
  });
});
