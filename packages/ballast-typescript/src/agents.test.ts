import path from 'path';
import {
  listAgents,
  getAgentDir,
  isValidAgent,
  resolveAgents,
  AGENT_IDS
} from './agents';

describe('agents', () => {
  describe('listAgents', () => {
    test('returns all agent ids', () => {
      expect(listAgents()).toEqual([...AGENT_IDS]);
      expect(listAgents()).toContain('linting');
      expect(listAgents()).toContain('local-dev');
      expect(listAgents()).toContain('docs');
      expect(listAgents()).toContain('cicd');
      expect(listAgents()).toContain('observability');
      expect(listAgents()).toContain('publishing');
      expect(listAgents()).toContain('testing');
    });
  });

  describe('getAgentDir', () => {
    test('returns path to agent directory', () => {
      const dir = getAgentDir('linting');
      expect(dir).toMatch(/agents[/\\]typescript[/\\]linting$/);
      expect(path.isAbsolute(dir) || dir.includes('agents')).toBe(true);
    });

    test('returns common path for common agents', () => {
      const dir = getAgentDir('local-dev', 'python');
      expect(dir).toMatch(/agents[/\\]common[/\\]local-dev$/);
    });
  });

  describe('isValidAgent', () => {
    test('returns true for known agents', () => {
      expect(isValidAgent('linting')).toBe(true);
      expect(isValidAgent('local-dev')).toBe(true);
      expect(isValidAgent('docs')).toBe(true);
      expect(isValidAgent('cicd')).toBe(true);
      expect(isValidAgent('observability')).toBe(true);
      expect(isValidAgent('publishing')).toBe(true);
      expect(isValidAgent('testing')).toBe(true);
    });

    test('returns false for unknown agents', () => {
      expect(isValidAgent('unknown')).toBe(false);
      expect(isValidAgent('')).toBe(false);
    });
  });

  describe('resolveAgents', () => {
    test('"all" returns all agents', () => {
      expect(resolveAgents('all')).toEqual([...AGENT_IDS]);
    });

    test('single valid agent returns array of one', () => {
      expect(resolveAgents('linting')).toEqual(['linting']);
    });

    test('single invalid agent returns empty array', () => {
      expect(resolveAgents('foo')).toEqual([]);
    });

    test('array with "all" returns all agents', () => {
      expect(resolveAgents(['all'])).toEqual([...AGENT_IDS]);
    });

    test('array of valid agents returns them', () => {
      expect(resolveAgents(['linting', 'cicd'])).toEqual(['linting', 'cicd']);
    });

    test('array filters out invalid agents', () => {
      expect(resolveAgents(['linting', 'foo', 'cicd'])).toEqual([
        'linting',
        'cicd'
      ]);
    });
  });

  describe('language profiles', () => {
    test('python profile uses same public agent ids', () => {
      expect(listAgents('python')).toEqual([...AGENT_IDS]);
      expect(isValidAgent('testing', 'python')).toBe(true);
    });

    test('go profile uses same public agent ids', () => {
      expect(listAgents('go')).toEqual([...AGENT_IDS]);
      expect(isValidAgent('logging', 'go')).toBe(true);
    });

    test('ansible profile uses same public agent ids', () => {
      expect(listAgents('ansible')).toEqual([...AGENT_IDS]);
      expect(isValidAgent('linting', 'ansible')).toBe(true);
    });
  });
});
