from __future__ import annotations

import argparse
import json
import os
import re
from dataclasses import dataclass, field
from functools import lru_cache
from importlib import metadata
from pathlib import Path

TARGETS = ["cursor", "claude", "opencode", "codex"]
LANGUAGES = ["typescript", "python", "go"]
COMMON_AGENTS = ["local-dev", "cicd", "observability"]
LANGUAGE_AGENTS = ["linting", "logging", "testing"]
AGENTS_BY_LANGUAGE = {
    "typescript": COMMON_AGENTS + LANGUAGE_AGENTS,
    "python": COMMON_AGENTS + LANGUAGE_AGENTS,
    "go": COMMON_AGENTS + LANGUAGE_AGENTS,
}


def cli_version() -> str:
    return ballast_version()


@dataclass
class InstallResult:
    installed: list[str] = field(default_factory=list)
    installed_rules: list[tuple[str, str]] = field(default_factory=list)
    installed_support_files: list[str] = field(default_factory=list)
    skipped: list[str] = field(default_factory=list)
    skipped_support_files: list[str] = field(default_factory=list)
    errors: list[tuple[str, str]] = field(default_factory=list)


def package_root() -> Path:
    repo_root = os.environ.get("BALLAST_REPO_ROOT")
    if repo_root:
        root = Path(repo_root).resolve()
        if (root / "agents").exists():
            return root
        raise FileNotFoundError(f"BALLAST_REPO_ROOT does not contain agents/: {root}")
    return Path(__file__).resolve().parent


@lru_cache(maxsize=1)
def resolve_agents_root() -> Path:
    root = package_root()
    packaged = root / "agents"
    if packaged.exists():
        return packaged

    legacy_repo_agents = Path(__file__).resolve().parents[3] / "agents"
    if legacy_repo_agents.exists():
        return legacy_repo_agents

    raise FileNotFoundError(
        "Unable to locate Ballast agents content. Reinstall ballast-python or set BALLAST_REPO_ROOT to a ballast repo checkout."
    )


def rulesrc_filename(language: str) -> str:
    return ".rulesrc.json"


def legacy_rulesrc_filename(language: str) -> str:
    if language == "typescript":
        return ".rulesrc.ts.json"
    return f".rulesrc.{language}.json"


@lru_cache(maxsize=1)
def ballast_version() -> str:
    try:
        return metadata.version("ballast-python")
    except metadata.PackageNotFoundError:
        pyproject = Path(__file__).resolve().parents[1] / "pyproject.toml"
        if pyproject.exists():
            match = re.search(r'(?m)^version = "([^"]+)"$', pyproject.read_text(encoding="utf-8"))
            if match:
                return match.group(1)
    return "dev"


def agent_dir(agent: str, language: str) -> Path:
    if agent in COMMON_AGENTS:
        return resolve_agents_root() / "common" / agent
    return resolve_agents_root() / language / agent


def resolve_project_root(cwd: Path) -> Path:
    for directory in [cwd, *cwd.parents]:
        has_pkg = (directory / "package.json").exists()
        has_go = (directory / "go.mod").exists()
        has_pyproject = (directory / "pyproject.toml").exists()
        has_any_cfg = (directory / ".rulesrc.json").exists() or (directory / ".rulesrc.ts.json").exists() or any(
            (directory / legacy_rulesrc_filename(lang)).exists() for lang in LANGUAGES
        )
        if has_pkg or has_go or has_pyproject or has_any_cfg:
            return directory
    return cwd


def is_ci_mode() -> bool:
    return (
        os.environ.get("CI") in {"true", "1"}
        or os.environ.get("TF_BUILD") == "true"
        or os.environ.get("GITHUB_ACTIONS") == "true"
        or os.environ.get("GITLAB_CI") == "true"
    )


def load_config(root: Path, language: str) -> dict[str, object] | None:
    file_path = root / rulesrc_filename(language)
    if not file_path.exists():
        file_path = root / legacy_rulesrc_filename(language)
    if not file_path.exists():
        return None
    try:
        data = json.loads(file_path.read_text(encoding="utf-8"))
        if not isinstance(data, dict):
            return None
        target = data.get("target")
        agents = data.get("agents")
        if not isinstance(target, str) or not isinstance(agents, list):
            return None
        if not all(isinstance(item, str) for item in agents):
            return None
        return {"target": target, "agents": agents}
    except Exception:
        return None


