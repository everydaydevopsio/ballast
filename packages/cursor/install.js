#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const {
  buildCursorFormat
} = require('@everydaydevops/typescript-linting-core');

const RULES_FILENAME = 'typescript-linting.mdc';

function install() {
  // Determine if this is a global installation
  const isGlobalInstall = process.env.npm_config_global === 'true';

  // Cursor global rules are configured in Settings UI, not files
  if (isGlobalInstall) {
    console.log('Cursor TypeScript Linting rules: Skipped');
    console.log(
      '  Global Cursor rules must be configured in Cursor Settings > Rules'
    );
    console.log(
      '  Use local installation (npm install) for project-specific rules'
    );
    process.exit(0);
    return;
  }

  // Get project root for local installs
  const projectRoot = process.env.npm_config_local_prefix || process.cwd();

  // Determine target directory
  const targetDir = path.join(projectRoot, '.cursor', 'rules');
  const targetFile = path.join(targetDir, RULES_FILENAME);

  // Check if file already exists (don't overwrite user's custom rules)
  if (fs.existsSync(targetFile)) {
    console.log('Cursor TypeScript Linting rules: Skipped');
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
    const content = buildCursorFormat();
    fs.writeFileSync(targetFile, content);

    console.log('Cursor TypeScript Linting rules installed successfully!');
    console.log(`  Installed to: ${targetFile}`);
    console.log('');
    console.log('Usage:');
    console.log('  Rules auto-apply to TypeScript/JavaScript files');
    console.log('  Or invoke with @typescript-linting in chat');
  } catch (error) {
    console.error(
      'Failed to install Cursor TypeScript Linting rules:',
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
