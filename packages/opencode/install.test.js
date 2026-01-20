describe('opencode install.js', () => {
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
      buildOpenCodeFormat: jest.fn().mockReturnValue('opencode content')
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
    test('installs to .opencode directory', () => {
      process.env.npm_config_local_prefix = '/mock/project';

      const { install } = require('./install.js');
      install();

      expect(fs.mkdirSync).toHaveBeenCalledWith('/mock/project/.opencode', {
        recursive: true
      });
      expect(fs.writeFileSync).toHaveBeenCalledWith(
        '/mock/project/.opencode/typescript-linting.md',
        'opencode content'
      );
    });

    test('shows success message', () => {
      process.env.npm_config_local_prefix = '/mock/project';

      const { install } = require('./install.js');
      install();

      expect(consoleLogSpy).toHaveBeenCalledWith(
        'OpenCode TypeScript Linting agent installed successfully!'
      );
      expect(consoleLogSpy).toHaveBeenCalledWith('  Installation type: local');
    });
  });

  describe('global installation', () => {
    test('installs to ~/.config/opencode/agent', () => {
      process.env.npm_config_global = 'true';

      const { install } = require('./install.js');
      install();

      expect(fs.mkdirSync).toHaveBeenCalledWith(
        '/mock/home/.config/opencode/agent',
        { recursive: true }
      );
      expect(fs.writeFileSync).toHaveBeenCalledWith(
        '/mock/home/.config/opencode/agent/typescript-linting.md',
        'opencode content'
      );
    });

    test('shows global installation type', () => {
      process.env.npm_config_global = 'true';

      const { install } = require('./install.js');
      install();

      expect(consoleLogSpy).toHaveBeenCalledWith('  Installation type: global');
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
        'Failed to install OpenCode TypeScript Linting agent:',
        'Permission denied'
      );
      expect(processExitSpy).toHaveBeenCalledWith(1);
    });
  });

  describe('directory handling', () => {
    test('skips directory creation if exists', () => {
      process.env.npm_config_local_prefix = '/mock/project';
      fs.existsSync.mockReturnValue(true);

      const { install } = require('./install.js');
      install();

      expect(fs.mkdirSync).not.toHaveBeenCalled();
    });
  });
});