def save_config(root: Path, language: str, target: str, agents: list[str]) -> None:
    (root / rulesrc_filename(language)).write_text(
        json.dumps({"target": target, "agents": agents}, indent=2), encoding="utf-8"
    )


def parse_agent_tokens(raw: str | None, all_agents: bool, language: str) -> list[str]:
    if all_agents:
        return list(AGENTS_BY_LANGUAGE[language])
    if not raw:
        return []
    values = [item.strip() for item in raw.split(",") if item.strip()]
    if "all" in values:
        return list(AGENTS_BY_LANGUAGE[language])
    return values


def is_valid_agent(agent: str, language: str) -> bool:
    return agent in AGENTS_BY_LANGUAGE[language]


def list_rule_suffixes(agent: str, language: str) -> list[str]:
    directory = agent_dir(agent, language)
    suffixes: list[str] = []
    if (directory / "content.md").exists():
        suffixes.append("")
    for entry in sorted(directory.glob("content-*.md")):
        suffix = entry.stem.removeprefix("content-")
        if suffix:
            suffixes.append(suffix)
    if not suffixes:
        raise FileNotFoundError(f"Agent {agent} has no content files")
    return suffixes


def read_content(agent: str, language: str, suffix: str = "") -> str:
    filename = "content.md" if suffix == "" else f"content-{suffix}.md"
    return (agent_dir(agent, language) / filename).read_text(encoding="utf-8")


def read_template(agent: str, language: str, filename: str, suffix: str = "") -> str:
    base, ext = filename.rsplit(".", 1)
    if suffix:
        candidate = agent_dir(agent, language) / "templates" / f"{base}-{suffix}.{ext}"
        if candidate.exists():
            return candidate.read_text(encoding="utf-8")
    return (agent_dir(agent, language) / "templates" / filename).read_text(encoding="utf-8")


def build_content(agent: str, target: str, language: str, suffix: str = "") -> str:
    body = read_content(agent, language, suffix)
    if target == "cursor":
        return read_template(agent, language, "cursor-frontmatter.yaml", suffix) + "\n" + body
    if target == "claude":
        return read_template(agent, language, "claude-header.md", suffix) + body
    if target == "opencode":
        return read_template(agent, language, "opencode-frontmatter.yaml", suffix) + "\n" + body
    try:
        header = read_template(agent, language, "codex-header.md", suffix)
    except FileNotFoundError:
        header = read_template(agent, language, "claude-header.md", suffix)
    return header + body


def destination(root: Path, target: str, basename: str) -> Path:
    rule_subdir = os.environ.get("BALLAST_RULE_SUBDIR", "").strip()
    if rule_subdir and not re.fullmatch(r"[A-Za-z0-9_-]+", rule_subdir):
        raise ValueError(
            f"Invalid BALLAST_RULE_SUBDIR value {rule_subdir!r}. Only letters, digits, '-' and '_' are allowed."
        )
    scoped_basename = (
        basename
        if not rule_subdir or rule_subdir == "common" or basename.startswith(f"{rule_subdir}-")
        else f"{rule_subdir}-{basename}"
    )
    if target == "cursor":
        base = root / ".cursor" / "rules"
        return (base / rule_subdir / f"{scoped_basename}.mdc") if rule_subdir else (base / f"{scoped_basename}.mdc")
    if target == "claude":
        base = root / ".claude" / "rules"
        return (base / rule_subdir / f"{scoped_basename}.md") if rule_subdir else (base / f"{scoped_basename}.md")
    if target == "opencode":
        base = root / ".opencode"
        return (base / rule_subdir / f"{scoped_basename}.md") if rule_subdir else (base / f"{scoped_basename}.md")
    base = root / ".codex" / "rules"
    return (base / rule_subdir / f"{scoped_basename}.md") if rule_subdir else (base / f"{scoped_basename}.md")


def extract_description_from_frontmatter(frontmatter: str) -> str | None:
    match = re.search(r"(?m)^description:\s*['\"]?(.+?)['\"]?\s*$", frontmatter)
    if match:
        value = match.group(1).strip()
        return value or None
    return None


def get_codex_rule_description(agent: str, language: str, suffix: str = "") -> str | None:
    try:
        frontmatter = read_template(agent, language, "cursor-frontmatter.yaml", suffix)
        return extract_description_from_frontmatter(frontmatter)
    except FileNotFoundError:
        return None


