import { buildContent } from './build';

describe('generated rule context hygiene', () => {
  test('local-dev env rule stays under the persistent-context budget', () => {
    const content = buildContent('local-dev', 'codex', 'env');

    expect(Buffer.byteLength(content, 'utf8')).toBeLessThan(6000);
    expect(content).toContain('.nvmrc');
    expect(content).toContain('docker-compose.local.yaml');
    expect(content).toContain('Makefile');
    expect(content).toContain('make up-local');
    expect(content).toContain('docs/agents/local-dev.md');
  });

  test('typescript logging rule stays under the persistent-context budget', () => {
    const content = buildContent('logging', 'codex');

    expect(Buffer.byteLength(content, 'utf8')).toBeLessThan(6000);
    expect(content).toContain('pino-browser');
    expect(content).toContain('/api/logs');
  });

  test('typescript testing rule stays under the persistent-context budget', () => {
    const content = buildContent('testing', 'codex');

    expect(Buffer.byteLength(content, 'utf8')).toBeLessThan(6000);
  });

  test('typescript linting rule stays under the persistent-context budget', () => {
    const content = buildContent('linting', 'codex');

    expect(Buffer.byteLength(content, 'utf8')).toBeLessThan(5000);
  });
});
