import { parseArgs } from './cli';

describe('parseArgs', () => {
  test('returns help for top-level help flag', () => {
    expect(parseArgs(['node', 'ballast-typescript', '--help'])).toEqual({
      help: true
    });
  });

  test('returns version for top-level version flag', () => {
    expect(parseArgs(['node', 'ballast-typescript', '--version'])).toEqual({
      version: true
    });
  });

  test('returns doctor for doctor command', () => {
    expect(parseArgs(['node', 'ballast-typescript', 'doctor'])).toEqual({
      doctor: true
    });
  });

  test('does not treat doctor as a later install argument', () => {
    expect(
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--agent',
        'doctor',
        '--target',
        'cursor'
      ])
    ).toEqual({
      target: 'cursor',
      agents: ['doctor'],
      skills: [],
      language: 'typescript',
      all: false,
      allSkills: false,
      force: false,
      patch: false,
      yes: false
    });
  });

  test('parses skill flags', () => {
    expect(
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--skill',
        'owasp-security-scan',
        '--all-skills'
      ])
    ).toEqual({
      target: undefined,
      agents: [],
      skills: ['owasp-security-scan'],
      language: 'typescript',
      all: false,
      allSkills: true,
      force: false,
      patch: false,
      yes: false
    });
  });
});