def rule_basename(agent: str, language: str, suffix: str = "") -> str:
    basename = agent if suffix == "" else f"{agent}-{suffix}"
    if agent in COMMON_AGENTS:
        return basename
    return f"{language}-{basename}"


def build_codex_agents_md(agents: list[str], language: str) -> str:
    lines = [
        "# AGENTS.md",
        "",
        "This file provides guidance to Codex (CLI and app) for working in this repository.",
        "",
        "## Installed agent rules",
        "",
        f"Created by Ballast v{ballast_version()}. Do not edit this section.",
        "",
        "Read and follow these rule files in `.codex/rules/` when they apply:",
        "",
    ]
    for agent in agents:
        for suffix in list_rule_suffixes(agent, language):
            basename = rule_basename(agent, language, suffix)
            description = get_codex_rule_description(agent, language, suffix) or f"Rules for {basename}"
            lines.append(f"- `.codex/rules/{basename}.md` — {description}")
    lines.append("")
    return "\n".join(lines)


def build_claude_md(agents: list[str], language: str) -> str:
    lines = [
        "# CLAUDE.md",
        "",
        "This file provides guidance to Claude Code for working in this repository.",
        "",
        "## Installed agent rules",
        "",
        f"Created by Ballast v{ballast_version()}. Do not edit this section.",
        "",
        "Read and follow these rule files in `.claude/rules/` when they apply:",
        "",
    ]
    for agent in agents:
        for suffix in list_rule_suffixes(agent, language):
            basename = rule_basename(agent, language, suffix)
            description = get_codex_rule_description(agent, language, suffix) or f"Rules for {basename}"
            lines.append(f"- `.claude/rules/{basename}.md` — {description}")
    lines.append("")
    return "\n".join(lines)


def normalize_line_endings(content: str) -> str:
    return content.replace("\r\n", "\n").replace("\r", "\n")


def split_frontmatter_document(content: str) -> tuple[str | None, str]:
    normalized = normalize_line_endings(content)
    match = re.match(r"^\s*---\n([\s\S]*?)\n---\n?", normalized)
    if not match:
        return None, normalized.lstrip()
    return match.group(0).rstrip(), normalized[match.end() :].lstrip()


def extract_frontmatter_yaml(frontmatter: str) -> str | None:
    match = re.match(r"^---\n([\s\S]*?)\n---$", frontmatter)
    if not match:
        return None
    return match.group(1)


def parse_top_level_yaml_blocks(yaml_content: str) -> tuple[str, list[tuple[str, str]]]:
    lines = normalize_line_endings(yaml_content).split("\n")
    blocks: list[tuple[str, str]] = []
    preamble_lines: list[str] = []
    current_key: str | None = None
    current_lines: list[str] = []

    def flush_current() -> None:
        nonlocal current_key, current_lines
        if current_key is not None:
            blocks.append((current_key, "\n".join(current_lines).rstrip()))
        current_key = None
        current_lines = []

    for line in lines:
        key_match = re.match(r"^([A-Za-z0-9_-]+):(.*)$", line)
        if key_match:
            flush_current()
            current_key = key_match.group(1)
            current_lines = [line]
            continue
        if current_key is None:
            preamble_lines.append(line)
        else:
            current_lines.append(line)

    flush_current()
    return "\n".join(preamble_lines).strip(), blocks


def split_nested_yaml_block(block: str) -> tuple[str, str, int] | None:
    lines = block.split("\n")
    if len(lines) < 2:
        return None

    body_lines = lines[1:]
    non_empty = [line for line in body_lines if line.strip()]
    if not non_empty:
        return None

    indent = min(len(line) - len(line.lstrip(" ")) for line in non_empty)
    if indent == 0:
        return None

    dedented_lines = []
    for line in body_lines:
        if not line.strip():
            dedented_lines.append("")
            continue
        dedented_lines.append(line[indent:])

    if any(line.startswith("- ") for line in dedented_lines if line):
        return None

    return lines[0], "\n".join(dedented_lines).rstrip(), indent


def merge_yaml_blocks(existing_block: str, canonical_block: str) -> str:
    existing_nested = split_nested_yaml_block(existing_block)
    canonical_nested = split_nested_yaml_block(canonical_block)
    if existing_nested is None or canonical_nested is None:
        return existing_block

    merged_body = merge_yaml_mapping_content(existing_nested[1], canonical_nested[1])
    if merged_body is None:
        return existing_block

    header = existing_nested[0]
    indent = canonical_nested[2]
    lines = [header]
    for line in merged_body.split("\n"):
        lines.append((" " * indent + line) if line else "")
    return "\n".join(lines).rstrip()


