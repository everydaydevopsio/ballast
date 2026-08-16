import fs from 'fs';
import os from 'os';
import path from 'path';
import { buildContent } from './build';
import {
  buildDoctorReport,
  collectRuleFileStatuses,
  detectAppType,
  formatDoctorReport,
  removeStaleRuleFiles,
  runDoctor
} from './doctor';

describe('doctor', () => {
  test('formats rule file status categories and remediation recommendations', () => {
    const report = buildDoctorReport(
      'ballast-typescript',
      '5.0.2',
      '/tmp/project/.rulesrc.json',
      '5.0.2',
      ['codex'],
      ['linting'],
      [],
      ['typescript'],
      {},
      {},
      [],
      null,
      null,
      [],
      [{ name: 'ballast-typescript', version: '5.0.2', path: '/tmp/bt' }],
      'unknown',
      [
        {
          path: '/tmp/project/.codex/rules/typescript-linting.md',
          target: 'codex',
          ruleId: 'typescript/linting',
          status: 'ok'
        },
        {
          path: '/tmp/project/.codex/rules/typescript-testing.md',
          target: 'codex',
          ruleId: 'typescript/testing',
          status: 'drifted'
        },
        {
          path: '/tmp/project/.codex/rules/typescript-docs.md',
          target: 'codex',
          ruleId: 'typescript/docs',
          status: 'stale'
        },
        {
          path: '/tmp/project/.codex/rules/manual.md',
          target: 'codex',
          ruleId: null,
          status: 'unowned'
        }
      ]
    );

    const output = formatDoctorReport(report);

    expect(output).toContain('Rule files:');
    expect(output).toContain('.codex/rules/typescript-linting.md [ok]');
    expect(output).toContain(
      '.codex/rules/typescript-testing.md [DRIFTED - run ballast install --refresh-config to restore]'
    );
    expect(output).toContain(
      '.codex/rules/typescript-docs.md [STALE - run ballast doctor --fix to remove]'
    );
    expect(output).toContain(
      '.codex/rules/manual.md [unowned - not managed by Ballast, skipped]'
    );
    expect(report.recommendations).toEqual(
      expect.arrayContaining([
        expect.stringContaining('Refresh drifted rule file'),
        expect.stringContaining('Remove stale managed rule file')
      ])
    );
  });

  test('recommends upgrades for older CLIs and config', () => {
    const report = buildDoctorReport(
      'ballast-typescript',
      '5.0.2',
      '/tmp/project/.rulesrc.json',
      '5.0.1',
      ['cursor'],
      ['linting', 'testing'],
      [],
      [],
      {},
      {},
      [],
      null,
      null,
      [],
      [
        {
          name: 'ballast-typescript',
          version: '5.0.2',
          path: '/tmp/ballast-typescript'
        },
        {
          name: 'ballast-python',
          version: '5.0.1',
          path: '/tmp/ballast-python'
        },
        { name: 'ballast-go', version: null, path: null }
      ]
    );

    expect(report.recommendations).toEqual(
      expect.arrayContaining([
        expect.stringContaining(
          'Run ballast doctor --fix to install or upgrade local Ballast CLIs.'
        ),
        expect.stringContaining(
          'Refresh .rulesrc.json to Ballast 5.0.2: ballast install --refresh-config'
        )
      ])
    );
  });

  test('formats clean reports without recommendations', () => {
    const report = buildDoctorReport(
      'ballast-typescript',
      '5.0.2',
      '/tmp/project/.rulesrc.json',
      '5.0.2',
      ['cursor'],
      ['linting'],
      ['owasp-security-scan'],
      ['typescript', 'ansible'],
      {
        typescript: ['apps/web'],
        ansible: ['infra/ansible']
      },
      {
        typescript: ['pnpm', 'corepack'],
        ansible: ['ansible-lint', 'molecule']
      },
      ['examples', 'tmp'],
      'jira',
      'hosted',
      ['cli', 'web'],
      [
        {
          name: 'ballast-typescript',
          version: '5.0.2',
          path: '/tmp/ballast-typescript'
        },
        {
          name: 'ballast-python',
          version: '5.0.2',
          path: '/tmp/ballast-python'
        },
        { name: 'ballast-go', version: '5.0.2', path: '/tmp/ballast-go' }
      ]
    );
    const output = formatDoctorReport(report);

    expect(output).toContain('Ballast doctor');
    expect(output).toContain('- targets: cursor');
    expect(output).toContain('Recommendations:');
    expect(output).toContain('- skills: owasp-security-scan');
    expect(output).toContain('- languages: typescript, ansible');
    expect(output).toContain(
      '- paths: typescript=apps/web; ansible=infra/ansible'
    );
    expect(output).toContain(
      '- tools: typescript=pnpm,corepack; ansible=ansible-lint,molecule'
    );
    expect(output).toContain('- discovery.excludePaths: examples,tmp');
    expect(output).toContain('- taskSystem: jira');
    expect(output).toContain('- deploymentModel: hosted');
    expect(output).toContain('- publishingProfiles: cli, web');
    expect(output).toContain('- No action needed.');
  });

  test('includes detected app type in formatted output when known', () => {
    const report = buildDoctorReport(
      'ballast-typescript',
      '5.0.2',
      '/tmp/project/.rulesrc.json',
      '5.0.2',
      ['claude'],
      ['publishing'],
      [],
      [],
      {},
      {},
      [],
      null,
      'kubernetes',
      [],
      [{ name: 'ballast-typescript', version: '5.0.2', path: '/tmp/bt' }],
      'cli'
    );
    const output = formatDoctorReport(report);
    expect(output).toContain('Detected app type: cli');
    expect(output).toContain('- deploymentModel: kubernetes');
  });

  test('omits detected app type line when unknown', () => {
    const report = buildDoctorReport(
      'ballast-typescript',
      '5.0.2',
      null,
      null,
      [],
      [],
      [],
      [],
      {},
      {},
      [],
      null,
      null,
      [],
      [],
      'unknown'
    );
    const output = formatDoctorReport(report);
    expect(output).not.toContain('Detected app type');
  });

  test('recommends adding publishing agent when app type known and agent missing', () => {
    const report = buildDoctorReport(
      'ballast-typescript',
      '5.0.2',
      '/tmp/project/.rulesrc.json',
      '5.0.2',
      ['claude'],
      ['linting'],
      [],
      [],
      {},
      {},
      [],
      null,
      null,
      [],
      [{ name: 'ballast-typescript', version: '5.0.2', path: '/tmp/bt' }],
      'web'
    );
    expect(report.recommendations).toEqual(
      expect.arrayContaining([
        expect.stringContaining('add the publishing agent')
      ])
    );
  });

  test('does not recommend publishing agent when already installed', () => {
    const report = buildDoctorReport(
      'ballast-typescript',
      '5.0.2',
      '/tmp/project/.rulesrc.json',
      '5.0.2',
      ['claude'],
      ['publishing'],
      [],
      [],
      {},
      {},
      [],
      null,
      null,
      [],
      [{ name: 'ballast-typescript', version: '5.0.2', path: '/tmp/bt' }],
      'api'
    );
    const publishingRecs = report.recommendations.filter((r) =>
      r.includes('add the publishing agent')
    );
    expect(publishingRecs).toHaveLength(0);
  });
});

