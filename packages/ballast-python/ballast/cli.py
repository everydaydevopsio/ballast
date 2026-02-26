from __future__ import annotations

import argparse
import json
from pathlib import Path

TARGETS = {"cursor", "claude", "opencode", "codex"}
COMMON_AGENTS = ["local-dev", "cicd", "observability"]
LANGUAGE_AGENTS = ["linting", "logging", "testing"]
ALL_AGENTS = COMMON_AGENTS + LANGUAGE_AGENTS
LANGUAGE = "python"
CONFIG_FILE = ".rulesrc.python.json"


def package_root() -> Path:
    env_root = Path(__import__("os").environ.get("BALLAST_REPO_ROOT", "")).resolve() if __import__("os").environ.get("BALLAST_REPO_ROOT") else None
    if env_root and env_root.exists():
        return env_root
    return Path(__file__).resolve().parents[3]


def agent_dir(agent: str) -> Path:
    root = package_root() / "agents"
    if agent in COMMON_AGENTS:
        return root / "common" / agent
    return root / LANGUAGE / agent


def resolve_project_root(cwd: Path) -> Path:
    for directory in [cwd, *cwd.parents]:
        if (directory / "package.json").exists() or (directory / CONFIG_FILE).exists():
            return directory
    return cwd


def parse_agents(raw: str | None, all_agents: bool) -> list[str]:
    if all_agents:
        return list(ALL_AGENTS)
    if not raw:
        return []
    values = [item.strip() for item in raw.split(",") if item.strip()]
    if "all" in values:
        return list(ALL_AGENTS)
    return [agent for agent in values if agent in ALL_AGENTS]


def list_rule_suffixes(agent: str) -> list[str]:
    directory = agent_dir(agent)
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


def read_content(agent: str, suffix: str = "") -> str:
    filename = "content.md" if suffix == "" else f"content-{suffix}.md"
    return (agent_dir(agent) / filename).read_text(encoding="utf-8")


def read_template(agent: str, filename: str, suffix: str = "") -> str:
    base = filename.rsplit(".", 1)[0]
    ext = filename.rsplit(".", 1)[1]
    if suffix:
        candidate = agent_dir(agent) / "templates" / f"{base}-{suffix}.{ext}"
        if candidate.exists():
            return candidate.read_text(encoding="utf-8")
    return (agent_dir(agent) / "templates" / filename).read_text(encoding="utf-8")


def build_content(agent: str, target: str, suffix: str = "") -> str:
    body = read_content(agent, suffix)
    if target == "cursor":
        return read_template(agent, "cursor-frontmatter.yaml", suffix) + "\n" + body
    if target == "claude":
        return read_template(agent, "claude-header.md", suffix) + body
    if target == "opencode":
        return read_template(agent, "opencode-frontmatter.yaml", suffix) + "\n" + body
    try:
        header = read_template(agent, "codex-header.md", suffix)
    except FileNotFoundError:
        header = read_template(agent, "claude-header.md", suffix)
    return header + body


def destination(root: Path, target: str, basename: str) -> Path:
    if target == "cursor":
        return root / ".cursor" / "rules" / f"{basename}.mdc"
    if target == "claude":
        return root / ".claude" / "rules" / f"{basename}.md"
    if target == "opencode":
        return root / ".opencode" / f"{basename}.md"
    return root / ".codex" / "rules" / f"{basename}.md"


def write_config(root: Path, target: str, agents: list[str]) -> None:
    (root / CONFIG_FILE).write_text(
        json.dumps({"target": target, "agents": agents}, indent=2),
        encoding="utf-8",
    )


def run_install(args: argparse.Namespace) -> int:
    root = resolve_project_root(Path.cwd())
    target = (args.target or "").strip().lower()
    if target not in TARGETS:
        print("Invalid --target. Use: cursor, claude, opencode, codex")
        return 1

    agents = parse_agents(args.agent, args.all)
    if not agents:
        print("No valid agents selected. Use --agent or --all")
        return 1

    write_config(root, target, agents)

    for agent in agents:
        for suffix in list_rule_suffixes(agent):
            basename = agent if suffix == "" else f"{agent}-{suffix}"
            dst = destination(root, target, basename)
            if dst.exists() and not args.force:
                continue
            dst.parent.mkdir(parents=True, exist_ok=True)
            dst.write_text(build_content(agent, target, suffix), encoding="utf-8")

    if target == "codex":
        agents_md = root / "AGENTS.md"
        if not agents_md.exists() or args.force:
            lines = [
                "# AGENTS.md",
                "",
                "Installed by ballast-python.",
                "",
                "## Installed rules",
                "",
            ]
            for agent in agents:
                for suffix in list_rule_suffixes(agent):
                    basename = agent if suffix == "" else f"{agent}-{suffix}"
                    lines.append(f"- `.codex/rules/{basename}.md`")
            agents_md.write_text("\n".join(lines) + "\n", encoding="utf-8")

    return 0


def parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="ballast", description="Install Ballast rules for Python projects"
    )
    sub = p.add_subparsers(dest="command")
    install = sub.add_parser("install", help="Install rule files")
    install.add_argument("--target", "-t")
    install.add_argument("--agent", "-a")
    install.add_argument("--all", action="store_true")
    install.add_argument("--force", "-f", action="store_true")
    install.add_argument("--yes", "-y", action="store_true")
    return p


def main(argv: list[str] | None = None) -> int:
    p = parser()
    args = p.parse_args(argv)
    command = args.command or "install"
    if command != "install":
        print(f"Unknown command: {command}")
        return 1
    return run_install(args)
