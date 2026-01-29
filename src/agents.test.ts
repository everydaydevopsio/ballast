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
      expect(listAgents()).toContain('cicd');
      expect(listAgents()).toContain('observability');
    });
  });

  describe('getAgentDir', () => {
    test('returns path to agent directory', () => {
      const dir = getAgentDir('linting');
      expect(dir).toMatch(/agents[/\\]linting$/);
      expect(path.isAbsolute(dir) || dir.includes('agents')).toBe(true);
    });
  });

  describe('isValidAgent', () => {
    test('returns true for known agents', () => {
      expect(isValidAgent('linting')).toBe(true);
      expect(isValidAgent('local-dev')).toBe(true);
      expect(isValidAgent('cicd')).toBe(true);
      expect(isValidAgent('observability')).toBe(true);
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
});