def merge_yaml_mapping_content(existing_yaml: str, canonical_yaml: str) -> str | None:
    existing_preamble, existing_blocks = parse_top_level_yaml_blocks(existing_yaml)
    canonical_preamble, canonical_blocks = parse_top_level_yaml_blocks(canonical_yaml)
    if not canonical_blocks:
        return None

    existing_by_key = {key: block for key, block in existing_blocks}
    canonical_keys = {key for key, _ in canonical_blocks}
    merged_blocks: list[str] = []

    for key, block in canonical_blocks:
        if key in existing_by_key:
            merged_blocks.append(merge_yaml_blocks(existing_by_key[key], block))
        else:
            merged_blocks.append(block)

    for key, block in existing_blocks:
        if key not in canonical_keys:
            merged_blocks.append(block)

    preamble = canonical_preamble or existing_preamble
    parts = [part for part in [preamble, *merged_blocks] if part]
    return "\n".join(parts).rstrip()


def merge_frontmatter(existing_frontmatter: str | None, canonical_frontmatter: str | None) -> str | None:
    if not canonical_frontmatter:
        return existing_frontmatter
    if not existing_frontmatter:
        return canonical_frontmatter

    existing_yaml = extract_frontmatter_yaml(existing_frontmatter)
    canonical_yaml = extract_frontmatter_yaml(canonical_frontmatter)
    if existing_yaml is None or canonical_yaml is None:
        return existing_frontmatter

    merged_yaml = merge_yaml_mapping_content(existing_yaml, canonical_yaml)
    if merged_yaml is None:
        return existing_frontmatter
    return f"---\n{merged_yaml}\n---"


def parse_markdown_body(content: str) -> tuple[str, list[tuple[str, str]]]:
    normalized = normalize_line_endings(content)
    matches = list(re.finditer(r"^## .*(?:\n|$)", normalized, re.MULTILINE))
    if not matches:
        return normalized.strip(), []

    intro = normalized[: matches[0].start()].strip()
    sections: list[tuple[str, str]] = []
    for index, match in enumerate(matches):
        end = matches[index + 1].start() if index + 1 < len(matches) else len(normalized)
        section_text = normalized[match.start() : end].strip()
        heading = section_text.splitlines()[0]
        sections.append((heading, section_text))
    return intro, sections


def merge_markdown_bodies(existing: str, canonical: str) -> str:
    if not existing.strip():
        return canonical

    existing_intro, existing_sections = parse_markdown_body(existing)
    canonical_intro, canonical_sections = parse_markdown_body(canonical)
    existing_by_heading = {heading: text for heading, text in existing_sections}
    canonical_headings = {heading for heading, _ in canonical_sections}

    parts: list[str] = []
    intro = existing_intro or canonical_intro
    if intro:
        parts.append(intro)

    for heading, text in canonical_sections:
        parts.append(existing_by_heading.get(heading, text))

    for heading, text in existing_sections:
        if heading not in canonical_headings:
            parts.append(text)

    return "\n\n".join(parts).rstrip() + "\n"


def patch_rule_content(existing: str, canonical: str, target: str) -> str:
    if not existing.strip():
        return canonical

    if target in {"cursor", "opencode"}:
        existing_frontmatter, existing_body = split_frontmatter_document(existing)
        canonical_frontmatter, canonical_body = split_frontmatter_document(canonical)
        frontmatter = merge_frontmatter(existing_frontmatter, canonical_frontmatter)
        body = merge_markdown_bodies(existing_body, canonical_body)
        if frontmatter:
            return f"{frontmatter}\n\n{body}"
        return body

    return merge_markdown_bodies(existing, canonical)


def find_markdown_section_range(content: str, heading: str) -> tuple[int, int] | None:
    normalized = normalize_line_endings(content)
    lines = normalized.split("\n")
    target = f"## {heading}"
    in_fence = False
    offset = 0

    for i, line in enumerate(lines):
        if line.startswith("```"):
            in_fence = not in_fence
        if not in_fence and line == target:
            start = offset
            offset += len(line) + 1
            for next_line in lines[i + 1 :]:
                if next_line.startswith("```"):
                    in_fence = not in_fence
                if not in_fence and next_line.startswith("## "):
                    return start, offset - 1
                offset += len(next_line) + 1
            return start, len(normalized)
        offset += len(line) + 1
    return None


