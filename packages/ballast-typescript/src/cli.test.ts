import { parseArgs } from './cli';

describe('parseArgs', () => {
  test('returns help for top-level help flag', () => {
    expect(parseArgs(['node', 'ballast-typescript', '--help'])).toEqual({
      help: true
    });
  });

  test('returns version for top-level version flag', () => {
    expect(parseArgs(['node', 'ballast-typescript', '--version'])).toEqual({
      version: true
    });
  });

  test('returns doctor for doctor command', () => {
    expect(parseArgs(['node', 'ballast-typescript', 'doctor'])).toEqual({
      doctor: true
    });
  });
});
