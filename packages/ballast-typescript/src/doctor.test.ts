import fs from 'fs';
import os from 'os';
import path from 'path';
import { buildDoctorReport, detectAppType, formatDoctorReport } from './doctor';

describe('doctor', () => {
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
      [{ name: 'ballast-typescript', version: '5.0.2', path: '/tmp/bt' }],
      'cli'
    );
    const output = formatDoctorReport(report);
    expect(output).toContain('Detected app type: cli');
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
      [{ name: 'ballast-typescript', version: '5.0.2', path: '/tmp/bt' }],
      'api'
    );
    const publishingRecs = report.recommendations.filter((r) =>
      r.includes('add the publishing agent')
    );
    expect(publishingRecs).toHaveLength(0);
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
