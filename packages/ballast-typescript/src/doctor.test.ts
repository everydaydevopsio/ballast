import fs from 'fs';
import os from 'os';
import path from 'path';
import { resolveHookRunner } from './config';
import {
  buildContent,
  calculateRuleChecksum,
  stripRuleMarker,
  verifyRuleChecksum
} from './build';
import { install } from './install';
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

  test('treats an omitted deploymentModel as none, matching install', () => {
    // install() resolves an omitted deploymentModel to DEFAULT_DEPLOYMENT_MODEL
    // ('none'), so it does not emit publishing-web/publishing-api. doctor must
    // use the same default or it classifies those rules as active and reports
    // the generated rule set inconsistently against a legacy config.
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });
    fs.writeFileSync(
      path.join(codexRules, 'publishing-web.md'),
      buildContent('publishing', 'codex', 'web', 'typescript'),
      'utf8'
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['publishing'],
      languages: ['typescript'],
      paths: {}
      // deploymentModel intentionally absent, as in a pre-5.19 config.
    });

    const web = statuses.find(
      (status) => path.basename(status.path) === 'publishing-web.md'
    );
    expect(web).toBeDefined();
    expect(web!.status).toBe('stale');
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

  // #376: doctor must compare against canonical content, not only against the
  // checksum in the file's own marker. A file written by an older Ballast is
  // internally consistent -- its marker matches its body exactly -- yet its
  // body is no longer what the installed version would generate.
  //
  // The existing drift case above edits the body without fixing the checksum,
  // so it only exercises the checksum branch. Deleting the canonical
  // comparison leaves that test green, which is how this regressed unnoticed.
  test('reports a self-consistent but non-canonical rule as drifted', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });

    const canonical = buildContent('linting', 'codex', undefined, 'typescript');
    const staleBody = stripRuleMarker(canonical).replace(
      '## Your Responsibilities',
      '## Your Responsibilities\n\nGuidance that an older Ballast version emitted.'
    );
    // Re-stamp the marker so the file is internally consistent at its stale
    // content, exactly as the older version would have written it.
    const selfConsistent = `<!-- ballast:rule id="typescript/linting" version="5.21.3" checksum="${calculateRuleChecksum(
      staleBody
    )}" -->\n${staleBody}`;

    const rulePath = path.join(codexRules, 'typescript-linting.md');
    fs.writeFileSync(rulePath, selfConsistent, 'utf8');

    // Guard the premise: the file really is self-consistent, so a
    // checksum-only check would call it ok.
    expect(verifyRuleChecksum(selfConsistent)).toBe(true);
    expect(stripRuleMarker(selfConsistent)).not.toBe(
      stripRuleMarker(canonical)
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['linting'],
      languages: ['typescript'],
      paths: {}
    });

    const status = statuses.find(
      (entry) => path.basename(entry.path) === 'typescript-linting.md'
    );
    expect(status?.status).toBe('drifted');
  });

  // An older version stamp on otherwise-canonical content is not a finding --
  // the ballast-audit skill says so explicitly -- so the stamp alone must not
  // drive the verdict.
  test('does not report drift for an old version stamp with canonical content', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });

    const canonical = buildContent('linting', 'codex', undefined, 'typescript');
    const body = stripRuleMarker(canonical);
    const oldStamp = `<!-- ballast:rule id="typescript/linting" version="1.0.0" checksum="${calculateRuleChecksum(
      body
    )}" -->\n${body}`;
    fs.writeFileSync(
      path.join(codexRules, 'typescript-linting.md'),
      oldStamp,
      'utf8'
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex'],
      agents: ['linting'],
      languages: ['typescript'],
      paths: {}
    });

    const status = statuses.find(
      (entry) => path.basename(entry.path) === 'typescript-linting.md'
    );
    expect(status?.status).toBe('ok');
  });

  // #356 made upgrade refuse to start on a dirty tree, on the reasoning that
  // doctor would surface any resulting skew. An interrupted upgrade can leave
  // one target current and another on older-but-self-consistent content, which
  // is exactly the state that must not read as clean.
  test('surfaces targets left on different content by an interrupted upgrade', () => {
    const codexRules = path.join(tmpDir, '.codex', 'rules');
    const claudeRules = path.join(tmpDir, '.claude', 'rules');
    fs.mkdirSync(codexRules, { recursive: true });
    fs.mkdirSync(claudeRules, { recursive: true });

    // codex was rewritten by the upgrade; claude was not.
    fs.writeFileSync(
      path.join(codexRules, 'typescript-linting.md'),
      buildContent('linting', 'codex', undefined, 'typescript'),
      'utf8'
    );

    const claudeCanonical = buildContent(
      'linting',
      'claude',
      undefined,
      'typescript'
    );
    const staleBody = stripRuleMarker(claudeCanonical).replace(
      '## Your Responsibilities',
      '## Your Responsibilities\n\nPre-upgrade guidance.'
    );
    fs.writeFileSync(
      path.join(claudeRules, 'typescript-linting.md'),
      `<!-- ballast:rule id="typescript/linting" version="5.21.3" checksum="${calculateRuleChecksum(
        staleBody
      )}" -->\n${staleBody}`,
      'utf8'
    );

    const statuses = collectRuleFileStatuses(tmpDir, {
      targets: ['codex', 'claude'],
      agents: ['linting'],
      languages: ['typescript'],
      paths: {}
    });

    const byTarget = Object.fromEntries(
      statuses.map((entry) => [entry.target, entry.status])
    );
    expect(byTarget.codex).toBe('ok');
    expect(byTarget.claude).toBe('drifted');
  });

  test('recommends trimming rules above the size threshold', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ballast-doctor-'));
    try {
      const claudeRules = path.join(tmpDir, '.claude', 'rules');
      fs.mkdirSync(claudeRules, { recursive: true });
      const canonical = buildContent('docs', 'claude');
      fs.writeFileSync(
        path.join(claudeRules, 'docs.md'),
        canonical + '\n' + 'x'.repeat(6 * 1024),
        'utf8'
      );

      const statuses = collectRuleFileStatuses(tmpDir, {
        targets: ['claude'],
        agents: ['docs'],
        languages: ['typescript'],
        paths: {}
      });
      const report = buildDoctorReport(
        'ballast-typescript',
        '5.0.2',
        path.join(tmpDir, '.rulesrc.json'),
        '5.0.2',
        ['claude'],
        ['docs'],
        [],
        ['typescript'],
        {},
        {},
        [],
        null,
        null,
        [],
        [],
        'unknown',
        statuses
      );
      expect(
        report.recommendations.some((line) =>
          line.includes('trim it or move procedural content to a skill')
        )
      ).toBe(true);
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  test('minimal ruleProfile treats only the core rule as active', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ballast-doctor-'));
    try {
      const claudeRules = path.join(tmpDir, '.claude', 'rules');
      fs.mkdirSync(claudeRules, { recursive: true });
      fs.writeFileSync(
        path.join(claudeRules, 'core.md'),
        buildContent('core', 'claude', undefined, 'typescript', {
          languages: ['typescript', 'go']
        }),
        'utf8'
      );

      const statuses = collectRuleFileStatuses(tmpDir, {
        targets: ['claude'],
        agents: ['linting', 'testing', 'docs'],
        languages: ['typescript', 'go'],
        paths: {},
        ruleProfile: 'minimal'
      });

      const core = statuses.find((s) => path.basename(s.path) === 'core.md');
      expect(core?.status).toBe('ok');
      expect(statuses.filter((s) => s.status !== 'ok')).toEqual([]);
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
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

  test('doctor fix never leaves a manifest indexing a rule it deleted', () => {
    // Regression: in a consuming repository, `doctor --fix` pruned every
    // .claude rule it judged stale but left CLAUDE.md indexing all of them.
    // Agent sessions then read a manifest promising rules that did not exist
    // and loaded none. Pruning is allowed; a manifest pointing at the pruned
    // file is not. See the castoff repository, commits d40387f and 27f3888.
    const previousCwd = process.cwd();
    process.chdir(tmpDir);
    try {
      fs.writeFileSync(path.join(tmpDir, 'package.json'), '{}\n', 'utf8');
      for (const target of ['claude', 'codex'] as const) {
        install({
          projectRoot: tmpDir,
          target,
          agents: ['linting', 'docs'],
          language: 'typescript',
          saveConfig: true
        });
      }

      runDoctor({ fix: true });

      const manifests = ['CLAUDE.md', 'AGENTS.md']
        .map((name) => path.join(tmpDir, name))
        .filter((file) => fs.existsSync(file));
      expect(manifests.length).toBeGreaterThan(0);

      const dangling: string[] = [];
      for (const manifest of manifests) {
        const body = fs.readFileSync(manifest, 'utf8');
        for (const match of body.matchAll(/`(\.[a-z]+\/rules\/[^`]+\.md)`/g)) {
          if (!fs.existsSync(path.join(tmpDir, match[1]))) {
            dangling.push(`${path.basename(manifest)} -> ${match[1]}`);
          }
        }
      }

      expect(dangling).toEqual([]);
    } finally {
      process.chdir(previousCwd);
    }
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

describe('hook runner drift', () => {
  // Install resolves the hook runner, so doctor must resolve it the same way.
  // Comparing a prek repository's rule against pre-commit guidance reports a
  // correctly generated file as drifted immediately after installation.
  test('a freshly generated prek rule is not reported as drifted', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ballast-prek-'));
    try {
      fs.writeFileSync(path.join(tmpDir, 'prek.toml'), '[hooks]\n', 'utf8');
      const codexRules = path.join(tmpDir, '.codex', 'rules');
      fs.mkdirSync(codexRules, { recursive: true });
      fs.writeFileSync(
        path.join(codexRules, 'python-git-hooks.md'),
        buildContent('git-hooks', 'codex', undefined, 'python', {
          hookRunner: 'prek'
        }),
        'utf8'
      );

      const statuses = collectRuleFileStatuses(tmpDir, {
        targets: ['codex'],
        agents: ['git-hooks'],
        languages: ['python'],
        paths: {},
        hookRunner: 'prek'
      });

      const status = statuses.find(
        (entry) => path.basename(entry.path) === 'python-git-hooks.md'
      );
      expect(status?.status).toBe('ok');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  // The converse: a pre-commit rule in a prek repository really is stale.
  test('a pre-commit rule in a prek repository is reported as drifted', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ballast-prek-'));
    try {
      const codexRules = path.join(tmpDir, '.codex', 'rules');
      fs.mkdirSync(codexRules, { recursive: true });
      fs.writeFileSync(
        path.join(codexRules, 'python-git-hooks.md'),
        buildContent('git-hooks', 'codex', undefined, 'python'),
        'utf8'
      );

      const statuses = collectRuleFileStatuses(tmpDir, {
        targets: ['codex'],
        agents: ['git-hooks'],
        languages: ['python'],
        paths: {},
        hookRunner: 'prek'
      });

      const status = statuses.find(
        (entry) => path.basename(entry.path) === 'python-git-hooks.md'
      );
      expect(status?.status).toBe('drifted');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  test('resolveHookRunner prefers an explicit choice over detection', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ballast-prek-'));
    try {
      expect(resolveHookRunner(tmpDir, null)).toBe('pre-commit');
      fs.writeFileSync(path.join(tmpDir, 'prek.toml'), '[hooks]\n', 'utf8');
      expect(resolveHookRunner(tmpDir, null)).toBe('prek');
      expect(
        resolveHookRunner(tmpDir, {
          targets: ['codex'],
          agents: ['git-hooks'],
          hookRunner: 'pre-commit'
        })
      ).toBe('pre-commit');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });
});
