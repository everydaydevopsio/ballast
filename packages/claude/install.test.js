describe('claude install.js', () => {
  let fs;
  let os;
  let core;
  let consoleLogSpy;
  let consoleErrorSpy;
  let processExitSpy;

  beforeEach(() => {
    // Reset module cache first
    jest.resetModules();

    // Clear environment variables
    delete process.env.npm_config_global;
    delete process.env.npm_config_local_prefix;

    // Set up mocks
    jest.doMock('fs', () => ({
      existsSync: jest.fn().mockReturnValue(false),
      mkdirSync: jest.fn(),
      writeFileSync: jest.fn()
    }));

    jest.doMock('os', () => ({
      homedir: jest.fn().mockReturnValue('/mock/home')
    }));

    jest.doMock('@everydaydevops/typescript-linting-core', () => ({
      buildClaudeFormat: jest.fn().mockReturnValue('claude content')
    }));

    // Get mocked modules
    fs = require('fs');
    os = require('os');
    core = require('@everydaydevops/typescript-linting-core');

    // Set up console and process spies
    consoleLogSpy = jest.spyOn(console, 'log').mockImplementation();
    consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();
    processExitSpy = jest.spyOn(process, 'exit').mockImplementation();
  });

  afterEach(() => {
    consoleLogSpy.mockRestore();
    consoleErrorSpy.mockRestore();
    processExitSpy.mockRestore();
    jest.resetModules();
  });

  describe('local installation', () => {
    test('installs to .claude/rules directory', () => {
      process.env.npm_config_local_prefix = '/mock/project';

      const { install } = require('./install.js');
      install();

      expect(fs.mkdirSync).toHaveBeenCalledWith('/mock/project/.claude/rules', {
        recursive: true
      });
      expect(fs.writeFileSync).toHaveBeenCalledWith(
        '/mock/project/.claude/rules/typescript-linting.md',
        'claude content'
      );
    });

    test('shows success message', () => {
      process.env.npm_config_local_prefix = '/mock/project';

      const { install } = require('./install.js');
      install();

      expect(consoleLogSpy).toHaveBeenCalledWith(
        'Claude Code TypeScript Linting rules installed successfully!'
      );
    });
  });

  describe('global installation', () => {
    test('installs to ~/.claude/rules', () => {
      process.env.npm_config_global = 'true';

      const { install } = require('./install.js');
      install();

      expect(fs.mkdirSync).toHaveBeenCalledWith('/mock/home/.claude/rules', {
        recursive: true
      });
      expect(fs.writeFileSync).toHaveBeenCalledWith(
        '/mock/home/.claude/rules/typescript-linting.md',
        'claude content'
      );
    });
  });

  describe('overwrite protection', () => {
    test('does not overwrite existing file', () => {
      process.env.npm_config_local_prefix = '/mock/project';
      fs.existsSync.mockReturnValue(true);

      const { install } = require('./install.js');
      install();

      expect(fs.writeFileSync).not.toHaveBeenCalled();
      expect(consoleLogSpy).toHaveBeenCalledWith(
        'Claude Code TypeScript Linting rules: Skipped'
      );
      expect(processExitSpy).toHaveBeenCalledWith(0);
    });
  });

  describe('error handling', () => {
    test('exits with code 1 on error', () => {
      process.env.npm_config_local_prefix = '/mock/project';
      fs.writeFileSync.mockImplementation(() => {
        throw new Error('Permission denied');
      });

      const { install } = require('./install.js');
      install();

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        'Failed to install Claude Code TypeScript Linting rules:',
        'Permission denied'
      );
      expect(processExitSpy).toHaveBeenCalledWith(1);
    });
  });
});
