# Ballast Audit Skill

This skill performs a "Context Hygiene" audit of the repository's AI rule files (`.codex/rules/`, `.gemini/rules/`, etc.). It identifies redundant, oversized, or low-density rules that degrade AI performance by consuming excessive context tokens.

---

## Capabilities

- **Find Duplicates**: Detect identical rule files across different language/target directories.
- **Calculate BAS**: Apply the Ballast Audit Score to evaluate rule efficiency.
- **Identify Bloat**: Flag files over 5KB for trimming, and escalate files over 10KB to skill-or-split candidates.
- **Recommend Merges**: Suggest consolidation for fragmented rules.

---

## Scoring Logic (BAS)

Evaluate every rule file against these four criteria:

1.  **Density (40%)**: (Project-Specific Lines / Total Lines).
2.  **Risk (30%)**: High (Security, CI, Build), Med (Linting, Tests), Low (Badges, Docs).
3.  **Uniqueness (20%)**: 0% if a byte-for-byte duplicate exists; 100% if unique.
4.  **Actionability (10%)**: Percentage of lines starting with imperatives (Use, Run, Don't, Ensure).

---

## Audit Procedures

### 1. Check for Redundancy (The "Duplication Tax")

Run this to find identical files that should be symlinked or merged:

```bash
find .codex/rules .gemini/rules -type f -exec md5sum {} + | sort | uniq -w32 -d
```

**Action**: If duplicates exist across `common/` and `<lang>/`, remove the language-specific copy and reference the common one.

### 2. Check for "AI Bloat" (The "Size Tax")

Identify files that are too large to be persistent rules:

```bash
find .codex/rules .gemini/rules -name "*.md" -size +5k
find .codex/rules .gemini/rules -name "*.md" -size +10k
```

**Action**:
- **Files > 10KB**: Convert to a **Skill** (loaded on-demand) rather than a **Rule** (loaded every turn).
- **Files > 5KB**: Audit for "Generic Tooling" sections (e.g., "How to install Node.js") and delete them.

### 3. Calculate Signal-to-Noise Ratio

For a specific file, check how many lines are actually project-specific:

```bash
# Count lines mentioning specific repo paths or custom scripts vs total lines
FILE="path/to/rule.md"
SPECIFIC=$(grep -cE "(\./|src/|packages/|ballast)" "$FILE")
TOTAL=$(wc -l < "$FILE" | awk '{print $1}')
echo "Density: $(( (SPECIFIC * 100) / TOTAL ))%"
```

---

## Recommendations for Optimization

1.  **Kill the Duplicates**: Move all shared logic to `.codex/rules/common` (or `.gemini/rules/common`) and ensure the installer doesn't create duplicate content in language subdirectories.
2.  **Rule-to-Skill Promotion**: Any rule file with a large "Example Script" or "Reference Implementation" block should be moved to a `.skill` file.
3.  **Inheritance over Copying**: Use a "Base Rule" for languages and only provide "Delta Rules" for specific overrides.
