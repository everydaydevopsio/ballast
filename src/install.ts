import fs from 'fs';
import readline from 'readline';
import {
  buildContent,
  buildCodexAgentsMd,
  getCodexAgentsMdPath,
  getDestination,
  listRuleSuffixes,
  listTargets
} from './build';
import { listAgents, resolveAgents, isValidAgent } from './agents';
import { findProjectRoot, loadConfig, saveConfig, isCiMode } from './config';
import type { Target } from './config';

function prompt(question: string): Promise<string> {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
  });
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve((answer || '').trim());
    });
  });
}

/**
 * Interactive: ask for target (cursor | claude | opencode | codex)
 */
async function promptTarget(): Promise<Target> {
  const targets = listTargets();
  const line = await prompt(`AI platform (${targets.join(', ')}): `);
  const t = line.toLowerCase();
  if (targets.includes(t)) return t as Target;
  console.error(`Invalid target. Choose one of: ${targets.join(', ')}`);
  return promptTarget();
}

/**
 * Interactive: ask for agents (comma-separated or "all")
 */
async function promptAgents(): Promise<string[]> {
  const agents = listAgents();
  const line = await prompt(
    `Agents (comma-separated or "all") [${agents.join(', ')}]: `
  );
  if (!line) return agents;
  const list =
    line.toLowerCase() === 'all'
      ? ['all']
      : line.split(',').map((s) => s.trim());
  const resolved = resolveAgents(list);
  if (resolved.length === 0) {
    console.error(
      `Invalid agents. Use "all" or comma-separated: ${agents.join(', ')}`
    );
    return promptAgents();
  }
  return resolved;
}

export interface ResolveTargetAndAgentsOptions {
  projectRoot?: string;
  target?: string;
  agents?: string | string[];
  yes?: boolean;
}

/**
 * Resolve target and agents from config + flags; in CI with no config, return null and caller should exit with error
 */
export async function resolveTargetAndAgents(
  options: ResolveTargetAndAgentsOptions = {}
): Promise<{ target: Target; agents: string[] } | null> {
  const projectRoot = options.projectRoot ?? findProjectRoot();
  const config = loadConfig(projectRoot);
  const ci = isCiMode() || options.yes;

  const targetFromFlag = options.target;
  const agentsFromFlag = options.agents;

  if (config && !targetFromFlag && agentsFromFlag === undefined) {
    return { target: config.target, agents: config.agents };
  }

  const target = targetFromFlag ?? config?.target;
  const agents =
    agentsFromFlag != null ? resolveAgents(agentsFromFlag) : config?.agents;

  if (target && agents && agents.length > 0) {
    return { target: target as Target, agents };
  }

  if (ci) {
    return null;
  }

  const resolvedTarget = (target ?? (await promptTarget())) as Target;
  const resolvedAgents = agents?.length ? agents : await promptAgents();
  return { target: resolvedTarget, agents: resolvedAgents };
}

export interface InstallOptions {
  projectRoot: string;
  target: Target;
  agents: string[];
  force?: boolean;
  saveConfig?: boolean;
}

export interface InstallResult {
  installed: string[];
  /** Per-rule: which (agent, ruleSuffix) files were written (ruleSuffix '' = main) */
  installedRules: Array<{ agentId: string; ruleSuffix: string }>;
  installedSupportFiles: string[];
  skipped: string[];
  skippedSupportFiles: string[];
  errors: Array<{ agent: string; error: string }>;
}

/**
 * Install agents for the given target into projectRoot. Single policy: do not overwrite unless force.
 */
