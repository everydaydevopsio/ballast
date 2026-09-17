import fs from 'fs';
import path from 'path';
import YAML from 'yaml';

// Issue #339: the golang.org/x/* family requires Go >= 1.26, so every Ballast
// Go module, every pinned setup-go version, and every golang Docker base image
// must stay on one supported toolchain.
const MINIMUM_GO_VERSION = [1, 26] as const;

const repoRoot = path.resolve(__dirname, '../../..');

// Sample projects under examples/ exist only so language detection sees a Go
// project; nothing builds them against the Ballast toolchain, so they may
// declare an older directive. Listing them explicitly means a NEW module is a
// production module until someone deliberately classifies it as a fixture.
const FIXTURE_MODULES = ['examples/smoke/go-sample/go.mod'];

const SKIP_DIRS = new Set([
  'node_modules',
  '.git',
  'dist',
  '.venv',
  'coverage'
]);

function readRepoFile(relativePath: string): string {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

function discoverGoModules(): string[] {
  const found: string[] = [];
  const walk = (dir: string): void => {
    for (const entry of fs.readdirSync(path.join(repoRoot, dir || '.'), {
      withFileTypes: true
    })) {
      const relative = dir ? path.posix.join(dir, entry.name) : entry.name;
      if (entry.isDirectory()) {
        if (!SKIP_DIRS.has(entry.name)) walk(relative);
      } else if (entry.name === 'go.mod') {
        found.push(relative);
      }
    }
  };
  walk('');
  return found.sort();
}

function productionModules(): string[] {
  return discoverGoModules().filter(
    (modulePath) => !FIXTURE_MODULES.includes(modulePath)
  );
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
    .map((entry) => path.posix.join('.github/workflows', entry));
}

function dockerfiles(): string[] {
  return fs
    .readdirSync(repoRoot)
    .filter((entry) => entry.startsWith('Dockerfile'))
    .filter((entry) => fs.statSync(path.join(repoRoot, entry)).isFile());
}

function collectGoVersionPins(node: unknown): string[] {
  const pins: string[] = [];
  const visit = (value: unknown): void => {
    if (Array.isArray(value)) {
      value.forEach(visit);
      return;
    }
    if (value === null || typeof value !== 'object') return;
    for (const [key, child] of Object.entries(
      value as Record<string, unknown>
    )) {
      // `go-version-file` points at a go.mod and needs no pin comparison.
      if (
        key === 'go-version' &&
        (typeof child === 'string' || typeof child === 'number')
      ) {
        pins.push(String(child));
      } else {
        visit(child);
      }
    }
  };
  visit(node);
  return pins;
}

describe('Go toolchain pins', () => {
  test('every discovered Go module is classified as production or fixture', () => {
    // Guards the fixture allowlist itself: a stale entry would silently exempt
    // nothing, and an unlisted new module correctly falls through to production.
    const discovered = discoverGoModules();
    expect(discovered.length).toBeGreaterThan(0);
    for (const fixture of FIXTURE_MODULES) {
      expect(discovered).toContain(fixture);
    }
    expect(productionModules().length).toBeGreaterThan(0);
  });

  test('every production Go module requires at least the supported toolchain', () => {
    const offenders = productionModules().filter(
      (modulePath) => !isAtLeastMinimum(parseGoDirective(modulePath))
    );
    expect(offenders).toEqual([]);
  });

  test('all production Go modules declare the same go directive', () => {
    const directives = productionModules().map((modulePath) =>
      parseGoDirective(modulePath).join('.')
    );
    expect(new Set(directives).size).toBe(1);
  });

  test('pinned setup-go versions in workflows match the module toolchain', () => {
    const [major, minor] = parseGoDirective(productionModules()[0]);
    const expectedPin = `${major}.${minor}.x`;
    const offenders: string[] = [];

    for (const workflowPath of workflowFiles()) {
      // Parse the YAML rather than the raw text: quoting is the author's
      // choice, and `go-version: 1.26` unquoted even parses as a number.
      for (const pin of collectGoVersionPins(
        YAML.parse(readRepoFile(workflowPath))
      )) {
        if (pin !== expectedPin) {
          offenders.push(`${workflowPath}: ${pin}`);
        }
      }
    }

    expect(offenders).toEqual([]);
  });

  test('golang Docker base images match the module toolchain', () => {
    const [major, minor] = parseGoDirective(productionModules()[0]);
    const expectedSeries = `${major}.${minor}`;
    const offenders: string[] = [];

    for (const dockerfile of dockerfiles()) {
      const content = readRepoFile(dockerfile);
      for (const match of content.matchAll(/^FROM\s+golang:([^\s-]+)/gm)) {
        if (match[1] !== expectedSeries) {
          offenders.push(`${dockerfile}: golang:${match[1]}`);
        }
      }
    }

    expect(offenders).toEqual([]);
  });
});