def patch_codex_agents_md(existing: str, canonical: str) -> str:
    if not existing.strip():
        return canonical

    canonical_range = find_markdown_section_range(canonical, "Installed agent rules")
    if not canonical_range:
        return existing
    canonical_section = canonical[canonical_range[0] : canonical_range[1]].rstrip()

    existing_range = find_markdown_section_range(existing, "Installed agent rules")
    if not existing_range:
        return existing.rstrip() + "\n\n" + canonical_section + "\n"

    return (
        existing[: existing_range[0]].rstrip()
        + "\n\n"
        + canonical_section
        + "\n\n"
        + existing[existing_range[1] :].lstrip()
    ).rstrip() + "\n"


def prompt(question: str) -> str:
    return input(question).strip()


def prompt_yes_no(question: str, default: bool = False) -> bool:
    suffix = " [Y/n]: " if default else " [y/N]: "
    value = prompt(question + suffix).lower()
    if not value:
        return default
    return value in {"y", "yes"}


def prompt_target() -> str:
    value = prompt(f"AI platform ({', '.join(TARGETS)}): ").lower()
    if value in TARGETS:
        return value
    print(f"Invalid target. Choose one of: {', '.join(TARGETS)}")
    return prompt_target()


def prompt_agents(language: str) -> list[str]:
    agent_ids = AGENTS_BY_LANGUAGE[language]
    value = prompt(f"Agents (comma-separated or \"all\") [{', '.join(agent_ids)}]: ")
    if not value:
        return list(agent_ids)
    tokens = parse_agent_tokens(value, False, language)
    resolved = [t for t in tokens if is_valid_agent(t, language)]
    if resolved:
        return resolved
    print(f"Invalid agents. Use 'all' or comma-separated: {', '.join(agent_ids)}")
    return prompt_agents(language)


def resolve_target_and_agents(args: argparse.Namespace, root: Path, language: str) -> tuple[str, list[str]] | None:
    cfg = load_config(root, language)
    ci = is_ci_mode() or bool(args.yes)

    target_from_flag = (args.target or "").strip().lower() if args.target else None
    agents_from_flag = parse_agent_tokens(args.agent, bool(args.all), language) if (args.agent or args.all) else None

    if cfg and not target_from_flag and agents_from_flag is None:
        return str(cfg["target"]), list(cfg["agents"])

    target = target_from_flag or (str(cfg["target"]) if cfg else None)
    agents = agents_from_flag if agents_from_flag is not None else (list(cfg["agents"]) if cfg else None)

    if target and agents and len(agents) > 0:
        return target, agents

    if ci:
        return None

    resolved_target = target if target else prompt_target()
    resolved_agents = agents if agents and len(agents) > 0 else prompt_agents(language)
    return resolved_target, resolved_agents


def install(
    root: Path,
    target: str,
    agents: list[str],
    language: str,
    force: bool,
    patch: bool,
    persist: bool,
    patch_claude_md: bool = False,
) -> InstallResult:
    result = InstallResult()
    processed_agents: list[str] = []
    disable_support_files = os.environ.get("BALLAST_DISABLE_SUPPORT_FILES") == "1"

    if persist:
        save_config(root, language, target, agents)

    for agent in agents:
        if not is_valid_agent(agent, language):
            result.errors.append((agent, "Unknown agent"))
            continue

        agent_installed = False
        agent_skipped = False
        agent_processed = False

        try:
            for suffix in list_rule_suffixes(agent, language):
                basename = rule_basename(agent, language, suffix)
                dst = destination(root, target, basename)
                content = build_content(agent, target, language, suffix)
                if dst.exists() and not force and not patch:
                    agent_skipped = True
                    agent_processed = True
                    continue
                dst.parent.mkdir(parents=True, exist_ok=True)
                next_content = (
                    patch_rule_content(dst.read_text(encoding="utf-8"), content, target)
                    if dst.exists() and not force and patch
                    else content
                )
                dst.write_text(next_content, encoding="utf-8")
                result.installed_rules.append((agent, suffix))
                agent_installed = True
                agent_processed = True
            if agent_processed:
                processed_agents.append(agent)
            if agent_installed:
                result.installed.append(agent)
            if agent_skipped and not agent_installed:
                result.skipped.append(agent)
        except Exception as err:
            result.errors.append((agent, str(err)))

    if target == "codex" and not disable_support_files:
        agents_md = root / "AGENTS.md"
        if agents_md.exists() and not force and not patch:
            result.skipped_support_files.append(str(agents_md))
        else:
            try:
                content = build_codex_agents_md(processed_agents, language)
                next_content = (
                    patch_codex_agents_md(agents_md.read_text(encoding="utf-8"), content)
                    if agents_md.exists() and not force and patch
                    else content
                )
                agents_md.write_text(next_content, encoding="utf-8")
                result.installed_support_files.append(str(agents_md))
            except Exception as err:
                result.errors.append(("codex", str(err)))

    if target == "claude" and not disable_support_files:
        claude_md = root / "CLAUDE.md"
        should_patch_claude_md = patch or patch_claude_md
        if claude_md.exists() and not force and not should_patch_claude_md:
            result.skipped_support_files.append(str(claude_md))
        else:
            try:
                content = build_claude_md(processed_agents, language)
                next_content = (
                    patch_codex_agents_md(claude_md.read_text(encoding="utf-8"), content)
                    if claude_md.exists() and not force and should_patch_claude_md
                    else content
                )
                claude_md.write_text(next_content, encoding="utf-8")
                result.installed_support_files.append(str(claude_md))
            except Exception as err:
                result.errors.append(("claude", str(err)))

    return result


