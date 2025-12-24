const globals = require('globals');
const pluginJs = require('@eslint/js');
const tseslint = require('typescript-eslint');
const eslintPluginPrettierRecommended = require('eslint-plugin-prettier/recommended');

module.exports = [
  { files: ['**/*.{js,mjs,cjs,ts}'] },
  { languageOptions: { globals: globals.node } },
  pluginJs.configs.recommended,
  ...tseslint.configs.recommended,
  eslintPluginPrettierRecommended,
  {
    files: ['**/*.test.js', '**/*.spec.js'],
    languageOptions: {
      globals: globals.jest
    },
    rules: {
      '@typescript-eslint/no-unused-vars': 'off'
    }
  },
  {
    rules: {
      'no-console': 'warn',
      '@typescript-eslint/no-require-imports': 'off'
    }
  },
  {
    files: ['install.js'],
    rules: {
      'no-console': 'off'
    }
  },
  {
    ignores: ['node_modules', 'dist', 'coverage']
  }
];
