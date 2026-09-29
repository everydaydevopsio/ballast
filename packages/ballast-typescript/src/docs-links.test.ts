import fs from 'fs';
import path from 'path';

const REPO_ROOT = path.resolve(__dirname, '../../..');

/**
 * The hand-written documentation surface. Generated output under `.claude/` and
 * `.codex/` is excluded: it is rebuilt from `agents/` and already covered by
 * repo-generated-artifacts.test.ts, and the packaged copies under `packages/`
 * are synced rather than authored.
 */
const DOC_ROOTS = ['docs', 'adr', 'plans', 'tasks'];
const DOC_FILES = ['README.md', 'AGENTS.md', 'CLAUDE.md'];

function collectMarkdown(): string[] {
  const found: string[] = [];

  for (const file of DOC_FILES) {
    if (fs.existsSync(path.join(REPO_ROOT, file))) {
      found.push(file);
    }
  }

  const walk = (relativeDir: string): void => {
    const absolute = path.join(REPO_ROOT, relativeDir);
    if (!fs.existsSync(absolute)) {
      return;
    }
    for (const entry of fs.readdirSync(absolute, { withFileTypes: true })) {
      const relative = path.join(relativeDir, entry.name);
      if (entry.isDirectory()) {
        walk(relative);
      } else if (entry.name.endsWith('.md')) {
        found.push(relative);
      }
    }
  };

  DOC_ROOTS.forEach(walk);
  return found.sort();
}

/** A path containing placeholder syntax is an example, not a reference. */
function isTemplate(target: string): boolean {
  return /[<>*{}]|\.\.\./.test(target);
}

describe('documentation links', () => {
  const files = collectMarkdown();

  test('finds the documentation surface it is meant to guard', () => {
    expect(files).toContain('AGENTS.md');
    expect(files).toContain('docs/README.md');
    expect(files.length).toBeGreaterThan(10);
  });

  test('every relative markdown link resolves', () => {
    const broken: string[] = [];

    for (const file of files) {
      const body = fs.readFileSync(path.join(REPO_ROOT, file), 'utf8');
      for (const match of body.matchAll(/\[[^\]]*\]\(([^)]+)\)/g)) {
        const target = match[1].split('#')[0].trim();
        if (
          target === '' ||
          /^[a-z][a-z0-9+.-]*:/i.test(target) ||
          target.startsWith('/') ||
          isTemplate(target)
        ) {
          continue;
        }
        const resolved = path.resolve(
          path.dirname(path.join(REPO_ROOT, file)),
          target
        );
        if (!fs.existsSync(resolved)) {
          broken.push(`${file} -> ${target}`);
        }
      }
    }

    expect(broken).toEqual([]);
  });

  test('every docs/ path named in backticks exists', () => {
    // #340: AGENTS.md said "Follow `docs/code_review.md` for code reviews" for a
    // file that did not exist. A markdown-link check would not have caught it,
    // because the reference was inline code. Scoped to docs/*.md so that
    // placeholder paths elsewhere are not mistaken for broken references.
    //
    // plans/ and tasks/ are excluded on purpose: they describe intent, and a
    // plan or todo naming a file it intends to create is correct, not drift.
    // Everything else here asserts what the repository has now, and must be
    // true -- README.md, AGENTS.md, CLAUDE.md, docs/ and adr/.
    const broken: string[] = [];
    const assertsCurrentState = (file: string): boolean =>
      !file.startsWith('plans/') && !file.startsWith('tasks/');

    for (const file of files.filter(assertsCurrentState)) {
      const body = fs.readFileSync(path.join(REPO_ROOT, file), 'utf8');
      for (const match of body.matchAll(/`(docs\/[A-Za-z0-9_./-]+\.md)`/g)) {
        const target = match[1];
        if (isTemplate(target)) {
          continue;
        }
        if (!fs.existsSync(path.join(REPO_ROOT, target))) {
          broken.push(`${file} -> ${target}`);
        }
      }
    }

    expect(broken).toEqual([]);
  });
});