def run_install(args: argparse.Namespace) -> int:
    language = (args.language or "python").strip().lower()
    if language not in LANGUAGES:
        print(f"Invalid --language. Use: {', '.join(LANGUAGES)}")
        return 1

    root = resolve_project_root(Path.cwd())
    resolved = resolve_target_and_agents(args, root, language)
    if not resolved:
        print(
            "In CI/non-interactive mode (--yes or CI env), --target and --agent (or --all) are required when config is missing."
        )
        print("Example: ballast-python install --yes --target cursor --agent linting")
        return 1

    target, agents = resolved
    if target not in TARGETS:
        print("Invalid --target. Use: cursor, claude, opencode, codex")
        return 1

    patch_claude_md = False
    if target == "claude" and (root / "CLAUDE.md").exists() and not bool(args.force):
        if bool(args.patch):
            patch_claude_md = True
        elif not is_ci_mode() and not bool(args.yes):
            patch_claude_md = prompt_yes_no(
                f"Existing CLAUDE.md found at {root / 'CLAUDE.md'}. Patch the Installed agent rules section?"
            )
    result = install(root, target, agents, language, bool(args.force), bool(args.patch), True, patch_claude_md)

    if result.errors:
        for agent, error in result.errors:
            print(f"Error installing {agent}: {error}")
        return 1

    if result.installed_rules:
        print(f"Installed for {target}: {', '.join(result.installed)}")
        for agent, suffix in result.installed_rules:
            basename = rule_basename(agent, language, suffix)
            print(f"  {basename} -> {destination(root, target, basename)}")
    if result.installed_support_files:
        for file in result.installed_support_files:
            label = "CLAUDE.md" if file.endswith("CLAUDE.md") else "AGENTS.md"
            print(f"  {label} -> {file}")
    if result.skipped:
        print("Skipped (already present; use --force to overwrite): " + ", ".join(result.skipped))
    if result.skipped_support_files:
        print(
            "Skipped support files (already present; use --force to overwrite): "
            + ", ".join(result.skipped_support_files)
        )
    if not result.installed and not result.skipped and not result.errors:
        print("Nothing to install.")

    return 0


def parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="ballast-python", description="Install Ballast rules for Python projects")
    p.add_argument("--version", action="version", version=cli_version())
    sub = p.add_subparsers(dest="command")
    install_cmd = sub.add_parser("install", help="Install rule files")
    install_cmd.add_argument("--target", "-t")
    install_cmd.add_argument("--language", "-l", default="python")
    install_cmd.add_argument("--agent", "-a")
    install_cmd.add_argument("--all", action="store_true")
    install_cmd.add_argument("--force", "-f", action="store_true")
    install_cmd.add_argument("--patch", "-p", action="store_true")
    install_cmd.add_argument("--yes", "-y", action="store_true")
    return p


def main(argv: list[str] | None = None) -> int:
    p = parser()
    args = p.parse_args(argv)
    command = args.command or "install"
    if command != "install":
        print(f"Unknown command: {command}")
        return 1
    return run_install(args)
