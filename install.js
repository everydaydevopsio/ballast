#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');

const agentName = 'typescript-linting.md';
const sourceFile = path.join(__dirname, 'agent', agentName);

// Determine the target directory
const globalConfigDir = path.join(os.homedir(), '.config', 'opencode', 'agent');

// Create the directory if it doesn't exist
if (!fs.existsSync(globalConfigDir)) {
  fs.mkdirSync(globalConfigDir, { recursive: true });
}

const targetFile = path.join(globalConfigDir, agentName);

// Copy the agent file
try {
  fs.copyFileSync(sourceFile, targetFile);
  console.log('✓ TypeScript Linting agent installed successfully!');
  console.log(`  Installed to: ${targetFile}`);
  console.log('\nUsage:');
  console.log('  Run "opencode" in any TypeScript project');
  console.log('  The typescript-linting subagent will be available');
  console.log(
    '  Primary agents can invoke it automatically for linting setup tasks'
  );
  console.log('\nManual invocation:');
  console.log('  @typescript-linting help me set up linting for this project');
} catch (error) {
  console.error('Failed to install TypeScript Linting agent:', error.message);
  process.exit(1);
}
