import fs from 'fs';
import path from 'path';
import os from 'os';
import readline from 'readline';
import zlib from 'zlib';
import { install, resolveTargetAndAgents, runInstall } from './install';
import {
  buildClaudeSkill,
  buildCodexAgentsMd,
  getClaudeMdPath,
  getDestination,
  getGeminiMdPath
} from './build';
import { findProjectRoot, saveConfig, loadConfig } from './config';
import { BALLAST_VERSION } from './version';

describe('install', () => {
  let tmpDir: string;
  const origEnv = process.env;

  beforeEach(() => {
    tmpDir = path.join(os.tmpdir(), `ballast-install-${Date.now()}`);
    fs.mkdirSync(tmpDir, { recursive: true });
    process.env = { ...origEnv };
    delete process.env.CI;
    delete process.env.GITHUB_ACTIONS;
  });

  afterEach(() => {
    if (tmpDir && fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true });
    }
    jest.restoreAllMocks();
    process.env = origEnv;
  });

  function buildClaudeSkillWithDataDescriptor(skillMd: string): Buffer {
    const fileName = Buffer.from('SKILL.md', 'utf8');
    const content = Buffer.from(skillMd, 'utf8');
    const compressed = zlib.deflateRawSync(content);

    const localHeader = Buffer.alloc(30);
    localHeader.writeUInt32LE(0x04034b50, 0);
    localHeader.writeUInt16LE(20, 4);
    localHeader.writeUInt16LE(0x0008, 6);
    localHeader.writeUInt16LE(8, 8);
    localHeader.writeUInt16LE(fileName.length, 26);

    const dataDescriptor = Buffer.alloc(16);
    dataDescriptor.writeUInt32LE(0x08074b50, 0);
    dataDescriptor.writeUInt32LE(compressed.length, 8);
    dataDescriptor.writeUInt32LE(content.length, 12);

    const centralDirectory = Buffer.alloc(46);
    centralDirectory.writeUInt32LE(0x02014b50, 0);
    centralDirectory.writeUInt16LE(20, 4);
    centralDirectory.writeUInt16LE(20, 6);
    centralDirectory.writeUInt16LE(0x0008, 8);
    centralDirectory.writeUInt16LE(8, 10);
    centralDirectory.writeUInt32LE(compressed.length, 20);
    centralDirectory.writeUInt32LE(content.length, 24);
    centralDirectory.writeUInt16LE(fileName.length, 28);

    const centralDirectoryOffset =
      localHeader.length +
      fileName.length +
      compressed.length +
      dataDescriptor.length;
    const eocd = Buffer.alloc(22);
    eocd.writeUInt32LE(0x06054b50, 0);
    eocd.writeUInt16LE(1, 8);
    eocd.writeUInt16LE(1, 10);
    eocd.writeUInt32LE(centralDirectory.length + fileName.length, 12);
    eocd.writeUInt32LE(centralDirectoryOffset, 16);

    return Buffer.concat([
      localHeader,
      fileName,
      compressed,
      dataDescriptor,
      centralDirectory,
      fileName,
      eocd
    ]);
  }

  function buildClaudeSkillWithDataDescriptorComment(
    skillMd: string,
    comment: Buffer
  ): Buffer {
    const archive = buildClaudeSkillWithDataDescriptor(skillMd);
    const eocdOffset = archive.length - 22;
    const eocd = Buffer.from(archive.subarray(eocdOffset));
    eocd.writeUInt16LE(comment.length, 20);
    return Buffer.concat([archive.subarray(0, eocdOffset), eocd, comment]);
  }

  function readSkillMdFromArchive(archive: Buffer): string | undefined {
    const eocdSignature = 0x06054b50;
    const centralDirectorySignature = 0x02014b50;
    const localFileHeaderSignature = 0x04034b50;
    const minEocdSize = 22;
    const maxCommentLength = 0xffff;
    const searchStart = Math.max(
      0,
      archive.length - minEocdSize - maxCommentLength
    );

    eocdLoop: for (
      let eocdOffset = archive.length - minEocdSize;
      eocdOffset >= searchStart;
      eocdOffset--
    ) {
      if (archive.readUInt32LE(eocdOffset) !== eocdSignature) {
        continue;
      }

      const centralDirectorySize = archive.readUInt32LE(eocdOffset + 12);
      const centralDirectoryOffset = archive.readUInt32LE(eocdOffset + 16);
      const centralDirectoryEnd = centralDirectoryOffset + centralDirectorySize;
      if (
        centralDirectoryOffset < 0 ||
        centralDirectoryEnd > archive.length ||
        centralDirectoryOffset > centralDirectoryEnd
      ) {
        continue;
      }

      let entryOffset = centralDirectoryOffset;
      while (entryOffset + 46 <= centralDirectoryEnd) {
        if (archive.readUInt32LE(entryOffset) !== centralDirectorySignature) {
          break;
        }

        const compressionMethod = archive.readUInt16LE(entryOffset + 10);
        const compressedSize = archive.readUInt32LE(entryOffset + 20);
        const fileNameLength = archive.readUInt16LE(entryOffset + 28);
        const extraLength = archive.readUInt16LE(entryOffset + 30);
        const commentLength = archive.readUInt16LE(entryOffset + 32);
        const localHeaderOffset = archive.readUInt32LE(entryOffset + 42);
        const fileNameStart = entryOffset + 46;
        const fileNameEnd = fileNameStart + fileNameLength;
        if (fileNameEnd > centralDirectoryEnd) {
          continue eocdLoop;
        }
        const fileName = archive.toString('utf8', fileNameStart, fileNameEnd);
        if (fileName === 'SKILL.md') {
          if (localHeaderOffset + 30 > archive.length) {
            continue eocdLoop;
          }
          if (
            archive.readUInt32LE(localHeaderOffset) !== localFileHeaderSignature
          ) {
            continue eocdLoop;
          }
          const localFileNameLength = archive.readUInt16LE(
            localHeaderOffset + 26
          );
          const localExtraLength = archive.readUInt16LE(localHeaderOffset + 28);
          const dataStart =
            localHeaderOffset + 30 + localFileNameLength + localExtraLength;
          const dataEnd = dataStart + compressedSize;
          if (dataEnd > archive.length) {
            continue eocdLoop;
          }
          const data = archive.subarray(dataStart, dataEnd);
          if (compressionMethod === 0) {
            return data.toString('utf8');
          }
          if (compressionMethod === 8) {
            return zlib.inflateRawSync(data).toString('utf8');
          }
          return undefined;
        }

        entryOffset = fileNameEnd + extraLength + commentLength;
      }
    }

    return undefined;
  }

  describe('resolveTargetAndAgents', () => {
    test('with target and agents from options returns them', async () => {
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['cursor'],
        agents: ['linting']
      });
      expect(result).toEqual({
        targets: ['cursor'],
        agents: ['linting', 'git-hooks'],
        skills: []
      });
    });

    test('with --all expands to all agents', async () => {
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['claude'],
        agents: 'all'
      });
      expect(result?.targets).toEqual(['claude']);
      expect(result?.agents).toContain('linting');
      expect(result?.agents).toContain('local-dev');
      expect(result?.agents).toContain('docs');
      expect(result?.agents).toContain('cicd');
      expect(result?.agents).toContain('observability');
      expect(result?.agents).toContain('publishing');
      expect(result?.agents).toContain('git-hooks');
      expect(result?.agents).toContain('logging');
      expect(result?.agents).toContain('testing');
      expect(result?.skills).toEqual([]);
    });

    test('with saved config returns config when no flags', async () => {
      saveConfig(
        {
          targets: ['opencode'],
          agents: ['linting', 'cicd'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir
      });
      expect(result).toEqual({
        targets: ['opencode'],
        agents: ['linting', 'cicd', 'git-hooks'],
        skills: ['owasp-security-scan']
      });
    });

    test('in CI with no config and no args returns null', async () => {
      process.env.CI = 'true';
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        yes: true
      });
      expect(result).toBeNull();
    });

    test('in CI with target and agents returns them', async () => {
      process.env.CI = 'true';
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        yes: true,
        targets: ['cursor'],
        agents: ['linting']
      });
      expect(result).toEqual({
        targets: ['cursor'],
        agents: ['linting', 'git-hooks'],
        skills: []
      });
    });

    test('supports multi-target flags and saved config', async () => {
      saveConfig(
        {
          targets: ['cursor', 'claude'],
          agents: ['linting'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir
      });
      expect(result).toEqual({
        targets: ['cursor', 'claude'],
        agents: ['linting', 'git-hooks'],
        skills: ['owasp-security-scan']
      });
    });

    test('interactive selection normalizes implicit git-hooks before returning', async () => {
      const answers = ['cursor', 'linting', ''];
      const createInterfaceSpy = jest
        .spyOn(readline, 'createInterface')
        .mockImplementation(
          () =>
            ({
              question: (_prompt: string, cb: (answer: string) => void) =>
                cb(answers.shift() ?? ''),
              close: () => {}
            }) as unknown as readline.Interface
        );
      try {
        const result = await resolveTargetAndAgents({
          projectRoot: tmpDir
        });

        expect(result).toEqual({
          targets: ['cursor'],
          agents: ['linting', 'git-hooks'],
          skills: []
        });
      } finally {
        createInterfaceSpy.mockRestore();
      }
    });

    test('rejects invalid target flags instead of ignoring them', async () => {
      await expect(
        resolveTargetAndAgents({
          projectRoot: tmpDir,
          targets: ['cursor', 'bogus'],
          agents: ['linting']
        })
      ).rejects.toThrow(
        'Invalid --target. Use: cursor, claude, opencode, codex, gemini'
      );
    });

    test('rejects invalid agent flags instead of silently ignoring them', async () => {
      await expect(
        resolveTargetAndAgents({
          projectRoot: tmpDir,
          targets: ['cursor'],
          agents: ['bogus-agent']
        })
      ).rejects.toThrow('Invalid --agent');
    });

    test('rejects invalid skill flags instead of silently ignoring them', async () => {
      await expect(
        resolveTargetAndAgents({
          projectRoot: tmpDir,
          targets: ['cursor'],
          agents: ['linting'],
          skills: ['bogus-skill']
        })
      ).rejects.toThrow('Invalid --skill');
    });

    test('merges new agent with existing config agents', async () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['cursor'],
        agents: ['testing']
      });
      expect(result?.agents).toContain('linting');
      expect(result?.agents).toContain('testing');
      expect(result?.agents).toContain('git-hooks');
      expect(result?.skills).toEqual(['owasp-security-scan']);
    });

    test('merges new skill with existing config skills', async () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['cursor'],
        agents: ['linting'],
        skills: ['aws-health-review']
      });
      expect(result?.agents).toContain('linting');
      expect(result?.skills).toContain('owasp-security-scan');
      expect(result?.skills).toContain('aws-health-review');
    });

    test('applies withImplicitAgents to the full merged agent set', async () => {
      // config has linting but is missing git-hooks (e.g. older or hand-edited config)
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          skills: []
        },
        tmpDir
      );
      // force-write config without git-hooks to simulate older/hand-edited state
      const fs = require('fs');
      const path = require('path');
      const rulesrc = path.join(tmpDir, '.rulesrc.json');
      const raw = JSON.parse(fs.readFileSync(rulesrc, 'utf8'));
      raw.agents = ['linting']; // strip git-hooks
      fs.writeFileSync(rulesrc, JSON.stringify(raw, null, 2));

      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['cursor'],
        agents: ['testing']
      });
      // git-hooks must appear even though it was missing from the saved config
      expect(result?.agents).toContain('linting');
      expect(result?.agents).toContain('testing');
      expect(result?.agents).toContain('git-hooks');
    });

    test('reinstalling a subset of agents retains the rest', async () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting', 'git-hooks', 'testing', 'logging'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['cursor'],
        agents: ['linting']
      });
      expect(result?.agents).toContain('linting');
      expect(result?.agents).toContain('testing');
      expect(result?.agents).toContain('logging');
      expect(result?.agents).toContain('git-hooks');
      expect(result?.skills).toEqual(['owasp-security-scan']);
    });

    test('reinstalling a subset of skills retains the rest', async () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          skills: ['owasp-security-scan', 'aws-health-review']
        },
        tmpDir
      );
      const result = await resolveTargetAndAgents({
        projectRoot: tmpDir,
        targets: ['cursor'],
        agents: ['linting'],
        skills: ['owasp-security-scan']
      });
      expect(result?.skills).toContain('owasp-security-scan');
      expect(result?.skills).toContain('aws-health-review');
    });
  });

  describe('install', () => {
    test('writes files and returns installed list', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        skills: ['owasp-security-scan'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(result.installedSkills).toContain('owasp-security-scan');
      expect(result.errors).toHaveLength(0);
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      expect(fs.readFileSync(cursorFile, 'utf8')).toContain(
        'TypeScript linting specialist'
      );
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'owasp-security-scan.mdc')
        )
      ).toBe(true);
      expect(
        fs.readFileSync(
          path.join(tmpDir, '.cursor', 'rules', 'owasp-security-scan.mdc'),
          'utf8'
        )
      ).toContain('Created by [Ballast]');
    });

    test('skips an existing skill file when force and patch are false', () => {
      const skillFile = path.join(
        tmpDir,
        '.opencode',
        'skills',
        'owasp-security-scan.md'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(skillFile, 'stale skill content', 'utf8');

      const result = install({
        projectRoot: tmpDir,
        target: 'opencode',
        agents: [],
        skills: ['owasp-security-scan'],
        force: false,
        saveConfig: false
      });

      expect(result.installedSkills).toEqual([]);
      expect(result.skipped).toEqual([]);
      expect(fs.readFileSync(skillFile, 'utf8')).toBe('stale skill content');
    });

    test('creates a missing skill file when patch is true', () => {
      const skillFile = path.join(
        tmpDir,
        '.opencode',
        'skills',
        'owasp-security-scan.md'
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'opencode',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.installedSkills).toContain('owasp-security-scan');
      expect(fs.readFileSync(skillFile, 'utf8')).toContain(
        '## Scan Architecture'
      );
      expect(fs.readFileSync(skillFile, 'utf8')).toContain(
        'Created by [Ballast]'
      );
    });

    test('patches an existing skill file when patch is true', () => {
      const skillFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'owasp-security-scan.mdc'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(
        skillFile,
        `---
description: Team customized skill
alwaysApply: true
---

Team intro.

## Usage

Keep team-specific usage notes.
`,
        'utf8'
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.installedSkills).toContain('owasp-security-scan');
      const content = fs.readFileSync(skillFile, 'utf8');
      expect(content).toContain('description: Team customized skill');
      expect(content).toContain('alwaysApply: true');
      expect(content).toContain('Keep team-specific usage notes.');
      expect(content).toContain('## Scan Architecture');
    });

    test('overwrites an existing skill file when force is true', () => {
      const skillFile = path.join(
        tmpDir,
        '.opencode',
        'skills',
        'owasp-security-scan.md'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(skillFile, 'stale skill content', 'utf8');

      const result = install({
        projectRoot: tmpDir,
        target: 'opencode',
        agents: [],
        skills: ['owasp-security-scan'],
        force: true,
        saveConfig: false
      });

      expect(result.installedSkills).toContain('owasp-security-scan');
      expect(fs.readFileSync(skillFile, 'utf8')).toContain(
        '## Scan Architecture'
      );
      expect(fs.readFileSync(skillFile, 'utf8')).not.toBe(
        'stale skill content'
      );
    });

    test('refresh mode overwrites an existing skill file without force or patch', () => {
      process.env.BALLAST_REFRESH_SKILLS = '1';
      const skillFile = path.join(
        tmpDir,
        '.opencode',
        'skills',
        'owasp-security-scan.md'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(skillFile, 'stale skill content', 'utf8');

      const result = install({
        projectRoot: tmpDir,
        target: 'opencode',
        agents: [],
        skills: ['owasp-security-scan'],
        force: false,
        saveConfig: false
      });

      expect(result.installedSkills).toContain('owasp-security-scan');
      expect(fs.readFileSync(skillFile, 'utf8')).toContain(
        '## Scan Architecture'
      );
    });

    test('changing taskSystem rewrites existing task rules without force or patch', () => {
      const taskRule = path.join(
        tmpDir,
        '.codex',
        'rules',
        'tasks-task-system.md'
      );
      fs.mkdirSync(path.dirname(taskRule), { recursive: true });
      fs.writeFileSync(
        taskRule,
        'Use jira as the system of record for work items.\n',
        'utf8'
      );
      saveConfig(
        {
          targets: ['codex'],
          agents: ['tasks'],
          skills: [],
          taskSystem: 'jira'
        },
        tmpDir
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'codex',
        agents: ['tasks'],
        skills: [],
        taskSystem: 'linear',
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('tasks');
      const content = fs.readFileSync(taskRule, 'utf8');
      expect(content).toContain('linear');
      expect(content).not.toContain('Use jira as the system of record');
    });

    test('patches SKILL.md inside an existing claude .skill archive when patch is true', () => {
      const skillFile = path.join(
        tmpDir,
        '.claude',
        'skills',
        'owasp-security-scan.skill'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      const existingSkillContent = `# owasp-security-scan

Team intro preserved by patch.

## Team Custom Section

Keep this team-specific section.
`;
      fs.writeFileSync(
        skillFile,
        buildClaudeSkill('owasp-security-scan', existingSkillContent)
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.installedSkills).toContain('owasp-security-scan');

      const archive = fs.readFileSync(skillFile);
      const skillMd = readSkillMdFromArchive(archive);
      expect(skillMd).toContain('Team intro preserved by patch.');
      expect(skillMd).toContain('Team Custom Section');
      expect(skillMd).toContain('## Scan Architecture');
    });

    test('patch falls back to overwrite when an existing claude .skill archive is unreadable', () => {
      const skillFile = path.join(
        tmpDir,
        '.claude',
        'skills',
        'owasp-security-scan.skill'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(skillFile, Buffer.from('not-a-zip-archive', 'utf8'));

      const result = install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.errors).toEqual([]);
      expect(result.installedSkills).toContain('owasp-security-scan');

      const archive = fs.readFileSync(skillFile);
      const skillMd = readSkillMdFromArchive(archive);
      expect(skillMd).toContain('## Scan Architecture');
      expect(skillMd).not.toContain('not-a-zip-archive');
    });

    test('patch falls back to overwrite when reading an existing claude archive throws', () => {
      const skillFile = path.join(
        tmpDir,
        '.claude',
        'skills',
        'owasp-security-scan.skill'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(
        skillFile,
        buildClaudeSkill('owasp-security-scan', '# existing skill\n')
      );

      const originalReadFileSync = fs.readFileSync;
      let injectedFailure = false;
      jest.spyOn(fs, 'readFileSync').mockImplementation(((
        file: fs.PathOrFileDescriptor,
        options?: unknown
      ) => {
        if (file === skillFile && options === undefined && !injectedFailure) {
          injectedFailure = true;
          throw new Error('permission denied');
        }
        return originalReadFileSync(file as never, options as never);
      }) as typeof fs.readFileSync);

      const result = install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.errors).toEqual([]);
      expect(result.installedSkills).toContain('owasp-security-scan');

      const archive = fs.readFileSync(skillFile);
      const skillMd = readSkillMdFromArchive(archive);
      expect(skillMd).toContain('## Scan Architecture');
    });

    test('patch preserves team content from valid claude archives that use data descriptors', () => {
      const skillFile = path.join(
        tmpDir,
        '.claude',
        'skills',
        'owasp-security-scan.skill'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(
        skillFile,
        buildClaudeSkillWithDataDescriptor(`# owasp-security-scan

Team intro preserved by patch.

## Team Custom Section

Keep this team-specific section.
`)
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.errors).toEqual([]);
      expect(result.installedSkills).toContain('owasp-security-scan');

      const archive = fs.readFileSync(skillFile);
      const skillMd = readSkillMdFromArchive(archive);
      expect(skillMd).toContain('Team intro preserved by patch.');
      expect(skillMd).toContain('Team Custom Section');
      expect(skillMd).toContain('## Scan Architecture');
    });

    test('patch preserves team content when the zip comment contains an EOCD signature', () => {
      const skillFile = path.join(
        tmpDir,
        '.claude',
        'skills',
        'owasp-security-scan.skill'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      fs.writeFileSync(
        skillFile,
        buildClaudeSkillWithDataDescriptorComment(
          `# owasp-security-scan

Team intro preserved by patch.

## Team Custom Section

Keep this team-specific section.
`,
          Buffer.from('comment-\x50\x4b\x05\x06-tail', 'binary')
        )
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.errors).toEqual([]);
      expect(result.installedSkills).toContain('owasp-security-scan');
      const skillMd = readSkillMdFromArchive(fs.readFileSync(skillFile));
      expect(skillMd).toContain('Team intro preserved by patch.');
      expect(skillMd).toContain('Team Custom Section');
      expect(skillMd).toContain('## Scan Architecture');
    });

    test('force-overwrites an existing claude .skill archive without patching', () => {
      const skillFile = path.join(
        tmpDir,
        '.claude',
        'skills',
        'owasp-security-scan.skill'
      );
      fs.mkdirSync(path.dirname(skillFile), { recursive: true });
      const existingSkillContent = `# owasp-security-scan

Team intro that should be discarded on force.

## Team Custom Section

This section should be gone after force.
`;
      fs.writeFileSync(
        skillFile,
        buildClaudeSkill('owasp-security-scan', existingSkillContent)
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: [],
        skills: ['owasp-security-scan'],
        patch: false,
        force: true,
        saveConfig: false
      });

      expect(result.installedSkills).toContain('owasp-security-scan');

      const skillMd = readSkillMdFromArchive(fs.readFileSync(skillFile));
      expect(skillMd).not.toContain(
        'Team intro that should be discarded on force.'
      );
      expect(skillMd).not.toContain('Team Custom Section');
      expect(skillMd).toContain('## Scan Architecture');
    });

    test('writes ansible language rules when requested', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        language: 'ansible',
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('linting');
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'ansible-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      expect(fs.readFileSync(cursorFile, 'utf8')).toContain(
        'Ansible linting specialist'
      );
    });

    test('writes terraform language rules when requested', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        language: 'terraform',
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('linting');
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'terraform-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).toContain('Terraform linting specialist');
      expect(content).toContain('.terraform-version');
      expect(content).toContain('tfenv install');
      expect(content).toContain('terraform fmt -check -recursive');
      expect(content).toContain('trivy config');
      expect(content).toContain('tfsec');
      expect(content).toContain('plugin blocks');
      expect(content).toContain('tflint --init');
      expect(content).toContain('OpenTofu');
      expect(content).toContain('tofu fmt');
    });

    test('writes terraform testing rules with native tests and CI guidance', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'codex',
        agents: ['testing'],
        language: 'terraform',
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('testing');
      const ruleFile = path.join(
        tmpDir,
        '.codex',
        'rules',
        'terraform-testing.md'
      );
      expect(fs.existsSync(ruleFile)).toBe(true);
      const content = fs.readFileSync(ruleFile, 'utf8');
      expect(content).toContain('terraform test');
      expect(content).toContain('Terraform 1.6');
      expect(content).toContain('Terratest');
      expect(content).toContain('concurrency:');
      expect(content).toContain('PR validation');
      expect(content).toContain('apply');
      expect(content).toContain('OpenTofu');
      expect(content).toContain('tofu test');
    });

    test('uses husky guidance for typescript-only installs', () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          languages: ['typescript']
        },
        tmpDir
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });

      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).not.toContain('.pre-commit-config.yaml');
      expect(content).not.toContain('pre-commit install');
      expect(content).not.toContain('pre-commit install --hook-type pre-push');
      expect(content).not.toContain(
        'Use Husky for TypeScript-only repositories.'
      );

      const gitHooksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'git-hooks.mdc'
      );
      expect(fs.existsSync(gitHooksFile)).toBe(true);
      const gitHooksContent = fs.readFileSync(gitHooksFile, 'utf8');
      expect(gitHooksContent).toContain(
        'Use Husky for TypeScript-only repositories.'
      );
      expect(gitHooksContent).toContain('lint-staged');
      expect(gitHooksContent).toContain('.husky/pre-push');
      expect(gitHooksContent).not.toContain('pre-commit install');
      expect(gitHooksContent).not.toContain('.pre-commit-config.yaml');
    });

    test('uses pre-commit guidance for multi-language installs', () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          languages: ['typescript', 'python', 'go', 'ansible', 'terraform']
        },
        tmpDir
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });

      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).not.toContain(
        'Use Husky for TypeScript-only repositories.'
      );
      expect(content).not.toContain('lint-staged');
      expect(content).not.toContain('.husky/pre-push');
      expect(content).not.toContain('pre-commit install');

      const gitHooksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'git-hooks.mdc'
      );
      expect(fs.existsSync(gitHooksFile)).toBe(true);
      const gitHooksContent = fs.readFileSync(gitHooksFile, 'utf8');
      expect(gitHooksContent).toContain('.pre-commit-config.yaml');
      expect(gitHooksContent).toContain(
        'pre-commit install --hook-type pre-push'
      );
      expect(gitHooksContent).not.toContain(
        'Use Husky for TypeScript-only repositories.'
      );
      expect(gitHooksContent).not.toContain('lint-staged');
      expect(gitHooksContent).not.toContain('.husky/pre-push');
    });

    test('uses husky guidance for typescript workspace monorepos even with one language', () => {
      fs.writeFileSync(
        path.join(tmpDir, 'package.json'),
        JSON.stringify({ name: 'workspace-root', private: true }, null, 2)
      );
      fs.writeFileSync(
        path.join(tmpDir, 'pnpm-workspace.yaml'),
        'packages:\n  - apps/*\n  - packages/*\n',
        'utf8'
      );
      fs.mkdirSync(path.join(tmpDir, 'apps', 'web'), { recursive: true });
      fs.writeFileSync(
        path.join(tmpDir, 'apps', 'web', 'package.json'),
        JSON.stringify({ name: '@app/web', private: true }, null, 2)
      );
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          languages: ['typescript']
        },
        tmpDir
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });

      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'typescript-linting.mdc'
      );
      const content = fs.readFileSync(cursorFile, 'utf8');
      expect(content).not.toContain('Set Up Git Hooks with Husky');
      expect(content).not.toContain('pre-commit install');

      const gitHooksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'git-hooks.mdc'
      );
      expect(fs.readFileSync(gitHooksFile, 'utf8')).toContain(
        'Use Husky for TypeScript-only repositories.'
      );
    });

    test('uses pre-commit for multi-language installs even with a typescript workspace manifest', () => {
      fs.writeFileSync(
        path.join(tmpDir, 'package.json'),
        JSON.stringify(
          { name: 'single-root', private: true, workspaces: ['apps/*'] },
          null,
          2
        )
      );
      fs.mkdirSync(path.join(tmpDir, '.codex', 'shadow'), { recursive: true });
      fs.writeFileSync(
        path.join(tmpDir, '.codex', 'shadow', 'package.json'),
        JSON.stringify({ name: 'hidden-package', private: true }, null, 2)
      );
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          languages: ['typescript', 'python']
        },
        tmpDir
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });

      const gitHooksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'git-hooks.mdc'
      );
      const gitHooksContent = fs.readFileSync(gitHooksFile, 'utf8');
      expect(gitHooksContent).not.toContain(
        'Use Husky for TypeScript-only repositories.'
      );
      expect(gitHooksContent).not.toContain('lint-staged');
      expect(gitHooksContent).toContain(
        'pre-commit install --hook-type pre-push'
      );
    });

    test('writes python language files and uses python-specific content', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        language: 'python',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      const cursorFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'python-linting.mdc'
      );
      expect(fs.existsSync(cursorFile)).toBe(true);
      expect(fs.readFileSync(cursorFile, 'utf8')).toContain(
        'Python linting specialist'
      );
    });

    test('skips when file exists and force is false', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'existing content'
      );
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['git-hooks']);
      expect(result.skipped).toContain('linting');
      expect(
        fs.readFileSync(path.join(cursorDir, 'typescript-linting.mdc'), 'utf8')
      ).toBe('existing content');
    });

    test('overwrites when force is true', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'existing content'
      );
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: true,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(
        fs.readFileSync(path.join(cursorDir, 'typescript-linting.mdc'), 'utf8')
      ).toContain('TypeScript linting specialist');
    });

    test('patches an existing rule file when patch is true', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        `---
description: Team customized linting rules
alwaysApply: true
---

User intro.

## Your Responsibilities

Keep my custom responsibilities.

## Team Overrides

Do not remove this note.
`
      );

      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        patch: true,
        force: false,
        saveConfig: false
      });

      expect(result.installed).toContain('linting');
      const content = fs.readFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'utf8'
      );
      expect(content).toContain('description: Team customized linting rules');
      expect(content).toContain('alwaysApply: true');
      expect(content).toContain('Keep my custom responsibilities.');
      expect(content).toContain('## Team Overrides');
      expect(content).toContain('## When Completed');
    });

    test('force wins over patch when both flags are true', () => {
      const cursorDir = path.join(tmpDir, '.cursor', 'rules');
      fs.mkdirSync(cursorDir, { recursive: true });
      fs.writeFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        `---
description: Team customized linting rules
alwaysApply: true
---

User intro.

## Your Responsibilities

Keep my custom responsibilities.
`
      );

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        patch: true,
        force: true,
        saveConfig: false
      });

      const content = fs.readFileSync(
        path.join(cursorDir, 'typescript-linting.mdc'),
        'utf8'
      );
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('alwaysApply: false');
      expect(content).not.toContain('Keep my custom responsibilities.');
    });

    test('saves config when saveConfig is true', () => {
      install({
        projectRoot: tmpDir,
        target: 'claude',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });
      const config = loadConfig(tmpDir);
      expect(config).toEqual({
        targets: ['claude'],
        agents: ['linting', 'local-dev', 'git-hooks'],
        ballastVersion: BALLAST_VERSION,
        languages: ['typescript'],
        paths: { typescript: ['.'] }
      });
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.languages).toEqual(['typescript']);
      expect(raw.ballastVersion).toBe(BALLAST_VERSION);
      expect(raw.paths).toEqual({ typescript: ['.'] });
    });

    test('saves shared .rulesrc.json for go installs', () => {
      install({
        projectRoot: tmpDir,
        target: 'claude',
        language: 'go',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });
      const config = loadConfig(tmpDir, 'go');
      expect(config).toEqual({
        targets: ['claude'],
        agents: ['linting', 'local-dev', 'git-hooks'],
        ballastVersion: BALLAST_VERSION,
        languages: ['go'],
        paths: { go: ['.'] }
      });
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.languages).toEqual(['go']);
      expect(raw.ballastVersion).toBe(BALLAST_VERSION);
      expect(raw.paths).toEqual({ go: ['.'] });
    });

    test('manual language installs accumulate languages in shared config', () => {
      install({
        projectRoot: tmpDir,
        target: 'claude',
        language: 'typescript',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });
      install({
        projectRoot: tmpDir,
        target: 'claude',
        language: 'go',
        agents: ['linting', 'local-dev'],
        force: false,
        saveConfig: true
      });

      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.ballastVersion).toBe(BALLAST_VERSION);
      expect(raw.languages).toEqual(['typescript', 'go']);
      expect(raw.paths).toEqual({
        typescript: ['.'],
        go: ['.']
      });
    });

    test('adds .ballast/ to .gitignore during install', () => {
      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(
        fs.readFileSync(path.join(tmpDir, '.gitignore'), 'utf8')
      ).toContain('.ballast/');
    });

    test('does not duplicate .ballast/ in an existing .gitignore', () => {
      fs.writeFileSync(path.join(tmpDir, '.gitignore'), '.ballast/\n', 'utf8');

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(fs.readFileSync(path.join(tmpDir, '.gitignore'), 'utf8')).toBe(
        '.ballast/\n'
      );
    });

    test('appends .ballast/ to an existing .gitignore without trailing newline', () => {
      fs.writeFileSync(path.join(tmpDir, '.gitignore'), 'node_modules', 'utf8');

      install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(fs.readFileSync(path.join(tmpDir, '.gitignore'), 'utf8')).toBe(
        'node_modules\n.ballast/\n'
      );
    });

    test('records a gitignore error and continues install when .gitignore is unreadable', () => {
      fs.mkdirSync(path.join(tmpDir, '.gitignore'));

      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting'],
        force: false,
        saveConfig: false
      });

      expect(result.errors).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ agent: 'gitignore' })
        ])
      );
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        )
      ).toBe(true);
    });

    test('installs multiple agents', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting', 'docs', 'cicd'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toContain('linting');
      expect(result.installed).toContain('docs');
      expect(result.installed).toContain('cicd');
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        )
      ).toBe(true);
      expect(
        fs.existsSync(path.join(tmpDir, '.cursor', 'rules', 'docs.mdc'))
      ).toBe(true);
      expect(
        fs.existsSync(path.join(tmpDir, '.cursor', 'rules', 'cicd.mdc'))
      ).toBe(true);
      expect(result.installedRules).toContainEqual({
        agentId: 'docs',
        ruleSuffix: ''
      });
    });

    test('installs docs rule', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['docs'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['docs']);
      expect(result.installedRules).toEqual([
        {
          agentId: 'docs',
          ruleSuffix: ''
        }
      ]);
      const docsFile = path.join(tmpDir, '.cursor', 'rules', 'docs.mdc');
      expect(fs.existsSync(docsFile)).toBe(true);
      expect(fs.readFileSync(docsFile, 'utf8')).toContain(
        'Documentation Agent'
      );
      expect(fs.readFileSync(docsFile, 'utf8')).toContain('publish-docs');
    });

    test('installs all rules for agent with multiple rules (local-dev)', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['local-dev'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['local-dev']);
      expect(result.installedRules.length).toBe(4);
      const envFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-env.mdc'
      );
      const mcpFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-mcp.mdc'
      );
      const licenseFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-license.mdc'
      );
      const badgesFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'local-dev-badges.mdc'
      );
      expect(fs.existsSync(envFile)).toBe(true);
      expect(fs.existsSync(mcpFile)).toBe(true);
      expect(fs.existsSync(licenseFile)).toBe(true);
      expect(fs.existsSync(badgesFile)).toBe(true);
      expect(fs.readFileSync(envFile, 'utf8')).toContain(
        'Local Development Environment Agent'
      );
      expect(fs.readFileSync(mcpFile, 'utf8')).toContain('GitHub MCP');
      expect(fs.readFileSync(licenseFile, 'utf8')).toContain('LICENSE');
      expect(fs.readFileSync(badgesFile, 'utf8')).toContain('README Badges');
    });

    test('installs all rules for agent with multiple rules (publishing)', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['publishing'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['publishing']);
      expect(result.installedRules.length).toBe(8);
      const librariesFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-libraries.mdc'
      );
      const sdksFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-sdks.mdc'
      );
      const appsFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-apps.mdc'
      );
      const cliFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-cli.mdc'
      );
      const webFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-web.mdc'
      );
      const apiFile = path.join(
        tmpDir,
        '.cursor',
        'rules',
        'publishing-api.mdc'
      );
      expect(fs.existsSync(librariesFile)).toBe(true);
      expect(fs.existsSync(sdksFile)).toBe(true);
      expect(fs.existsSync(appsFile)).toBe(true);
      expect(fs.existsSync(cliFile)).toBe(true);
      expect(fs.existsSync(webFile)).toBe(true);
      expect(fs.existsSync(apiFile)).toBe(true);
      expect(fs.readFileSync(librariesFile, 'utf8')).toContain(
        'Publishing Libraries Agent'
      );
      expect(fs.readFileSync(sdksFile, 'utf8')).toContain(
        'Publishing SDKs Agent'
      );
      expect(fs.readFileSync(appsFile, 'utf8')).toContain(
        'Publishing Apps Agent'
      );
    });

    test('adds to errors for unknown agent and continues with valid ones', () => {
      const result = install({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: ['linting', 'unknown-agent', 'cicd'],
        force: false,
        saveConfig: false
      });
      expect(result.installed).toEqual(['linting', 'cicd', 'git-hooks']);
      expect(result.errors).toHaveLength(1);
      expect(result.errors[0]).toEqual({
        agent: 'unknown-agent',
        error: 'Unknown agent'
      });
    });

    describe('file locations', () => {
      test('cursor: writes to .cursor/rules/<agent>.mdc', () => {
        install({
          projectRoot: tmpDir,
          target: 'cursor',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'cursor', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        );
        expect(dir).toBe(path.join(tmpDir, '.cursor', 'rules'));
      });

      test('claude: writes to .claude/rules/<agent>.md', () => {
        install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'claude', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.claude', 'rules', 'typescript-linting.md')
        );
        expect(dir).toBe(path.join(tmpDir, '.claude', 'rules'));
        const claudeMd = getClaudeMdPath(tmpDir);
        expect(fs.existsSync(claudeMd)).toBe(true);
        expect(fs.readFileSync(claudeMd, 'utf8')).toContain(
          '`.claude/rules/typescript-linting.md`'
        );
      });

      test('claude patches existing CLAUDE.md by default', () => {
        fs.writeFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          `# CLAUDE.md

## Team Notes

Keep this section.

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.

Read and follow these rule files in \`.claude/rules/\` when they apply:

- \`.claude/rules/old.md\` — Old rule
`
        );

        const result = install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });

        expect(result.installedSupportFiles).toContain(
          path.join(tmpDir, 'CLAUDE.md')
        );
        expect(result.skippedSupportFiles).not.toContain(
          path.join(tmpDir, 'CLAUDE.md')
        );
        const claudeMd = fs.readFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          'utf8'
        );
        expect(claudeMd).toContain('## Team Notes');
        expect(claudeMd).toContain('`.claude/rules/typescript-linting.md`');
        expect(claudeMd).not.toContain('`.claude/rules/old.md`');
      });

      test('claude patch updates installed rules section without removing user notes', () => {
        const claudeRulesDir = path.join(tmpDir, '.claude', 'rules');
        fs.mkdirSync(claudeRulesDir, { recursive: true });
        fs.writeFileSync(
          path.join(claudeRulesDir, 'typescript-linting.md'),
          `# TypeScript Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
`
        );
        fs.writeFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          `# CLAUDE.md

## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in \`.claude/rules/\` when they apply:

- \`.claude/rules/old.md\` — Old rule
`
        );

        install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          patchClaudeMd: true,
          force: false,
          saveConfig: false
        });

        const claudeMd = fs.readFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          'utf8'
        );
        expect(claudeMd).toContain('## Team Notes');
        expect(claudeMd).toContain('Keep this section.');
        expect(claudeMd).toContain('`.claude/rules/typescript-linting.md`');
        expect(claudeMd).not.toContain('`.claude/rules/old.md`');
      });

      test('claude --patch updates installed rules section without prompt approval', () => {
        fs.writeFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          `# CLAUDE.md

## Installed agent rules

Read and follow these rule files in \`.claude/rules/\` when they apply:

- \`.claude/rules/old.md\` — Old rule
`
        );

        const result = install({
          projectRoot: tmpDir,
          target: 'claude',
          agents: ['linting'],
          patch: true,
          force: false,
          saveConfig: false
        });

        expect(result.installedSupportFiles).toContain(
          path.join(tmpDir, 'CLAUDE.md')
        );
        const claudeMd = fs.readFileSync(
          path.join(tmpDir, 'CLAUDE.md'),
          'utf8'
        );
        expect(claudeMd).toContain('`.claude/rules/typescript-linting.md`');
        expect(claudeMd).not.toContain('`.claude/rules/old.md`');
      });

      test('opencode: writes to .opencode/<agent>.md', () => {
        install({
          projectRoot: tmpDir,
          target: 'opencode',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'opencode', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.opencode', 'typescript-linting.md')
        );
        expect(dir).toBe(path.join(tmpDir, '.opencode'));
      });

      test('gemini patch updates installed rules section without removing user notes', () => {
        const geminiDir = path.join(tmpDir, '.gemini', 'rules');
        fs.mkdirSync(geminiDir, { recursive: true });
        fs.writeFileSync(
          path.join(geminiDir, 'typescript-linting.md'),
          `# TypeScript Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
`
        );
        fs.writeFileSync(
          path.join(tmpDir, 'GEMINI.md'),
          `# GEMINI.md

## Team Notes

Keep this section.

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.

Read and follow these rule files in \`.gemini/rules/\` when they apply:

- \`.gemini/rules/old.md\` — Old rule
`
        );

        install({
          projectRoot: tmpDir,
          target: 'gemini',
          agents: ['linting'],
          patch: true,
          force: false,
          saveConfig: false
        });

        const geminiRule = fs.readFileSync(
          path.join(geminiDir, 'typescript-linting.md'),
          'utf8'
        );
        expect(geminiRule).toContain('Keep my custom rule text.');
        expect(geminiRule).toContain('## When Completed');

        const geminiMd = fs.readFileSync(getGeminiMdPath(tmpDir), 'utf8');
        expect(geminiMd).toContain('## Team Notes');
        expect(geminiMd).toContain('Keep this section.');
        expect(geminiMd).toContain('`.gemini/rules/typescript-linting.md`');
        expect(geminiMd).not.toContain('`.gemini/rules/old.md`');
      });

      test('gemini creates GEMINI.md when missing', () => {
        install({
          projectRoot: tmpDir,
          target: 'gemini',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });

        const geminiMd = fs.readFileSync(getGeminiMdPath(tmpDir), 'utf8');
        expect(geminiMd).toContain('## Repository Facts');
        expect(geminiMd).toContain('## Memory Tiering');
        expect(geminiMd).toContain('`.gemini/rules/typescript-linting.md`');

        expect(fs.existsSync(path.join(tmpDir, 'AGENTS.md'))).toBe(false);
      });

      test('gemini patches existing GEMINI.md by default', () => {
        fs.writeFileSync(
          path.join(tmpDir, 'GEMINI.md'),
          `# GEMINI.md

## Team Notes

Keep this section.

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.

Read and follow these rule files in \`.gemini/rules/\` when they apply:

- \`.gemini/rules/old.md\` — Old rule
`
        );

        const result = install({
          projectRoot: tmpDir,
          target: 'gemini',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });

        expect(result.installedSupportFiles).toContain(
          path.join(tmpDir, 'GEMINI.md')
        );
        expect(result.skippedSupportFiles).not.toContain(
          path.join(tmpDir, 'GEMINI.md')
        );
        const geminiMd = fs.readFileSync(
          path.join(tmpDir, 'GEMINI.md'),
          'utf8'
        );
        expect(geminiMd).toContain('## Team Notes');
        expect(geminiMd).toContain('`.gemini/rules/typescript-linting.md`');
        expect(geminiMd).not.toContain('`.gemini/rules/old.md`');
      });

      test('codex: writes to .codex/rules/<agent>.md and AGENTS.md', () => {
        install({
          projectRoot: tmpDir,
          target: 'codex',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });
        const { dir, file } = getDestination('linting', 'codex', tmpDir);
        expect(fs.existsSync(file)).toBe(true);
        expect(file).toBe(
          path.join(tmpDir, '.codex', 'rules', 'typescript-linting.md')
        );
        expect(dir).toBe(path.join(tmpDir, '.codex', 'rules'));
        const agentsMd = path.join(tmpDir, 'AGENTS.md');
        expect(fs.existsSync(agentsMd)).toBe(true);
        expect(fs.readFileSync(agentsMd, 'utf8')).toContain(
          '`.codex/rules/typescript-linting.md`'
        );
      });

      test('codex patch updates installed rules section without removing user notes', () => {
        const codexDir = path.join(tmpDir, '.codex', 'rules');
        fs.mkdirSync(codexDir, { recursive: true });
        fs.writeFileSync(
          path.join(codexDir, 'typescript-linting.md'),
          `# TypeScript Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
`
        );
        fs.writeFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          `# AGENTS.md

## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in \`.codex/rules/\` when they apply:

- \`.codex/rules/old.md\` — Old rule
`
        );

        install({
          projectRoot: tmpDir,
          target: 'codex',
          agents: ['linting'],
          patch: true,
          force: false,
          saveConfig: false
        });

        const codexRule = fs.readFileSync(
          path.join(codexDir, 'typescript-linting.md'),
          'utf8'
        );
        expect(codexRule).toContain('Keep my custom rule text.');
        expect(codexRule).toContain('## When Completed');

        const agentsMd = fs.readFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          'utf8'
        );
        expect(agentsMd).toContain('## Team Notes');
        expect(agentsMd).toContain('Keep this section.');
        expect(agentsMd).toContain('`.codex/rules/typescript-linting.md`');
        expect(agentsMd).not.toContain('`.codex/rules/old.md`');
      });

      test('codex default patch preserves unmanaged installed rules section', () => {
        fs.writeFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          `# AGENTS.md

## Installed agent rules

- \`.codex/rules/old.md\` — Team managed rule
`
        );

        install({
          projectRoot: tmpDir,
          target: 'codex',
          agents: ['linting'],
          force: false,
          saveConfig: false
        });

        const agentsMd = fs.readFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          'utf8'
        );
        expect(agentsMd).toContain('`.codex/rules/old.md`');
        expect(agentsMd).toContain('`.codex/rules/typescript-linting.md`');
      });

      test('codex patch keeps rules in AGENTS.md for config-backed skill-only updates', () => {
        saveConfig(
          {
            targets: ['codex'],
            agents: ['linting'],
            skills: ['owasp-security-scan']
          },
          tmpDir
        );
        fs.writeFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          buildCodexAgentsMd(['linting'], ['owasp-security-scan'])
        );

        const result = install({
          projectRoot: tmpDir,
          target: 'codex',
          agents: [],
          skills: ['owasp-security-scan', 'github-health-check'],
          patch: true,
          force: false,
          saveConfig: false
        });

        expect(result.errors).toHaveLength(0);
        const agentsMd = fs.readFileSync(
          path.join(tmpDir, 'AGENTS.md'),
          'utf8'
        );
        expect(agentsMd).toContain('`.codex/rules/typescript-linting.md`');
        expect(agentsMd).toContain('`.codex/rules/owasp-security-scan.md`');
        expect(agentsMd).not.toContain('`.codex/rules/github-health-check.md`');
      });

      test('written path matches getDestination for each target', () => {
        const targets: Array<{
          target: 'cursor' | 'claude' | 'opencode' | 'codex' | 'gemini';
          ext: string;
        }> = [
          { target: 'cursor', ext: 'mdc' },
          { target: 'claude', ext: 'md' },
          { target: 'opencode', ext: 'md' },
          { target: 'codex', ext: 'md' },
          { target: 'gemini', ext: 'md' }
        ];
        for (const { target, ext } of targets) {
          const subDir = path.join(tmpDir, target);
          fs.mkdirSync(subDir, { recursive: true });
          install({
            projectRoot: subDir,
            target,
            agents: ['linting'],
            force: false,
            saveConfig: false
          });
          const { file: expectedFile } = getDestination(
            'linting',
            target,
            subDir
          );
          expect(fs.existsSync(expectedFile)).toBe(true);
          expect(path.extname(expectedFile)).toBe(
            ext === 'mdc' ? '.mdc' : '.md'
          );
        }
      });
    });
  });

  describe('runInstall', () => {
    test('writes files for every requested target in one run', async () => {
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        targets: ['cursor', 'claude'],
        agents: ['linting'],
        yes: true
      });
      expect(exitCode).toBe(0);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'typescript-linting.mdc')
        )
      ).toBe(true);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.claude', 'rules', 'typescript-linting.md')
        )
      ).toBe(true);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.targets).toEqual(['cursor', 'claude']);
    });

    test('defaults deploymentModel to none for non-interactive publishing installs', async () => {
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'codex',
        agents: ['publishing'],
        yes: true
      });

      expect(exitCode).toBe(0);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.deploymentModel).toBe('none');
    });

    test('stores explicit deploymentModel for publishing installs', async () => {
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'codex',
        agents: ['publishing'],
        deploymentModel: 'kubernetes',
        yes: true
      });

      expect(exitCode).toBe(0);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.deploymentModel).toBe('kubernetes');
      expect(
        fs.readFileSync(
          path.join(tmpDir, '.codex', 'rules', 'publishing-apps.md'),
          'utf8'
        )
      ).toContain('charts/<app>/');
    });

    test('rejects invalid deploymentModel values', async () => {
      const errorSpy = jest.spyOn(console, 'error').mockImplementation();

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'codex',
        agents: ['publishing'],
        deploymentModel: 'kuberntes',
        yes: true
      });

      expect(exitCode).toBe(1);
      expect(errorSpy).toHaveBeenCalledWith(
        'Invalid --deployment-model value: "kuberntes". Valid values: none, kubernetes, serverless, server, hosted'
      );
    });

    test('prompts for deploymentModel when publishing is selected interactively', async () => {
      const answers = ['codex', 'publishing', '', 'hosted'];
      const createInterfaceSpy = jest
        .spyOn(readline, 'createInterface')
        .mockImplementation(
          () =>
            ({
              question: (_prompt: string, cb: (answer: string) => void) =>
                cb(answers.shift() ?? ''),
              close: () => {}
            }) as unknown as readline.Interface
        );

      const exitCode = await runInstall({ projectRoot: tmpDir });

      expect(exitCode).toBe(0);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.deploymentModel).toBe('hosted');
      createInterfaceSpy.mockRestore();
    });

    test('preserves existing deploymentModel when adding another agent', async () => {
      saveConfig(
        {
          targets: ['codex'],
          agents: ['publishing'],
          deploymentModel: 'serverless'
        },
        tmpDir
      );

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'codex',
        agents: ['docs'],
        yes: true
      });

      expect(exitCode).toBe(0);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.deploymentModel).toBe('serverless');
    });

    test('writes files to correct locations for given target', async () => {
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'claude',
        agents: ['linting'],
        yes: true
      });
      expect(exitCode).toBe(0);
      const { file } = getDestination('linting', 'claude', tmpDir);
      expect(fs.existsSync(file)).toBe(true);
      expect(file).toBe(
        path.join(tmpDir, '.claude', 'rules', 'typescript-linting.md')
      );
      expect(fs.existsSync(path.join(tmpDir, '.rulesrc.json'))).toBe(true);
    });

    test('retains configured agents when adding a skill without agent flags', async () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          skills: []
        },
        tmpDir
      );

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'cursor',
        skills: ['owasp-security-scan'],
        yes: true
      });

      expect(exitCode).toBe(0);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.agents).toEqual(['linting', 'git-hooks']);
      expect(raw.skills).toEqual(['owasp-security-scan']);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.cursor', 'rules', 'owasp-security-scan.mdc')
        )
      ).toBe(true);
    });

    test('retains configured agents when adding a skill with empty agents array (CLI default)', async () => {
      saveConfig(
        {
          targets: ['cursor'],
          agents: ['linting'],
          skills: []
        },
        tmpDir
      );

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'cursor',
        agents: [],
        skills: ['owasp-security-scan'],
        yes: true
      });

      expect(exitCode).toBe(0);
      const raw = JSON.parse(
        fs.readFileSync(path.join(tmpDir, '.rulesrc.json'), 'utf8')
      );
      expect(raw.agents).toEqual(['linting', 'git-hooks']);
      expect(raw.skills).toEqual(['owasp-security-scan']);
    });

    test('uses saved config when CLI passes empty agent and skill arrays', async () => {
      saveConfig(
        {
          targets: ['codex'],
          agents: ['linting'],
          skills: ['owasp-security-scan']
        },
        tmpDir
      );

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        agents: [],
        skills: [],
        yes: true
      });

      expect(exitCode).toBe(0);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.codex', 'rules', 'typescript-linting.md')
        )
      ).toBe(true);
      expect(
        fs.existsSync(
          path.join(tmpDir, '.codex', 'rules', 'owasp-security-scan.md')
        )
      ).toBe(true);
    });

    test('returns 1 when CI and no config and no target/agents', async () => {
      process.env.CI = 'true';
      const exitCode = await runInstall({
        projectRoot: tmpDir,
        yes: true
      });
      expect(exitCode).toBe(1);
    });

    test('prompts before force-overwriting an existing support file and skips on no', async () => {
      const agentsMdPath = path.join(tmpDir, 'AGENTS.md');
      fs.writeFileSync(
        agentsMdPath,
        '# AGENTS.md\n\nTeam customizations.\n',
        'utf8'
      );
      const consoleLog = jest
        .spyOn(console, 'log')
        .mockImplementation(() => {});
      jest.spyOn(readline, 'createInterface').mockImplementation(
        () =>
          ({
            question: (_prompt: string, cb: (answer: string) => void) =>
              cb('n'),
            close: () => {}
          }) as unknown as readline.Interface
      );

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'codex',
        skills: ['owasp-security-scan'],
        force: true
      });

      expect(exitCode).toBe(0);
      expect(fs.readFileSync(agentsMdPath, 'utf8')).toContain(
        'Team customizations.'
      );
      expect(consoleLog).toHaveBeenCalledWith(
        expect.stringContaining('Skipped support file: AGENTS.md')
      );
    });

    test('force-overwrites an existing support file when prompt is accepted', async () => {
      const agentsMdPath = path.join(tmpDir, 'AGENTS.md');
      fs.writeFileSync(
        agentsMdPath,
        '# AGENTS.md\n\nTeam customizations.\n',
        'utf8'
      );
      jest.spyOn(readline, 'createInterface').mockImplementation(
        () =>
          ({
            question: (_prompt: string, cb: (answer: string) => void) =>
              cb('y'),
            close: () => {}
          }) as unknown as readline.Interface
      );

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'codex',
        skills: ['owasp-security-scan'],
        force: true
      });

      expect(exitCode).toBe(0);
      const content = fs.readFileSync(agentsMdPath, 'utf8');
      expect(content).toContain('## Installed skills');
      expect(content).not.toContain('Team customizations.');
    });

    test('refuses force-overwriting an existing support file in non-interactive mode', async () => {
      const agentsMdPath = path.join(tmpDir, 'AGENTS.md');
      fs.writeFileSync(
        agentsMdPath,
        '# AGENTS.md\n\nTeam customizations.\n',
        'utf8'
      );
      const consoleError = jest
        .spyOn(console, 'error')
        .mockImplementation(() => {});

      const exitCode = await runInstall({
        projectRoot: tmpDir,
        target: 'codex',
        skills: ['owasp-security-scan'],
        force: true,
        yes: true
      });

      expect(exitCode).toBe(1);
      expect(fs.readFileSync(agentsMdPath, 'utf8')).toContain(
        'Team customizations.'
      );
      expect(consoleError).toHaveBeenCalledWith(
        expect.stringContaining(
          'Cannot overwrite existing support file AGENTS.md in non-interactive mode'
        )
      );
    });
  });
});