describe('rule file status collection', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ballast-rule-status-'));
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  test('categorizes ok, drifted, stale, and unowned rule files', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });
    fs.writeFileSync(
      path.join(codexRules, 'typescript-linting.md'),
      buildContent('linting', 'codex', undefined, 'typescript'),
      'utf8'
    );
    fs.writeFileSync(
      path.join(codexRules, 'typescript-testing.md'),
      `${buildContent('testing', 'codex', undefined, 'typescript')}\nManual edit\n`,
      'utf8'
    );
    fs.writeFileSync(
      path.join(codexRules, 'typescript-logging.md'),
      buildContent('logging', 'codex', undefined, 'typescript'),
      'utf8'
    );
    fs.writeFileSync(
      path.join(codexRules, 'manual.md'),
      '# Manual notes\n',
      'utf8'
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['linting', 'testing'],
      languages: ['typescript'],
      paths: {}
    });

    expect(
      statuses.map((status) => ({
        basename: path.basename(status.path),
        ruleId: status.ruleId,
        status: status.status
      }))
    ).toEqual(
      expect.arrayContaining([
        {
          basename: 'typescript-linting.md',
          ruleId: 'typescript/linting',
          status: 'ok'
        },
        {
          basename: 'typescript-testing.md',
          ruleId: 'typescript/testing',
          status: 'drifted'
        },
        {
          basename: 'typescript-logging.md',
          ruleId: 'typescript/logging',
          status: 'stale'
        },
        { basename: 'manual.md', ruleId: null, status: 'unowned' }
      ])
    );
  });

  test('removeStaleRuleFiles removes only stale managed files', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });
    const stale = path.join(codexRules, 'typescript-logging.md');
    const drifted = path.join(codexRules, 'typescript-testing.md');
    const unowned = path.join(codexRules, 'manual.md');
    fs.writeFileSync(
      stale,
      buildContent('logging', 'codex', undefined, 'typescript'),
      'utf8'
    );
    fs.writeFileSync(
      drifted,
      `${buildContent('testing', 'codex', undefined, 'typescript')}\nManual edit\n`,
      'utf8'
    );
    fs.writeFileSync(unowned, '# Manual notes\n', 'utf8');

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['testing'],
      languages: ['typescript'],
      paths: {}
    });
    const removed = removeStaleRuleFiles(statuses);

    expect(removed).toEqual([stale]);
    expect(fs.existsSync(stale)).toBe(false);
    expect(fs.existsSync(drifted)).toBe(true);
    expect(fs.existsSync(unowned)).toBe(true);
  });

  test('treats copied rule marker examples in body content as unowned', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });
    const unowned = path.join(codexRules, 'manual.md');
    fs.writeFileSync(
      unowned,
      [
        '# Manual notes',
        '',
        'Example managed marker:',
        '<!-- ballast:rule id="typescript/logging" version="5.0.0" checksum="0123456789abcdef" -->',
        ''
      ].join('\n'),
      'utf8'
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['testing'],
      languages: ['typescript'],
      paths: {}
    });
    const removed = removeStaleRuleFiles(statuses);

    expect(statuses).toEqual([
      expect.objectContaining({
        path: unowned,
        ruleId: null,
        status: 'unowned'
      })
    ]);
    expect(removed).toEqual([]);
    expect(fs.existsSync(unowned)).toBe(true);
  });

  test('uses TypeScript-only hook mode when comparing git-hooks content', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });
    fs.writeFileSync(
      path.join(codexRules, 'git-hooks.md'),
      buildContent('git-hooks', 'codex', undefined, 'typescript', {
        hookMode: 'husky'
      }),
      'utf8'
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['git-hooks'],
      languages: ['typescript'],
      paths: { typescript: ['apps/web'] }
    });

    expect(statuses).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          ruleId: 'typescript/git-hooks',
          status: 'ok'
        })
      ])
    );
  });

  test('uses multi-path hook mode when comparing git-hooks content', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });
    fs.writeFileSync(
      path.join(codexRules, 'git-hooks.md'),
      buildContent('git-hooks', 'codex', undefined, 'typescript', {
        hookMode: 'pre-commit'
      }),
      'utf8'
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['git-hooks'],
      languages: ['typescript'],
      paths: { typescript: ['apps/web'], python: ['packages/api'] }
    });

    expect(statuses).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          ruleId: 'typescript/git-hooks',
          status: 'ok'
        })
      ])
    );
  });

  test('doctor fix does not delete stale managed rules without a loaded config', () => {
    const previousCwd = process.cwd();
    process.chdir(tmpDir);
    try {
      fs.writeFileSync(path.join(tmpDir, 'package.json'), '{}\n', 'utf8');
      fs.writeFileSync(path.join(tmpDir, '.rulesrc.json'), '{not json', 'utf8');
      const codexRules = path.join(tmpDir, '.codex', 'rules');
      fs.mkdirSync(codexRules, { recursive: true });
      const managedRule = path.join(codexRules, 'typescript-linting.md');
      fs.writeFileSync(
        managedRule,
        buildContent('linting', 'codex', undefined, 'typescript'),
        'utf8'
      );

      runDoctor({ fix: true });

      expect(fs.existsSync(managedRule)).toBe(true);
    } finally {
      process.chdir(previousCwd);
    }
  });
});

