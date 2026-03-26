import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const packageRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(packageRoot, '..', '..');
const sourceAgents = path.join(repoRoot, 'agents');
const targetAgents = path.join(packageRoot, 'agents');
const sourceSkills = path.join(repoRoot, 'skills');
const targetSkills = path.join(packageRoot, 'skills');

if (!fs.existsSync(sourceAgents)) {
  throw new Error(`Missing source agents directory: ${sourceAgents}`);
}
if (!fs.existsSync(sourceSkills)) {
  throw new Error(`Missing source skills directory: ${sourceSkills}`);
}

fs.rmSync(targetAgents, { recursive: true, force: true });
fs.cpSync(sourceAgents, targetAgents, { recursive: true });
fs.rmSync(targetSkills, { recursive: true, force: true });
fs.cpSync(sourceSkills, targetSkills, { recursive: true });
