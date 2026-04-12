#!/usr/bin/env node

import { runInstall } from './install';
import { LANGUAGES, Language, AGENT_IDS, SKILL_IDS } from './agents';
import { runDoctor } from './doctor';
import { BALLAST_VERSION } from './version';

export interface CliOptions {
  targets: string[];
  agents: string[];
  skills: string[];
  language: string;
  all: boolean;
  allSkills: boolean;
  force: boolean;
  patch: boolean;
  yes: boolean;
}

export type ParseArgsResult =
  | CliOptions
  | { help: true }
  | { version: true }
  | { doctor: true }
  | { list: true };

export function parseArgs(argv: string[]): ParseArgsResult {
  const args = argv.slice(2);
  const command = args[0];
  if (command === 'doctor') {
    return { doctor: true };
  }
  if (command === 'list') {
    return { list: true };
  }
  const options: CliOptions = {
    targets: [],
    agents: [],
    skills: [],
    language: 'typescript',
    all: false,
    allSkills: false,
    force: false,
    patch: false,
    yes: false
  };
  let i = 0;
  while (i < args.length) {
    const arg = args[i];
    if (arg === '--target' || arg === '-t') {
      const value = args[++i];
      if (value) {
        options.targets = options.targets.concat(
          value.split(',').map((s) => s.trim())
        );
      }
      i++;
      continue;
    }
    if (arg === '--agent' || arg === '-a') {
      const value = args[++i];
      if (value) {
        options.agents = options.agents.concat(
          value.split(',').map((s) => s.trim())
        );
      }
      i++;
      continue;
    }
    if (arg === '--language' || arg === '-l') {
      const value = args[++i];
      if (value) options.language = value.trim().toLowerCase();
      i++;
      continue;
    }
    if (arg === '--skill' || arg === '-s') {
      const value = args[++i];
      if (value) {
        options.skills = options.skills.concat(
          value.split(',').map((s) => s.trim())
        );
      }
      i++;
      continue;
    }
    if (arg === '--all') {
      options.all = true;
      i++;
      continue;
    }
    if (arg === '--all-skills') {
      options.allSkills = true;
      i++;
      continue;
    }
    if (arg === '--force' || arg === '-f') {
      options.force = true;
      i++;
      continue;
    }
    if (arg === '--patch' || arg === '-p') {
      options.patch = true;
      i++;
      continue;
    }
    if (arg === '--yes' || arg === '-y') {
      options.yes = true;
      i++;
      continue;
    }
    if (arg === '--help' || arg === '-h') {
      return { help: true };
    }
    if (arg === '--version' || arg === '-v') {
      return { version: true };
    }
    i++;
  }
  return options;
}

export function printHelp(): void {
  const pkg = require('../package.json');
  console.log(`
${pkg.name} v${pkg.version}

Usage: ballast-typescript <command> [options]

Commands:
  install    Install agent rules for the chosen AI platform (default)
  list       List available agents and skills
  doctor     Check local Ballast CLI versions and .rulesrc.json metadata

Options:
  --target, -t <platforms>  AI platform(s): cursor, claude, opencode, codex (comma-separated or repeated)
  --language, -l <lang>     Language profile: ${LANGUAGES.join(', ')} (default: typescript)
  --agent, -a <agents>      Agent(s) to install (comma-separated); run 'list' to see available agents
  --skill, -s <skills>      Skill(s) to install (comma-separated); run 'list' to see available skills
  --all                     Install all agents
  --all-skills              Install all skills
  --force, -f               Overwrite existing rule files
  --patch, -p               Merge upstream rule updates into existing files; ignored when --force is set
  --yes, -y                 Non-interactive; require --target and --agent/--all if no .rulesrc.json
  --help, -h                Show this help
  --version, -v             Show version

Examples:
  ballast-typescript list
  ballast-typescript install
  ballast-typescript install --target cursor --agent linting
  ballast-typescript install --target cursor,claude --all
  ballast-typescript install --language python --target cursor --all
  ballast-typescript install --target claude --all --force
  ballast-typescript install --target claude --skill owasp-security-scan
  ballast-typescript install --target cursor --agent linting --patch
  ballast-typescript install --yes --target cursor --all
`);
}

export function printList(): void {
  console.log(`
Agents
------
${AGENT_IDS.map((id) => `  ${id}`).join('\n')}

Skills
------
${SKILL_IDS.map((id) => `  ${id}`).join('\n')}
`);
}

export function printVersion(): void {
  console.log(BALLAST_VERSION);
}

export async function main(): Promise<void> {
  const argv = process.argv;
  const command = argv[2];
  const isInstall = !command || command === 'install';

  if (argv.includes('--help') || argv.includes('-h')) {
    printHelp();
    process.exit(0);
  }
  if (argv.includes('--version') || argv.includes('-v')) {
    printVersion();
    process.exit(0);
  }

  if (!isInstall) {
    if (command === 'doctor') {
      process.exit(runDoctor());
    }
    if (command === 'list') {
      printList();
      process.exit(0);
    }
    console.error(`Unknown command: ${command}`);
    console.error('Run ballast-typescript --help for usage.');
    process.exit(1);
  }

  const options = parseArgs(argv);
  if ('help' in options && options.help) {
    printHelp();
    process.exit(0);
  }
  if ('version' in options && options.version) {
    printVersion();
    process.exit(0);
  }
  if ('doctor' in options && options.doctor) {
    process.exit(runDoctor());
  }
  if ('list' in options && options.list) {
    printList();
    process.exit(0);
  }
  const cliOptions = options as CliOptions;
  if (!LANGUAGES.includes(cliOptions.language as (typeof LANGUAGES)[number])) {
    console.error(
      `Invalid language: ${cliOptions.language}. Choose one of: ${LANGUAGES.join(', ')}`
    );
    process.exit(1);
  }

  const normalizedOptions: Parameters<typeof runInstall>[0] = {
    ...cliOptions,
    language: cliOptions.language as Language,
    targets: cliOptions.targets,
    agents: cliOptions.agents.length > 0 ? cliOptions.agents : undefined,
    skills: cliOptions.skills.length > 0 ? cliOptions.skills : undefined
  };

  const exitCode = await runInstall(normalizedOptions);
  process.exit(exitCode);
}

if (require.main === module) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
