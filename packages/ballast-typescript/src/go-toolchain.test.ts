import fs from 'fs';
import path from 'path';

// Issue #339: the golang.org/x/* family requires Go >= 1.26, so every Go module
// and every pinned setup-go version in CI must stay on one supported toolchain.
const MINIMUM_GO_VERSION = [1, 26] as const;

const repoRoot = path.resolve(__dirname, '../../..');

const GO_MODULES = ['packages/ballast-go/go.mod', 'cli/ballast/go.mod'];

function readRepoFile(relativePath: string): string {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

function parseGoDirective(relativePath: string): number[] {
  const match = readRepoFile(relativePath).match(/^go\s+(\d+(?:\.\d+)+)\s*$/m);
  if (!match) {
    throw new Error(`No go directive found in ${relativePath}`);
  }
  return match[1].split('.').map(Number);
}

function isAtLeastMinimum(version: number[]): boolean {
  const [major, minor] = version;
  const [minMajor, minMinor] = MINIMUM_GO_VERSION;
  return major > minMajor || (major === minMajor && minor >= minMinor);
}

function workflowFiles(): string[] {
  const workflowDir = path.join(repoRoot, '.github/workflows');
  return fs
    .readdirSync(workflowDir)
    .filter((entry) => entry.endsWith('.yml') || entry.endsWith('.yaml'))
    .map((entry) => path.join('.github/workflows', entry));
}

describe('Go toolchain pins', () => {
  test('every Go module requires at least the supported toolchain', () => {
    for (const modulePath of GO_MODULES) {
      const version = parseGoDirective(modulePath);
      expect({ modulePath, ok: isAtLeastMinimum(version) }).toEqual({
        modulePath,
        ok: true
      });
    }
  });

  test('all Go modules declare the same go directive', () => {
    const directives = GO_MODULES.map((modulePath) =>
      parseGoDirective(modulePath).join('.')
    );
    expect(new Set(directives).size).toBe(1);
  });

  test('pinned setup-go versions in workflows match the module toolchain', () => {
    const [major, minor] = parseGoDirective(GO_MODULES[0]);
    const expectedPin = `${major}.${minor}.x`;
    const offenders: string[] = [];

    for (const workflowPath of workflowFiles()) {
      const content = readRepoFile(workflowPath);
      for (const match of content.matchAll(/go-version:\s*'([^']+)'/g)) {
        if (match[1] !== expectedPin) {
          offenders.push(`${workflowPath}: ${match[1]}`);
        }
      }
    }

    expect(offenders).toEqual([]);
  });
});
