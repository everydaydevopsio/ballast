from __future__ import annotations

import argparse
import json
import os
import re
from functools import lru_cache
from dataclasses import dataclass, field
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
BALLAST_VERSION = "4.1.7"


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
    if language == "typescript":
        return ".rulesrc.ts.json"
    return f".rulesrc.{language}.json"


def agent_dir(agent: str, language: str) -> Path:
    if agent in COMMON_AGENTS:
        return resolve_agents_root() / "common" / agent
    return resolve_agents_root() / language / agent


def resolve_project_root(cwd: Path) -> Path:
    for directory in [cwd, *cwd.parents]:
        has_pkg = (directory / "package.json").exists()
        has_go = (directory / "go.mod").exists()
        has_pyproject = (directory / "pyproject.toml").exists()
        has_any_cfg = (directory / ".rulesrc.ts.json").exists() or (directory / ".rulesrc.json").exists() or any(
            (directory / rulesrc_filename(lang)).exists() for lang in LANGUAGES
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
    if not file_path.exists() and language == "typescript":
        file_path = root / ".rulesrc.json"
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
    if target == "cursor":
        return root / ".cursor" / "rules" / f"{basename}.mdc"
    if target == "claude":
        return root / ".claude" / "rules" / f"{basename}.md"
    if target == "opencode":
        return root / ".opencode" / f"{basename}.md"
    return root / ".codex" / "rules" / f"{basename}.md"


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


def build_codex_agents_md(agents: list[str], language: str) -> str:
    lines = [
        "# AGENTS.md",
        "",
        "This file provides guidance to Codex (CLI and app) for working in this repository.",
        "",
        "## Installed agent rules",
        "",
        f"Created by Ballast v{BALLAST_VERSION}. Do not edit this section.",
        "",
        "Read and follow these rule files in `.codex/rules/` when they apply:",
        "",
    ]
    for agent in agents:
        for suffix in list_rule_suffixes(agent, language):
            basename = agent if suffix == "" else f"{agent}-{suffix}"
            description = get_codex_rule_description(agent, language, suffix) or f"Rules for {basename}"
            lines.append(f"- `.codex/rules/{basename}.md` — {description}")
    lines.append("")
    return "\n".join(lines)


def split_frontmatter_document(content: str) -> tuple[str | None, str]:
    match = re.match(r"^---\r?\n([\s\S]*?)\r?\n---\r?\n?", content)
    if not match:
        return None, content.lstrip()
    return match.group(0).rstrip(), content[match.end() :].lstrip()


def parse_markdown_body(content: str) -> tuple[str, list[tuple[str, str]]]:
    matches = list(re.finditer(r"^## .*(?:\r?\n|$)", content, re.MULTILINE))
    if not matches:
        return content.strip(), []

    intro = content[: matches[0].start()].strip()
    sections: list[tuple[str, str]] = []
    for index, match in enumerate(matches):
        end = matches[index + 1].start() if index + 1 < len(matches) else len(content)
        section_text = content[match.start() : end].strip()
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
        body = merge_markdown_bodies(existing_body, canonical_body)
        if existing_frontmatter:
            return f"{existing_frontmatter}\n\n{body}"
        if canonical_frontmatter:
            return f"{canonical_frontmatter}\n\n{body}"
        return body

    return merge_markdown_bodies(existing, canonical)


def find_markdown_section_range(content: str, heading: str) -> tuple[int, int] | None:
    match = re.search(rf"^## {re.escape(heading)}$", content, re.MULTILINE)
    if not match:
        return None
    next_heading = re.search(r"\n## .*", content[match.end() :])
    end = match.end() + next_heading.start() + 1 if next_heading else len(content)
    return match.start(), end


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
    root: Path, target: str, agents: list[str], language: str, force: bool, patch: bool, persist: bool
) -> InstallResult:
    result = InstallResult()
    processed_agents: list[str] = []

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
                basename = agent if suffix == "" else f"{agent}-{suffix}"
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

    if target == "codex":
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

    persist = not args.target and not args.agent and not args.all
    result = install(root, target, agents, language, bool(args.force), bool(args.patch), persist)

    if result.errors:
        for agent, error in result.errors:
            print(f"Error installing {agent}: {error}")
        return 1

    if result.installed_rules:
        print(f"Installed for {target}: {', '.join(result.installed)}")
        for agent, suffix in result.installed_rules:
            basename = agent if suffix == "" else f"{agent}-{suffix}"
            print(f"  {basename} -> {destination(root, target, basename)}")
    if result.installed_support_files:
        for file in result.installed_support_files:
            print(f"  AGENTS.md -> {file}")
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
