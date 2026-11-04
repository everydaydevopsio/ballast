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
      taskSystem: '',
      codeReviewer: '',
      deploymentModel: ''
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
      taskSystem: '',
      codeReviewer: '',
      deploymentModel: ''
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
      taskSystem: '',
      codeReviewer: '',
      deploymentModel: ''
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
      taskSystem: 'jira',
      codeReviewer: '',
      deploymentModel: ''
    });
  });

  test('parses --code-reviewer flag', () => {
    const parsed = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--target',
      'cursor',
      '--skill',
      'github-pr-review-cycle',
      '--code-reviewer',
      'bosun',
      '--yes'
    ]);
    expect(parsed).toMatchObject({
      skills: ['github-pr-review-cycle'],
      codeReviewer: 'bosun'
    });
  });

  test('parses the equals form of --code-reviewer', () => {
    const parsed = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--target',
      'codex',
      '--skill',
      'github-pr-review-cycle',
      '--code-reviewer=none',
      '--yes'
    ]);
    expect(parsed).toMatchObject({ codeReviewer: 'none' });
  });

  // Go's flag package and Python's argparse both accept --flag=value natively,
  // so this backend was the only one that dropped it -- silently, saving a
  // default instead of the explicit choice.
  test('parses the equals form of every value flag', () => {
    const parsed = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--target=codex',
      '--agent=docs',
      '--language=python',
      '--skill=ballast-audit',
      '--task-system=jira',
      '--deployment-model=kubernetes',
      '--code-reviewer=bosun',
      '--yes'
    ]);
    expect(parsed).toMatchObject({
      targets: ['codex'],
      agents: ['docs'],
      language: 'python',
      skills: ['ballast-audit'],
      taskSystem: 'jira',
      deploymentModel: 'kubernetes',
      codeReviewer: 'bosun'
    });
  });

  test('equals and space forms produce the same result', () => {
    const equals = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--task-system=jira',
      '--deployment-model=kubernetes',
      '--yes'
    ]);
    const spaced = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--task-system',
      'jira',
      '--deployment-model',
      'kubernetes',
      '--yes'
    ]);
    expect(equals).toEqual(spaced);
  });

  test('splits the equals form on the first = so values may contain one', () => {
    const parsed = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--code-reviewer=skill:my-reviewer',
      '--repository-facts-file=/tmp/a=b/facts.md',
      '--yes'
    ]);
    expect(parsed).toMatchObject({
      codeReviewer: 'skill:my-reviewer',
      repositoryFactsFile: '/tmp/a=b/facts.md'
    });
  });

  test('comma-separated values still work in the equals form', () => {
    const parsed = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--target=codex,claude',
      '--agent=docs,linting',
      '--yes'
    ]);
    expect(parsed).toMatchObject({
      targets: ['codex', 'claude'],
      agents: ['docs', 'linting']
    });
  });

  test('leaves a non-flag argument containing = untouched', () => {
    const parsed = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--target',
      'codex',
      '--unknown=value',
      '--yes'
    ]);
    expect(parsed).toMatchObject({ targets: ['codex'], yes: true });
  });

  test('parses a skill: reference as the code reviewer', () => {
    const parsed = parseArgs([
      'node',
      'ballast-typescript',
      'install',
      '--target',
      'cursor',
      '--code-reviewer',
      'Skill:My-Reviewer',
      '--yes'
    ]);
    expect(parsed).toMatchObject({ codeReviewer: 'skill:my-reviewer' });
  });

  test('parses --deployment-model flag', () => {
    expect(
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--target',
        'cursor',
        '--agent',
        'publishing',
        '--deployment-model',
        'kubernetes',
        '--yes'
      ])
    ).toEqual({
      targets: ['cursor'],
      agents: ['publishing'],
      skills: [],
      language: 'typescript',
      all: false,
      allSkills: false,
      force: false,
      patch: false,
      yes: true,
      taskSystem: '',
      codeReviewer: '',
      deploymentModel: 'kubernetes'
    });
  });

  test('rejects missing --task-system value', () => {
    expect(() =>
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--target',
        'cursor',
        '--agent',
        'tasks',
        '--task-system',
        '--yes'
      ])
    ).toThrow('Missing value for --task-system');
  });

  test('rejects missing --deployment-model value', () => {
    expect(() =>
      parseArgs([
        'node',
        'ballast-typescript',
        'install',
        '--target',
        'cursor',
        '--agent',
        'publishing',
        '--deployment-model',
        '--yes'
      ])
    ).toThrow('Missing value for --deployment-model');
  });
});
