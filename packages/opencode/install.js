#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');
const {
  buildOpenCodeFormat
} = require('@everydaydevops/typescript-linting-core');

const AGENT_FILENAME = 'typescript-linting.md';

function install() {
  // Determine if this is a global installation
  const isGlobalInstall = process.env.npm_config_global === 'true';

  // Get project root for local installs
  const projectRoot = process.env.npm_config_local_prefix || process.cwd();

  // Determine target directory
  const targetDir = isGlobalInstall
    ? path.join(os.homedir(), '.config', 'opencode', 'agent')
    : path.join(projectRoot, '.opencode');

  const targetFile = path.join(targetDir, AGENT_FILENAME);

  // Create directory if needed
  if (!fs.existsSync(targetDir)) {
    fs.mkdirSync(targetDir, { recursive: true });
  }

  // Build and write the agent file
  try {
    const content = buildOpenCodeFormat();
    fs.writeFileSync(targetFile, content);

    console.log('OpenCode TypeScript Linting agent installed successfully!');
    console.log(`  Installed to: ${targetFile}`);
    console.log(`  Installation type: ${isGlobalInstall ? 'global' : 'local'}`);
    console.log('');
    console.log('Usage:');
    console.log('  Run "opencode" in any TypeScript project');
    console.log('  Then use: @typescript-linting help me set up linting');
  } catch (error) {
    console.error(
      'Failed to install OpenCode TypeScript Linting agent:',
      error.message
    );
    process.exit(1);
    return;
  }
}

// Run if this is the main module
if (require.main === module) {
  install();
}

module.exports = { install };
