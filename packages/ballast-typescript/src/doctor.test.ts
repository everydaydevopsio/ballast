import { buildDoctorReport, formatDoctorReport } from './doctor';

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
});
