import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const packageRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(packageRoot, '..', '..');
const sourceAgents = path.join(repoRoot, 'agents');
const targetAgents = path.join(packageRoot, 'agents');

if (!fs.existsSync(sourceAgents)) {
  throw new Error(`Missing source agents directory: ${sourceAgents}`);
}

fs.rmSync(targetAgents, { recursive: true, force: true });
fs.cpSync(sourceAgents, targetAgents, { recursive: true });