export function install(options: InstallOptions): InstallResult {
  const {
    projectRoot,
    target,
    agents,
    force = false,
    saveConfig: persist
  } = options;
  const installed: string[] = [];
  const installedRules: Array<{ agentId: string; ruleSuffix: string }> = [];
  const installedSupportFiles: string[] = [];
  const skipped: string[] = [];
  const skippedSupportFiles: string[] = [];
  const errors: Array<{ agent: string; error: string }> = [];
  const validAgents = new Set<string>();

  if (persist) {
    saveConfig({ target, agents }, projectRoot);
  }

  for (const agentId of agents) {
    if (!isValidAgent(agentId)) {
      errors.push({ agent: agentId, error: 'Unknown agent' });
      continue;
    }
    validAgents.add(agentId);
    let agentInstalled = false;
    let agentSkipped = false;
    try {
      const suffixes = listRuleSuffixes(agentId);
      for (const ruleSuffix of suffixes) {
        const { dir, file } = getDestination(
          agentId,
          target,
          projectRoot,
          ruleSuffix || undefined
        );
        if (fs.existsSync(file) && !force) {
          agentSkipped = true;
          continue;
        }
        if (!fs.existsSync(dir)) {
          fs.mkdirSync(dir, { recursive: true });
        }
        const content = buildContent(agentId, target, ruleSuffix || undefined);
        fs.writeFileSync(file, content, 'utf8');
        installedRules.push({ agentId, ruleSuffix: ruleSuffix || '' });
        agentInstalled = true;
      }
      if (agentInstalled) installed.push(agentId);
      if (agentSkipped && !agentInstalled) skipped.push(agentId);
    } catch (err) {
      errors.push({
        agent: agentId,
        error: err instanceof Error ? err.message : String(err)
      });
    }
  }

  if (target === 'codex') {
    const agentsMdPath = getCodexAgentsMdPath(projectRoot);
    if (fs.existsSync(agentsMdPath) && !force) {
      skippedSupportFiles.push(agentsMdPath);
    } else {
      const content = buildCodexAgentsMd(Array.from(validAgents));
      fs.writeFileSync(agentsMdPath, content, 'utf8');
      installedSupportFiles.push(agentsMdPath);
    }
  }

  return {
    installed,
    installedRules,
    installedSupportFiles,
    skipped,
    skippedSupportFiles,
    errors
  };
}

export interface RunInstallOptions {
  projectRoot?: string;
  target?: string;
  agents?: string[];
  all?: boolean;
  force?: boolean;
  yes?: boolean;
}

/**
 * Run install flow: resolve target/agents (interactive or from config/flags), then install.
 * In CI mode with no .rulesrc.json, --target and (--agent X or --all) are required.
 */
export async function runInstall(
  options: RunInstallOptions = {}
): Promise<number> {
  const projectRoot = options.projectRoot ?? findProjectRoot();
  const agentsFromFlag = options.all ? 'all' : options.agents;
  const resolved = await resolveTargetAndAgents({
    projectRoot,
    target: options.target,
    agents: agentsFromFlag,
    yes: options.yes
  });

  if (!resolved) {
    console.error(
      'In CI/non-interactive mode (--yes or CI env), --target and --agent (or --all) are required when .rulesrc.json is missing.'
    );
    console.error(
      'Example: ballast install --yes --target cursor --agent linting'
    );
    return 1;
  }

  const { target, agents } = resolved;
  const persist = !options.target && !options.agents && !options.all;
  const result = install({
    projectRoot,
    target,
    agents,
    force: options.force ?? false,
    saveConfig: persist
  });

  if (result.errors.length > 0) {
    result.errors.forEach(({ agent, error }) => {
      console.error(`Error installing ${agent}: ${error}`);
    });
    return 1;
  }

  if (result.installedRules.length > 0) {
    console.log(`Installed for ${target}: ${result.installed.join(', ')}`);
    result.installedRules.forEach(({ agentId, ruleSuffix }) => {
      const { file } = getDestination(
        agentId,
        target,
        projectRoot,
        ruleSuffix || undefined
      );
      const label = ruleSuffix ? `${agentId}-${ruleSuffix}` : agentId;
      console.log(`  ${label} -> ${file}`);
    });
  }
  if (result.installedSupportFiles.length > 0) {
    result.installedSupportFiles.forEach((file) => {
      console.log(`  AGENTS.md -> ${file}`);
    });
  }
  if (result.skipped.length > 0) {
    console.log(
      `Skipped (already present; use --force to overwrite): ${result.skipped.join(', ')}`
    );
  }
  if (result.skippedSupportFiles.length > 0) {
    console.log(
      `Skipped support files (already present; use --force to overwrite): ${result.skippedSupportFiles.join(
        ', '
      )}`
    );
  }
  if (
    result.installed.length === 0 &&
    result.skipped.length === 0 &&
    result.errors.length === 0
  ) {
    console.log('Nothing to install.');
  }

  return 0;
}
