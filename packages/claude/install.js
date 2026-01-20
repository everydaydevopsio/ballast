#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');
const {
  buildClaudeFormat
} = require('@everydaydevops/typescript-linting-core');

const RULES_FILENAME = 'typescript-linting.md';

function install() {
  // Determine if this is a global installation
  const isGlobalInstall = process.env.npm_config_global === 'true';

  // Get project root for local installs
  const projectRoot = process.env.npm_config_local_prefix || process.cwd();

  // Determine target directory
  const targetDir = isGlobalInstall
    ? path.join(os.homedir(), '.claude', 'rules')
    : path.join(projectRoot, '.claude', 'rules');

  const targetFile = path.join(targetDir, RULES_FILENAME);

  // Check if file already exists (don't overwrite user's custom rules)
  if (fs.existsSync(targetFile)) {
    console.log('Claude Code TypeScript Linting rules: Skipped');
    console.log(`  ${targetFile} already exists`);
    console.log('  Existing rules preserved to protect your customizations');
    process.exit(0);
    return;
  }

  // Create directory if needed
  if (!fs.existsSync(targetDir)) {
    fs.mkdirSync(targetDir, { recursive: true });
  }

  // Build and write the rules file
  try {
    const content = buildClaudeFormat();
    fs.writeFileSync(targetFile, content);

    console.log('Claude Code TypeScript Linting rules installed successfully!');
    console.log(`  Installed to: ${targetFile}`);
    console.log(`  Installation type: ${isGlobalInstall ? 'global' : 'local'}`);
    console.log('');
    console.log('Usage:');
    console.log(
      '  Rules are automatically loaded when working in this project'
    );
    console.log('  Simply ask Claude to help with linting setup');
  } catch (error) {
    console.error(
      'Failed to install Claude Code TypeScript Linting rules:',
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
