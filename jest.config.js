module.exports = {
  testEnvironment: 'node',
  collectCoverageFrom: ['install.js'],
  coverageDirectory: 'coverage',
  testMatch: ['**/*.test.js'],
  verbose: true,
  coverageThreshold: {
    global: {
      branches: 80,
      functions: 80,
      lines: 80,
      statements: 80
    }
  }
};
