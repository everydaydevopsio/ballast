#!/usr/bin/env node

import { runInstall } from './install';

export interface CliOptions {
  target?: string;
  agents: string[];
  all: boolean;
  force: boolean;
  yes: boolean;
}

export type ParseArgsResult = CliOptions | { help: true } | { version: true };

function parseArgs(argv: string[]): ParseArgsResult {
  const args = argv.slice(2);
  const options: CliOptions = {
    target: undefined,
    agents: [],
    all: false,
    force: false,
    yes: false
  };
  let i = 0;
  while (i < args.length) {
    const arg = args[i];
    if (arg === '--target' || arg === '-t') {
      options.target = args[++i];
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
    if (arg === '--all') {
      options.all = true;
      i++;
      continue;
    }
    if (arg === '--force' || arg === '-f') {
      options.force = true;
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

function printHelp(): void {
  const pkg = require('../package.json');
  console.log(`
${pkg.name} v${pkg.version}

Usage: ballast install [options]

Commands:
  install    Install agent rules for the chosen AI platform (default)

Options:
  --target, -t <platform>   AI platform: cursor, claude, opencode
  --agent, -a <agents>      Agent(s): linting, local-dev, cicd, observability (comma-separated)
  --all                     Install all agents
  --force, -f               Overwrite existing rule files
  --yes, -y                 Non-interactive; require --target and --agent/--all if no .rulesrc.json
  --help, -h                Show this help
  --version, -v             Show version

Examples:
  ballast install
  ballast install --target cursor --agent linting
  ballast install --target claude --all --force
  ballast install --yes --target cursor --all
`);
}

function printVersion(): void {
  const pkg = require('../package.json');
  console.log(pkg.version);
}

async function main(): Promise<void> {
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
    console.error(`Unknown command: ${command}`);
    console.error('Run ballast --help for usage.');
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

  const exitCode = await runInstall(
    options as Parameters<typeof runInstall>[0]
  );
  process.exit(exitCode);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
