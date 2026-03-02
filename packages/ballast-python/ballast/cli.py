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
        has_any_cfg = (directory / ".rulesrc.ts.json").exists() or (directory / ".rulesrc.json").exists() or any(
            (directory / rulesrc_filename(lang)).exists() for lang in LANGUAGES
        )
        if has_pkg or has_any_cfg:
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


def install(root: Path, target: str, agents: list[str], language: str, force: bool, persist: bool) -> InstallResult:
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
                if dst.exists() and not force:
                    agent_skipped = True
                    agent_processed = True
                    continue
                dst.parent.mkdir(parents=True, exist_ok=True)
                dst.write_text(build_content(agent, target, language, suffix), encoding="utf-8")
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
        if agents_md.exists() and not force:
            result.skipped_support_files.append(str(agents_md))
        else:
            try:
                agents_md.write_text(build_codex_agents_md(processed_agents, language), encoding="utf-8")
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
        print("Example: ballast install --yes --target cursor --agent linting")
        return 1

    target, agents = resolved
    if target not in TARGETS:
        print("Invalid --target. Use: cursor, claude, opencode, codex")
        return 1

    persist = not args.target and not args.agent and not args.all
    result = install(root, target, agents, language, bool(args.force), persist)

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
    p = argparse.ArgumentParser(prog="ballast", description="Install Ballast rules for Python projects")
    sub = p.add_subparsers(dest="command")
    install_cmd = sub.add_parser("install", help="Install rule files")
    install_cmd.add_argument("--target", "-t")
    install_cmd.add_argument("--language", "-l", default="python")
    install_cmd.add_argument("--agent", "-a")
    install_cmd.add_argument("--all", action="store_true")
    install_cmd.add_argument("--force", "-f", action="store_true")
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
