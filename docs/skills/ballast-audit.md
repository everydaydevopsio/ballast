# Ballast Audit Skill Guide

This skill helps you audit and optimize the AI rule files in a Ballast-managed repository. It ensures that AI agents have high-quality, low-token guidance.

## When to Use

- When the AI agent feels "slow" or hits context limits.
- When you notice the agent is confused by conflicting or redundant rules.
- After adding a new language profile to ensure it doesn't duplicate `common` rules.

## How to Audit

### 1. Identify Context Bloat
Run the following to see the "token weight" of your current rules:
```bash
du -sh .codex/rules/* .gemini/rules/* | sort -rh
```

### 2. Find Redundant Rules
Check for byte-for-byte duplicates:
```bash
find .codex/rules .gemini/rules -type f -exec md5sum {} + | sort | uniq -w32 -d
```

### 3. Calculate the Ballast Audit Score (BAS)
Evaluate a file's effectiveness:
- **Density**: Does it focus on *this* repo's specific paths and quirks?
- **Uniqueness**: Is this content found nowhere else?
- **Risk**: Does it prevent critical failures?

## Remediation
- **Delete** generic "How to use Git/NVM/PNPM" sections.
- **Merge** duplicate files into a single `common/` rule.
- **Convert** large rules (>10KB) into **Skills**.