describe('detectAppType', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ballast-doctor-test-'));
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  test('detects cli from .goreleaser.yaml', () => {
    fs.writeFileSync(path.join(tmpDir, '.goreleaser.yaml'), 'version: 2\n');
    expect(detectAppType(tmpDir)).toBe('cli');
  });

  test('detects cli from package.json with bin field', () => {
    fs.writeFileSync(
      path.join(tmpDir, 'package.json'),
      JSON.stringify({ name: 'mycli', bin: { mycli: './dist/index.js' } })
    );
    expect(detectAppType(tmpDir)).toBe('cli');
  });

  test('detects web from Dockerfile with next.config.js', () => {
    fs.writeFileSync(path.join(tmpDir, 'Dockerfile'), 'FROM node\n');
    fs.writeFileSync(
      path.join(tmpDir, 'next.config.js'),
      'module.exports = {};\n'
    );
    expect(detectAppType(tmpDir)).toBe('web');
  });

  test('detects api from Dockerfile with go.mod', () => {
    fs.writeFileSync(path.join(tmpDir, 'Dockerfile'), 'FROM golang\n');
    fs.writeFileSync(path.join(tmpDir, 'go.mod'), 'module example.com/myapi\n');
    expect(detectAppType(tmpDir)).toBe('api');
  });

  test('detects library from package.json without bin or Dockerfile', () => {
    fs.writeFileSync(
      path.join(tmpDir, 'package.json'),
      JSON.stringify({ name: 'mylib', version: '1.0.0' })
    );
    expect(detectAppType(tmpDir)).toBe('library');
  });

  test('detects library from go.mod without Dockerfile', () => {
    fs.writeFileSync(path.join(tmpDir, 'go.mod'), 'module example.com/mylib\n');
    expect(detectAppType(tmpDir)).toBe('library');
  });

  test('returns unknown for empty directory', () => {
    expect(detectAppType(tmpDir)).toBe('unknown');
  });

  test('prefers cli over web when both .goreleaser.yaml and Dockerfile present', () => {
    fs.writeFileSync(path.join(tmpDir, '.goreleaser.yaml'), 'version: 2\n');
    fs.writeFileSync(path.join(tmpDir, 'Dockerfile'), 'FROM golang\n');
    expect(detectAppType(tmpDir)).toBe('cli');
  });
});
