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

  test('returns list for list command', () => {
    expect(parseArgs(['node', 'ballast-typescript', 'list'])).toEqual({
      list: true
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
      targets: ['cursor'],
      agents: ['doctor'],
      skills: [],
      language: 'typescript',
      all: false,
      allSkills: false,
      force: false,
      patch: false,
      yes: false,
      taskSystem: ''
    });
  });

  test('parses skill flags', () => {
    expect(
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--skill',
        'owasp-security-scan,aws-health-review',
        '--all-skills'
      ])
    ).toEqual({
      targets: [],
      agents: [],
      skills: ['owasp-security-scan', 'aws-health-review'],
      language: 'typescript',
      all: false,
      allSkills: true,
      force: false,
      patch: false,
      yes: false,
      taskSystem: ''
    });
  });

  test('parses comma-separated and repeated target flags', () => {
    expect(
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--target',
        'cursor,claude',
        '--target',
        'codex',
        '--agent',
        'linting'
      ])
    ).toEqual({
      targets: ['cursor', 'claude', 'codex'],
      agents: ['linting'],
      skills: [],
      language: 'typescript',
      all: false,
      allSkills: false,
      force: false,
      patch: false,
      yes: false,
      taskSystem: ''
    });
  });

  test('parses --task-system flag', () => {
    expect(
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--target',
        'cursor',
        '--agent',
        'tasks',
        '--task-system',
        'jira',
        '--yes'
      ])
    ).toEqual({
      targets: ['cursor'],
      agents: ['tasks'],
      skills: [],
      language: 'typescript',
      all: false,
      allSkills: false,
      force: false,
      patch: false,
      yes: true,
      taskSystem: 'jira'
    });
  });
});
