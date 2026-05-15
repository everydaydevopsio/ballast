import path from 'path';
import { spawnSync } from 'child_process';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const scriptPath = path.join(__dirname, 'sync-content.sh');

const result = spawnSync('bash', [scriptPath], { stdio: 'inherit' });

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}
