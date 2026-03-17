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
});
