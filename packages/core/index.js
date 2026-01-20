const fs = require('fs');
const path = require('path');

const CORE_ROOT = __dirname;

/**
 * Read the core content file
 */
function getContent() {
  const contentPath = path.join(CORE_ROOT, 'src', 'content.md');
  return fs.readFileSync(contentPath, 'utf8');
}

/**
 * Read a template file
 */
function getTemplate(filename) {
  const templatePath = path.join(CORE_ROOT, 'templates', filename);
  return fs.readFileSync(templatePath, 'utf8');
}

/**
 * Build OpenCode format (YAML frontmatter + content)
 */
function buildOpenCodeFormat() {
  const frontmatter = getTemplate('opencode-frontmatter.yaml');
  const content = getContent();
  return frontmatter + '\n' + content;
}

/**
 * Build Cursor format (MDC frontmatter + content)
 */
function buildCursorFormat() {
  const frontmatter = getTemplate('cursor-frontmatter.yaml');
  const content = getContent();
  return frontmatter + '\n' + content;
}

/**
 * Build Claude Code format (header + content)
 */
function buildClaudeFormat() {
  const header = getTemplate('claude-header.md');
  const content = getContent();
  return header + content;
}

module.exports = {
  buildOpenCodeFormat,
  buildCursorFormat,
  buildClaudeFormat,
  getContent,
  getTemplate
};
