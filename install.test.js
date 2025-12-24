const fs = require('fs');
const path = require('path');
const os = require('os');

// Mock modules
jest.mock('fs');
jest.mock('os');

describe('install.js', () => {
  let consoleLogSpy;
  let consoleErrorSpy;
  let processExitSpy;

  beforeEach(() => {
    // Clear all mocks before each test
    jest.clearAllMocks();

    // Spy on console methods
    consoleLogSpy = jest.spyOn(console, 'log').mockImplementation();
    consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();
    processExitSpy = jest.spyOn(process, 'exit').mockImplementation();

    // Default mock implementations
    os.homedir.mockReturnValue('/mock/home');
    fs.existsSync.mockReturnValue(true);
    fs.mkdirSync.mockImplementation();
    fs.copyFileSync.mockImplementation();
  });

  afterEach(() => {
    consoleLogSpy.mockRestore();
    consoleErrorSpy.mockRestore();
    processExitSpy.mockRestore();
  });

  test('successfully installs agent when directory exists', () => {
    // Mock directory already exists
    fs.existsSync.mockReturnValue(true);

    // Run the install script
    require('./install.js');

    // Verify directory creation was not called
    expect(fs.mkdirSync).not.toHaveBeenCalled();

    // Verify file was copied
    expect(fs.copyFileSync).toHaveBeenCalledWith(
      expect.stringContaining('agent/typescript-linting.md'),
      '/mock/home/.config/opencode/agent/typescript-linting.md'
    );

    // Verify success message
    expect(consoleLogSpy).toHaveBeenCalledWith(
      expect.stringContaining(
        '✓ TypeScript Linting agent installed successfully!'
      )
    );
  });

  test('creates directory when it does not exist', () => {
    // Mock directory does not exist
    fs.existsSync.mockReturnValue(false);

    // Run the install script
    require('./install.js');

    // Verify directory creation
    expect(fs.mkdirSync).toHaveBeenCalledWith(
      '/mock/home/.config/opencode/agent',
      { recursive: true }
    );

    // Verify file was copied
    expect(fs.copyFileSync).toHaveBeenCalled();

    // Verify success message
    expect(consoleLogSpy).toHaveBeenCalledWith(
      expect.stringContaining(
        '✓ TypeScript Linting agent installed successfully!'
      )
    );
  });

  test('handles copy error gracefully', () => {
    const errorMessage = 'Permission denied';
    fs.copyFileSync.mockImplementation(() => {
      throw new Error(errorMessage);
    });

    // Run the install script
    require('./install.js');

    // Verify error message
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      'Failed to install TypeScript Linting agent:',
      errorMessage
    );

    // Verify process exit with error code
    expect(processExitSpy).toHaveBeenCalledWith(1);
  });

  test('displays correct usage instructions', () => {
    // Run the install script
    require('./install.js');

    // Verify all usage instructions are displayed
    expect(consoleLogSpy).toHaveBeenCalledWith(
      expect.stringContaining('Usage:')
    );
    expect(consoleLogSpy).toHaveBeenCalledWith(
      expect.stringContaining('Run "opencode" in any TypeScript project')
    );
    expect(consoleLogSpy).toHaveBeenCalledWith(
      expect.stringContaining('Manual invocation:')
    );
    expect(consoleLogSpy).toHaveBeenCalledWith(
      expect.stringContaining(
        '@typescript-linting help me set up linting for this project'
      )
    );
  });

  test('uses correct target path', () => {
    os.homedir.mockReturnValue('/custom/home');

    // Run the install script
    require('./install.js');

    // Verify correct target path
    expect(fs.copyFileSync).toHaveBeenCalledWith(
      expect.any(String),
      '/custom/home/.config/opencode/agent/typescript-linting.md'
    );
  });

  test('source file path is constructed correctly', () => {
    // Run the install script
    require('./install.js');

    // Verify source file includes correct path
    const copyCall = fs.copyFileSync.mock.calls[0][0];
    expect(copyCall).toContain('agent');
    expect(copyCall).toContain('typescript-linting.md');
  });
});
