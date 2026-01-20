module.exports = {
  testEnvironment: 'node',
  collectCoverageFrom: [
    'packages/core/index.js',
    'packages/opencode/install.js',
    'packages/claude/install.js',
    'packages/cursor/install.js'
  ],
  coverageDirectory: 'coverage',
  testMatch: ['**/packages/**/*.test.js'],
  verbose: true,
  coverageThreshold: {
    global: {
      branches: 80,
      functions: 80,
      lines: 80,
      statements: 80
    }
  },
  moduleNameMapper: {
    '^@everydaydevops/typescript-linting-core$':
      '<rootDir>/packages/core/index.js'
  }
};
