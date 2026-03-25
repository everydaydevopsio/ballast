#!/usr/bin/env node

const { main } = require('../dist/cli');

try {
  const result = main();
  if (result && typeof result.catch === 'function') {
    result.catch((err) => {
      console.error(err);
      process.exit(1);
    });
  }
} catch (err) {
  console.error(err);
  process.exit(1);
}
