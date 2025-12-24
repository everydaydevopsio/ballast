# @everydaydevops/opencode-typescript-linting

[![npm version](https://badge.fury.io/js/@everydaydevops%2Fopencode-typescript-linting.svg)](https://www.npmjs.com/package/@everydaydevops/opencode-typescript-linting)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

OpenCode agent for implementing TypeScript linting and code formatting following Everyday DevOps best practices.

## About

This package provides an OpenCode subagent that automates the setup of TypeScript/JavaScript linting and formatting infrastructure in your projects. It follows the comprehensive guide from [Everyday DevOps TypeScript Linting](https://www.markcallen.com/typescript-linting/).

## Prerequisites

This agent requires [OpenCode](https://opencode.ai) to be installed:

```bash
npm install -g opencode-ai
```

## Installation

Install the agent globally or in your project:

```bash
npm install @everydaydevops/opencode-typescript-linting
```

The postinstall script will automatically copy the agent configuration to `~/.config/opencode/agent/typescript-linting.md`.

## Usage

Once installed, the `typescript-linting` subagent is available in any OpenCode session.

### Automatic Invocation

Primary OpenCode agents can automatically invoke this subagent for linting setup tasks.

### Manual Invocation

You can directly invoke the agent using:

```bash
opencode
```

Then in the OpenCode session:

```
@typescript-linting help me set up linting for this project
```

## What It Does

The agent will:

- Install and configure ESLint with TypeScript support
- Set up Prettier for code formatting
- Configure Husky for Git hooks
- Set up lint-staged for pre-commit checks
- Create GitHub Actions workflow for CI linting
- Add helpful npm scripts for linting and formatting

## Learn More

For detailed information about the linting setup and best practices, visit:

- [TypeScript Linting Guide](https://www.markcallen.com/typescript-linting/)
- [OpenCode Documentation](https://opencode.ai/docs)

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Mark C Allen ([@markcallen](https://github.com/markcallen))
