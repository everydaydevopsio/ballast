/** @type {import('jest').Config} */
module.exports = {
  testEnvironment: 'node',
  preset: 'ts-jest',
  testEnvironmentOptions: {},
  transform: {
    '^.+\\.tsx?$': [
      'ts-jest',
      {
        tsconfig: 'tsconfig.jest.json'
      }
    ]
  },
  collectCoverageFrom: [
    'src/**/*.ts',
    '!src/**/*.test.ts',
    '!src/cli.ts',
    '!**/node_modules/**'
  ],
  coverageDirectory: 'coverage',
  testMatch: ['**/src/**/*.test.ts', '**/*.test.ts'],
  verbose: true,
  coverageThreshold: {
    global: {
      branches: 66,
      functions: 75,
      lines: 80,
      statements: 80
    }
  }
};
