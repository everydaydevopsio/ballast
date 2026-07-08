import fs from 'fs';
import path from 'path';
import {
  buildClaudeSkill,
  buildContent,
  buildSkillMarkdown,
  getDestination,
  getSkillDestination,
  listRuleSuffixes
} from './build';
import {
  LANGUAGES,
  resolveAgents,
  resolveSkills,
  type Language
} from './agents';

type Candidate = {
  content: Buffer;
};

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const RULESRC_PATH = path.join(REPO_ROOT, '.rulesrc.json');
const ALLOWED_NON_GENERATED = new Set([
  '.claude/settings.json',
  '.claude/settings.local.json',
  '.claude/rules/linting.md',
  '.claude/rules/local-dev.md',
  '.claude/rules/logging.md',
  '.claude/rules/testing.md'
]);

function collectFiles(dir: string): string[] {
  if (!fs.existsSync(dir)) {
    return [];
  }

  const files: string[] = [];
  const walk = (currentDir: string): void => {
    for (const entry of fs.readdirSync(currentDir, { withFileTypes: true })) {
      const fullPath = path.join(currentDir, entry.name);
      if (entry.isDirectory()) {
        walk(fullPath);
      } else {
        files.push(path.relative(REPO_ROOT, fullPath));
      }
    }
  };

  walk(dir);
  files.sort();
  return files;
}

function addCandidate(
  candidates: Map<string, Candidate[]>,
  relPath: string,
  candidate: Candidate
): void {
  const existing = candidates.get(relPath) ?? [];
  existing.push(candidate);
  candidates.set(relPath, existing);
}

describe('repo generated artifacts', () => {
  test('tracked .codex and .claude artifacts stay in sync with source templates', () => {
    const rulesrc = JSON.parse(fs.readFileSync(RULESRC_PATH, 'utf8')) as {
      agents?: string[];
      languages?: string[];
      skills?: string[];
      taskSystem?: string;
    };

    const configuredLanguages: Language[] = Array.isArray(rulesrc.languages)
      ? LANGUAGES.filter((language) => rulesrc.languages?.includes(language))
      : ['typescript'];
    const configuredAgents = Array.isArray(rulesrc.agents)
      ? rulesrc.agents
      : [];
    const configuredSkills = Array.isArray(rulesrc.skills)
      ? rulesrc.skills
      : [];
    const taskSystem = rulesrc.taskSystem ?? 'github';

    const candidates = new Map<string, Candidate[]>();
    const ruleSubdirs: Array<string | null> = [
      null,
      'common',
      ...configuredLanguages
    ];

    const optionsFor = (language: Language) => ({
      variables: { taskSystem },
      hookMode:
        language === 'typescript' && configuredLanguages.length === 1
          ? ('monorepo' as const)
          : ('standalone' as const)
    });

    for (const target of ['codex', 'claude'] as const) {
      for (const subdir of ruleSubdirs) {
        if (subdir) {
          process.env.BALLAST_RULE_SUBDIR = subdir;
        } else {
          delete process.env.BALLAST_RULE_SUBDIR;
        }

        for (const language of configuredLanguages) {
          const resolvedAgents = resolveAgents(configuredAgents, language);
          for (const agentId of resolvedAgents) {
            for (const suffix of listRuleSuffixes(agentId, language)) {
              const destination = getDestination(
                agentId,
                target,
                REPO_ROOT,
                suffix,
                language
              );
              const relPath = path.relative(REPO_ROOT, destination.file);
              addCandidate(candidates, relPath, {
                content: Buffer.from(
                  buildContent(
                    agentId,
                    target,
                    suffix,
                    language,
                    optionsFor(language)
                  ),
                  'utf8'
                )
              });
            }
          }
        }
      }
    }

    delete process.env.BALLAST_RULE_SUBDIR;

    for (const target of ['codex', 'claude'] as const) {
      for (const skillId of resolveSkills(configuredSkills, 'typescript')) {
        const destination = getSkillDestination(skillId, target, REPO_ROOT);
        const relPath = path.relative(REPO_ROOT, destination.file);
        addCandidate(candidates, relPath, {
          content:
            target === 'claude'
              ? buildClaudeSkill(skillId)
              : Buffer.from(buildSkillMarkdown(skillId), 'utf8')
        });
      }
    }

    const actualFiles = [
      ...collectFiles(path.join(REPO_ROOT, '.codex')),
      ...collectFiles(path.join(REPO_ROOT, '.claude'))
    ].filter((relPath) => {
      if (ALLOWED_NON_GENERATED.has(relPath)) {
        return false;
      }
      return /\.(md|skill)$/.test(relPath);
    });

    const drift: string[] = [];
    const unexpected: string[] = [];

    for (const relPath of actualFiles) {
      const expected = candidates.get(relPath);
      if (!expected || expected.length === 0) {
        unexpected.push(relPath);
        continue;
      }

      const actual = fs.readFileSync(path.join(REPO_ROOT, relPath));
      const exactMatch = expected.some(
        (candidate) => Buffer.compare(actual, candidate.content) === 0
      );
      if (!exactMatch) {
        drift.push(relPath);
      }
    }

    if (process.env.BALLAST_ENFORCE_REPO_GENERATED_ARTIFACTS === '1') {
      expect({ drift, unexpected }).toEqual({ drift: [], unexpected: [] });
      return;
    }

    expect(unexpected).toEqual([]);
  });
});
