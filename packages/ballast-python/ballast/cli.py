from __future__ import annotations

import argparse
import io
import json
import os
import re
import shutil
import subprocess
import zipfile
from dataclasses import dataclass, field
from functools import lru_cache
from importlib import metadata
from pathlib import Path

TARGETS = ["cursor", "claude", "opencode", "codex", "gemini"]
LANGUAGES = ["typescript", "python", "go", "ansible", "terraform"]
COMMON_AGENTS = [
    "local-dev",
    "docs",
    "cicd",
    "observability",
    "publishing",
    "git-hooks",
]
LANGUAGE_AGENTS = ["linting", "logging", "testing"]
AGENTS_BY_LANGUAGE = {
    "typescript": COMMON_AGENTS + LANGUAGE_AGENTS,
    "python": COMMON_AGENTS + LANGUAGE_AGENTS,
    "go": COMMON_AGENTS + LANGUAGE_AGENTS,
    "ansible": COMMON_AGENTS + LANGUAGE_AGENTS,
    "terraform": COMMON_AGENTS + LANGUAGE_AGENTS,
}
COMMON_SKILLS = [
    "owasp-security-scan",
    "aws-health-review",
    "aws-live-health-review",
    "aws-weekly-security-review",
    "github-health-check",
]
SKILLS_BY_LANGUAGE = {
    "typescript": COMMON_SKILLS,
    "python": COMMON_SKILLS,
    "go": COMMON_SKILLS,
    "ansible": COMMON_SKILLS,
    "terraform": COMMON_SKILLS,
}
GIT_HOOKS_GUIDANCE_TOKEN = "{{BALLAST_GIT_HOOKS_GUIDANCE}}"


def with_implicit_agents(agents: list[str]) -> list[str]:
    resolved = list(agents)
    if "linting" in resolved and "git-hooks" not in resolved:
        resolved.append("git-hooks")
    return resolved


def cli_version() -> str:
    return ballast_version()


@dataclass
class InstallResult:
    installed: list[str] = field(default_factory=list)
    installed_rules: list[tuple[str, str]] = field(default_factory=list)
    installed_skills: list[str] = field(default_factory=list)
    installed_support_files: list[str] = field(default_factory=list)
    skipped: list[str] = field(default_factory=list)
    skipped_skills: list[str] = field(default_factory=list)
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


def normalize_target_tokens(raw: object | None) -> list[str]:
    if raw is None:
        return []
    if isinstance(raw, str):
        values = [raw]
    elif isinstance(raw, list):
        values = raw
    else:
        return []
    targets: list[str] = []
    for value in values:
        if not isinstance(value, str):
            continue
        for item in value.split(","):
            token = item.strip().lower()
            if token and token not in targets:
                targets.append(token)
    return targets


@lru_cache(maxsize=1)
def ballast_version() -> str:
    try:
        return metadata.version("ballast-python")
    except metadata.PackageNotFoundError:
        pyproject = Path(__file__).resolve().parents[1] / "pyproject.toml"
        if pyproject.exists():
            match = re.search(
                r'(?m)^version = "([^"]+)"$', pyproject.read_text(encoding="utf-8")
            )
            if match:
                return match.group(1)
    return "dev"


def agent_dir(agent: str, language: str) -> Path:
    if agent in COMMON_AGENTS:
        return resolve_agents_root() / "common" / agent
    return resolve_agents_root() / language / agent


@lru_cache(maxsize=1)
def resolve_skills_root() -> Path:
    root = package_root()
    packaged = root / "skills"
    if packaged.exists():
        return packaged

    legacy_repo_skills = Path(__file__).resolve().parents[3] / "skills"
    if legacy_repo_skills.exists():
        return legacy_repo_skills

    raise FileNotFoundError(
        "Unable to locate Ballast skills content. Reinstall ballast-python or set BALLAST_REPO_ROOT to a ballast repo checkout."
    )


def skill_dir(skill: str, language: str) -> Path:
    if skill in COMMON_SKILLS:
        return resolve_skills_root() / "common" / skill
    return resolve_skills_root() / language / skill


def resolve_project_root(cwd: Path) -> Path:
    for directory in [cwd, *cwd.parents]:
        has_pkg = (directory / "package.json").exists()
        has_go = (directory / "go.mod").exists()
        has_pyproject = (directory / "pyproject.toml").exists()
        has_ansible = any(
            (directory / marker).exists()
            for marker in (
                "ansible.cfg",
                "site.yml",
                "playbook.yml",
                "requirements.yml",
                "requirements.yaml",
            )
        )
        has_terraform = any(
            (directory / marker).exists()
            for marker in (
                ".terraform-version",
                "main.tf",
                "providers.tf",
                "versions.tf",
                "terraform.tf",
            )
        )
        has_any_cfg = (
            (directory / ".rulesrc.json").exists()
            or (directory / ".rulesrc.ts.json").exists()
            or any(
                (directory / legacy_rulesrc_filename(lang)).exists()
                for lang in LANGUAGES
            )
        )
        if (
            has_pkg
            or has_go
            or has_pyproject
            or has_ansible
            or has_terraform
            or has_any_cfg
        ):
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
        targets = normalize_target_tokens(data.get("targets"))
        legacy_target = data.get("target")
        if not targets and isinstance(legacy_target, str):
            targets = normalize_target_tokens(legacy_target)
        agents = data.get("agents")
        if not targets or not isinstance(agents, list):
            return None
        if not all(isinstance(item, str) for item in agents):
            return None
        ballast_version_value = data.get("ballastVersion")
        skills = data.get("skills")
        return {
            "targets": targets,
            "agents": agents,
            "skills": skills if isinstance(skills, list) else [],
            "ballastVersion": (
                ballast_version_value
                if isinstance(ballast_version_value, str)
                else None
            ),
            "languages": [
                item for item in data.get("languages", []) if isinstance(item, str)
            ]
            if isinstance(data.get("languages"), list)
            else [],
            "paths": {
                key: value
                for key, value in data.get("paths", {}).items()
                if isinstance(key, str)
                and isinstance(value, list)
                and all(isinstance(item, str) for item in value)
            }
            if isinstance(data.get("paths"), dict)
            else {},
        }
    except Exception:
        return None


def save_config(
    root: Path,
    language: str,
    target: str | list[str],
    agents: list[str],
    skills: list[str] | None = None,
) -> None:
    file_path = root / rulesrc_filename(language)
    languages: list[str] = []
    paths: dict[str, list[str]] = {}
    if file_path.exists():
        try:
            raw = json.loads(file_path.read_text(encoding="utf-8"))
            if isinstance(raw, dict):
                existing_languages = raw.get("languages")
                existing_paths = raw.get("paths")
                if isinstance(existing_languages, list) and all(
                    isinstance(item, str) for item in existing_languages
                ):
                    languages = list(existing_languages)
                if isinstance(existing_paths, dict):
                    for key, value in existing_paths.items():
                        if (
                            isinstance(key, str)
                            and isinstance(value, list)
                            and all(isinstance(item, str) for item in value)
                        ):
                            paths[key] = list(value)
        except Exception:
            pass

    if language not in languages:
        languages.append(language)
    for item in languages:
        if item not in paths or not paths[item]:
            paths[item] = ["."]

    targets = normalize_target_tokens(target)

    file_path.write_text(
        json.dumps(
            {
                "targets": targets,
                "agents": agents,
                "skills": skills or [],
                "ballastVersion": ballast_version(),
                "languages": languages,
                "paths": paths,
            },
            indent=2,
        ),
        encoding="utf-8",
    )


def compare_versions(left: str, right: str) -> int:
    if left == right:
        return 0
    try:
        left_parts = [int(part) for part in left.split(".")]
    except ValueError:
        left_parts = []
        left_numeric = False
    else:
        left_numeric = True
    try:
        right_parts = [int(part) for part in right.split(".")]
    except ValueError:
        right_parts = []
        right_numeric = False
    else:
        right_numeric = True
    if left_numeric and not right_numeric:
        return 1
    if not left_numeric and right_numeric:
        return -1
    if not left_numeric or not right_numeric:
        return -1 if left < right else 1
    length = max(len(left_parts), len(right_parts))
    for index in range(length):
        delta = (left_parts[index] if index < len(left_parts) else 0) - (
            right_parts[index] if index < len(right_parts) else 0
        )
        if delta != 0:
            return delta
    return 0


def latest_version(values: list[str | None]) -> str:
    versions = [value for value in values if isinstance(value, str) and value]
    if not versions:
        return ballast_version()
    current = versions[0]
    for value in versions[1:]:
        if compare_versions(value, current) > 0:
            current = value
    return current


def detect_installed_cli(name: str) -> dict[str, str | None]:
    cli_path = shutil.which(name)
    if cli_path is None:
        return {"name": name, "version": None, "path": None}
    result = subprocess.run(
        [name, "--version"], capture_output=True, text=True, check=False
    )
    version = result.stdout.strip() if result.returncode == 0 else None
    return {"name": name, "version": version, "path": cli_path}


def upgrade_command(name: str, version: str) -> str:
    _ = (name, version)
    return "ballast doctor --fix"


def build_doctor_report(
    current_cli: str,
    current_version: str,
    config_path: Path | None,
    config: dict[str, object] | None,
    installed: list[dict[str, str | None]],
) -> str:
    config_version = (
        config.get("ballastVersion")
        if isinstance(config, dict) and isinstance(config.get("ballastVersion"), str)
        else None
    )
    target_version = latest_version(
        [current_version, config_version]
        + [item.get("version") for item in installed if isinstance(item, dict)]
    )
    recommendations: list[str] = []
    needs_cli_fix = False

    for item in installed:
        version = item["version"]
        if version is None:
            needs_cli_fix = True
            continue
        if compare_versions(version, target_version) < 0:
            needs_cli_fix = True

    if needs_cli_fix:
        recommendations.append(
            "Run ballast doctor --fix to install or upgrade local Ballast CLIs."
        )

    if config_path is not None and (
        config_version is None or compare_versions(config_version, target_version) < 0
    ):
        targets = config.get("targets") if isinstance(config, dict) else None
        agents = config.get("agents") if isinstance(config, dict) else None
        if (
            isinstance(targets, list)
            and all(isinstance(target, str) for target in targets)
            and isinstance(agents, list)
            and all(isinstance(agent, str) for agent in agents)
        ):
            recommendations.append(
                f"Refresh {config_path.name} to Ballast {target_version}: "
                "ballast install --refresh-config"
            )
        else:
            recommendations.append(
                f"Refresh {config_path.name} with a current Ballast install command."
            )

    lines = [
        "Ballast doctor",
        f"Current CLI: {current_cli} {current_version}",
        "",
        "Installed CLIs:",
    ]
    for item in installed:
        if item["path"] is None:
            lines.append(f"- {item['name']}: not found")
            continue
        lines.append(
            f"- {item['name']}: {item['version'] or 'unknown'} ({item['path']})"
        )

    lines.extend(["", "Config:"])
    if config_path is None or config is None:
        lines.append("- .rulesrc.json: not found")
    else:
        lines.append(f"- file: {config_path}")
        lines.append(f"- ballastVersion: {config_version or 'missing'}")
        targets = config.get("targets")
        agents = config.get("agents")
        skills = config.get("skills")
        languages = config.get("languages")
        paths = config.get("paths")
        if isinstance(targets, list) and all(
            isinstance(target, str) for target in targets
        ):
            lines.append(f"- targets: {', '.join(targets)}")
        if isinstance(agents, list) and all(isinstance(agent, str) for agent in agents):
            lines.append(f"- agents: {', '.join(agents)}")
        if isinstance(skills, list) and all(isinstance(skill, str) for skill in skills):
            lines.append(f"- skills: {', '.join(skills)}")
        if isinstance(languages, list) and all(
            isinstance(language, str) for language in languages
        ):
            lines.append(f"- languages: {', '.join(languages)}")
        if (
            isinstance(languages, list)
            and all(isinstance(language, str) for language in languages)
            and isinstance(paths, dict)
        ):
            formatted_paths = _format_config_paths(
                languages,
                {
                    key: value
                    for key, value in paths.items()
                    if isinstance(key, str)
                    and isinstance(value, list)
                    and all(isinstance(item, str) for item in value)
                },
            )
            if formatted_paths:
                lines.append(f"- paths: {formatted_paths}")

    lines.extend(["", "Recommendations:"])
    if recommendations:
        lines.extend(f"- {item}" for item in recommendations)
    else:
        lines.append("- No action needed.")
    return "\n".join(lines) + "\n"


def _format_config_paths(
    languages: list[str], paths: dict[str, list[str]]
) -> str | None:
    ordered_keys = [
        *[language for language in languages if language in paths],
        *sorted(key for key in paths if key not in languages),
    ]
    entries = [
        f"{key}={','.join(paths[key])}"
        for key in ordered_keys
        if isinstance(paths.get(key), list) and paths[key]
    ]
    return "; ".join(entries) if entries else None


def run_doctor() -> int:
    root = resolve_project_root(Path.cwd())
    config_path = root / rulesrc_filename("python")
    if not config_path.exists():
        config_path = None
    config = load_config(root, "python")
    installed = [
        detect_installed_cli("ballast-typescript"),
        detect_installed_cli("ballast-python"),
        detect_installed_cli("ballast-go"),
    ]
    print(
        build_doctor_report(
            "ballast-python", ballast_version(), config_path, config, installed
        ),
        end="",
    )
    return 0


def parse_agent_tokens(raw: str | None, all_agents: bool, language: str) -> list[str]:
    if all_agents:
        return list(AGENTS_BY_LANGUAGE[language])
    if not raw:
        return []
    values = [item.strip() for item in raw.split(",") if item.strip()]
    if "all" in values:
        return list(AGENTS_BY_LANGUAGE[language])
    return values


def parse_skill_tokens(raw: str | None, all_skills: bool, language: str) -> list[str]:
    if all_skills:
        return list(SKILLS_BY_LANGUAGE[language])
    if not raw:
        return []
    values = [item.strip() for item in raw.split(",") if item.strip()]
    if "all" in values:
        return list(SKILLS_BY_LANGUAGE[language])
    return values


def is_valid_agent(agent: str, language: str) -> bool:
    return agent in AGENTS_BY_LANGUAGE[language]


def is_valid_skill(skill: str, language: str) -> bool:
    return skill in SKILLS_BY_LANGUAGE[language]


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


def render_git_hooks_guidance(language: str, hook_mode: str) -> str:
    if language == "typescript":
        if hook_mode == "monorepo":
            return "\n".join(
                [
                    "- Use Husky for this monorepo.",
                    "- Install and initialize Husky.",
                    "- Create `.husky/pre-commit` with the repo's fast lint command, such as `npx lint-staged`.",
                    "- Create `.husky/pre-push` with the repo's unit test command, and for TypeScript monorepos run the build before the tests when the test command depends on generated output.",
                    "- Keep the hook file executable with `chmod +x .husky/pre-commit`.",
                    "- Keep `.husky/pre-push` executable with `chmod +x .husky/pre-push`.",
                    "- Keep the hook in sync with the repo's linting workflow whenever the command changes.",
                ]
            )
        return "\n".join(
            [
                "- Use `pre-commit` for this repository layout.",
                "- Create `.pre-commit-config.yaml` at the repo root.",
                "- Install hooks with `pre-commit install`.",
                "- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
                "- Configure `.pre-commit-config.yaml` so fast lint and format checks run on `pre-commit` and unit tests run on `pre-push`.",
                "- Keep the configuration current with `pre-commit autoupdate`.",
                "- Verify the hook configuration with `pre-commit run --all-files`.",
            ]
        )
    if language == "python":
        return "\n".join(
            [
                "- Use `pre-commit` for Python projects.",
                "- Create `.pre-commit-config.yaml` at the repo root.",
                "- Install hooks with `pre-commit install`.",
                "- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
                "- Configure `.pre-commit-config.yaml` so unit tests run on `pre-push`.",
                "- Keep the configuration current with `pre-commit autoupdate`.",
                "- Re-run `pre-commit run --all-files` after hook changes.",
            ]
        )
    if language == "go":
        return "\n".join(
            [
                "- Use `pre-commit` for Go projects, and fan out to language-local configs with `sub-pre-commit` when needed.",
                "- Create or update `.pre-commit-config.yaml` at the repo root.",
                "- Use `sub-pre-commit` hooks to invoke nested `.pre-commit-config.yaml` files in Go subprojects.",
                "- Install hooks with `pre-commit install` and `pre-commit install --hook-type pre-push`.",
                "- Configure the pre-push stage to run Go unit tests for each module.",
                "- Keep the configuration current with `pre-commit autoupdate`.",
                "- Verify the hook configuration with `pre-commit run --all-files`.",
            ]
        )
    if language == "ansible":
        return "\n".join(
            [
                "- Use `pre-commit` for Ansible repositories.",
                "- Create or update `.pre-commit-config.yaml` at the repo root.",
                "- Install hooks with `pre-commit install`.",
                "- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
                "- Run `ansible-lint`, `yamllint`, and `ansible-playbook --syntax-check` from the hook configuration.",
                "- Keep secrets out of logs and commits; prefer Ansible Vault or external secret stores.",
                "- Keep the configuration current with `pre-commit autoupdate`.",
            ]
        )
    if language == "terraform":
        return "\n".join(
            [
                "- Use `pre-commit` for Terraform repositories.",
                "- Create or update `.pre-commit-config.yaml` at the repo root.",
                "- Commit `.terraform-version` and use `tfenv install` plus `tfenv use` before running Terraform commands.",
                "- Install hooks with `pre-commit install`.",
                "- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
                "- Run `terraform fmt -check -recursive`, `terraform validate`, `tflint`, and `tfsec` from the hook configuration.",
                "- Keep `.terraform/`, state files, and plan files out of Git.",
                "- Keep the configuration current with `pre-commit autoupdate`.",
            ]
        )
    return ""


def has_workspace_monorepo(root: Path) -> bool:
    package_json = root / "package.json"
    if not (root / "pnpm-workspace.yaml").exists():
        if not package_json.exists():
            return False
        try:
            raw = json.loads(package_json.read_text(encoding="utf-8"))
        except Exception:
            return False
        if not raw.get("workspaces"):
            return False

    ignored = {
        ".git",
        "node_modules",
        "dist",
        "build",
        "coverage",
        ".next",
        ".turbo",
        ".pnpm-store",
    }
    count = 0
    for current, dirs, files in os.walk(root):
        rel = Path(current).relative_to(root)
        if len(rel.parts) > 4:
            dirs[:] = []
            continue
        dirs[:] = [name for name in dirs if name not in ignored]
        if "package.json" in files:
            count += 1
            if count > 1:
                return True
    return False


def resolve_ts_hook_mode(root: Path, language: str) -> str:
    if language != "typescript":
        return "standalone"

    config_path = root / ".rulesrc.json"
    if config_path.exists():
        try:
            raw = json.loads(config_path.read_text(encoding="utf-8"))
            languages = raw.get("languages") or []
            if (
                isinstance(languages, list)
                and len({item for item in languages if isinstance(item, str)}) > 1
            ):
                return "monorepo"
            paths = raw.get("paths") or {}
            if isinstance(paths, dict) and len(paths.keys()) > 1:
                return "monorepo"
        except Exception:
            pass

    if has_workspace_monorepo(root):
        return "monorepo"
    return "standalone"


def apply_hook_guidance(
    content: str, agent: str, language: str, root: Path | None
) -> str:
    if agent != "git-hooks" or GIT_HOOKS_GUIDANCE_TOKEN not in content:
        return content
    hook_mode = (
        resolve_ts_hook_mode(root, language) if root is not None else "standalone"
    )
    return content.replace(
        GIT_HOOKS_GUIDANCE_TOKEN, render_git_hooks_guidance(language, hook_mode)
    )


def read_template(agent: str, language: str, filename: str, suffix: str = "") -> str:
    base, ext = filename.rsplit(".", 1)
    if suffix:
        candidate = agent_dir(agent, language) / "templates" / f"{base}-{suffix}.{ext}"
        if candidate.exists():
            return candidate.read_text(encoding="utf-8")
    return (agent_dir(agent, language) / "templates" / filename).read_text(
        encoding="utf-8"
    )


def build_content(
    agent: str, target: str, language: str, suffix: str = "", root: Path | None = None
) -> str:
    body = apply_hook_guidance(
        read_content(agent, language, suffix), agent, language, root
    )
    if target == "cursor":
        return (
            read_template(agent, language, "cursor-frontmatter.yaml", suffix)
            + "\n"
            + body
        )
    if target == "claude":
        return read_template(agent, language, "claude-header.md", suffix) + body
    if target == "gemini":
        try:
            header = read_template(agent, language, "gemini-header.md", suffix)
        except FileNotFoundError:
            try:
                header = read_template(agent, language, "claude-header.md", suffix)
            except FileNotFoundError:
                header = read_template(agent, language, "codex-header.md", suffix)
        return header + body
    if target == "opencode":
        return (
            read_template(agent, language, "opencode-frontmatter.yaml", suffix)
            + "\n"
            + body
        )
    try:
        header = read_template(agent, language, "codex-header.md", suffix)
    except FileNotFoundError:
        header = read_template(agent, language, "claude-header.md", suffix)
    return header + body


def read_skill(skill: str, language: str) -> str:
    return (skill_dir(skill, language) / "SKILL.md").read_text(encoding="utf-8")


def split_skill_document(content: str) -> tuple[str | None, str]:
    normalized = normalize_line_endings(content)
    match = re.match(r"^---\n([\s\S]*?)\n---\n?", normalized)
    if not match:
        return None, normalized.lstrip()
    return match.group(0).rstrip(), normalized[match.end() :].lstrip()


def skill_description(skill: str, language: str) -> str:
    frontmatter, _ = split_skill_document(read_skill(skill, language))
    if not frontmatter:
        return f"Skill {skill}"
    description = extract_description_from_frontmatter(frontmatter)
    return description or f"Skill {skill}"


def build_cursor_skill_format(skill: str, language: str) -> str:
    _, body = split_skill_document(read_skill(skill, language))
    description = skill_description(skill, language).replace('"', '\\"')
    return (
        f'---\ndescription: "{description}"\nalwaysApply: false\n---\n\n'
        + body.rstrip()
        + "\n"
    )


def build_skill_markdown(skill: str, language: str) -> str:
    _, body = split_skill_document(read_skill(skill, language))
    return body.rstrip() + "\n"


def build_claude_skill(skill: str, language: str) -> bytes:
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_STORED) as archive:
        archive.writestr("SKILL.md", read_skill(skill, language))
        references = skill_dir(skill, language) / "references"
        if references.exists():
            for ref in sorted(references.rglob("*")):
                if ref.is_file():
                    archive.write(
                        ref, f"references/{ref.relative_to(references).as_posix()}"
                    )
    return output.getvalue()


def destination(root: Path, target: str, basename: str) -> Path:
    rule_subdir = os.environ.get("BALLAST_RULE_SUBDIR", "").strip()
    if rule_subdir and not re.fullmatch(r"[A-Za-z0-9_-]+", rule_subdir):
        raise ValueError(
            f"Invalid BALLAST_RULE_SUBDIR value {rule_subdir!r}. Only letters, digits, '-' and '_' are allowed."
        )
    scoped_basename = (
        basename
        if not rule_subdir
        or rule_subdir == "common"
        or basename.startswith(f"{rule_subdir}-")
        else f"{rule_subdir}-{basename}"
    )
    if target == "cursor":
        base = root / ".cursor" / "rules"
        return (
            (base / rule_subdir / f"{scoped_basename}.mdc")
            if rule_subdir
            else (base / f"{scoped_basename}.mdc")
        )
    if target == "claude":
        base = root / ".claude" / "rules"
        return (
            (base / rule_subdir / f"{scoped_basename}.md")
            if rule_subdir
            else (base / f"{scoped_basename}.md")
        )
    if target == "gemini":
        base = root / ".gemini" / "rules"
        return (
            (base / rule_subdir / f"{scoped_basename}.md")
            if rule_subdir
            else (base / f"{scoped_basename}.md")
        )
    if target == "opencode":
        base = root / ".opencode"
        return (
            (base / rule_subdir / f"{scoped_basename}.md")
            if rule_subdir
            else (base / f"{scoped_basename}.md")
        )
    base = root / ".codex" / "rules"
    return (
        (base / rule_subdir / f"{scoped_basename}.md")
        if rule_subdir
        else (base / f"{scoped_basename}.md")
    )


def ensure_gitignore_entry(root: Path, entry: str) -> None:
    normalized = entry.strip()
    if not normalized:
        return
    gitignore = root / ".gitignore"
    if not gitignore.exists():
        gitignore.write_text(f"{normalized}\n", encoding="utf-8")
        return
    content = gitignore.read_text(encoding="utf-8")
    if any(line.strip() == normalized for line in content.splitlines()):
        return
    separator = "" if not content or content.endswith("\n") else "\n"
    gitignore.write_text(f"{content}{separator}{normalized}\n", encoding="utf-8")


def skill_destination(root: Path, target: str, skill: str) -> Path:
    if target == "cursor":
        return root / ".cursor" / "rules" / f"{skill}.mdc"
    if target == "claude":
        return root / ".claude" / "skills" / f"{skill}.skill"
    if target == "gemini":
        return root / ".gemini" / "rules" / f"{skill}.md"
    if target == "opencode":
        return root / ".opencode" / "skills" / f"{skill}.md"
    return root / ".codex" / "rules" / f"{skill}.md"


def extract_description_from_frontmatter(frontmatter: str) -> str | None:
    lines = frontmatter.replace("\r\n", "\n").replace("\r", "\n").split("\n")
    for index, line in enumerate(lines):
        stripped = line.strip()
        if not stripped.startswith("description:"):
            continue
        value = stripped.removeprefix("description:").strip()
        if value in {">", "|", ">-", "|-", ">+", "|+"}:
            values: list[str] = []
            for candidate in lines[index + 1 :]:
                if candidate.strip() == "":
                    if values:
                        values.append("")
                    continue
                if len(candidate) - len(candidate.lstrip(" ")) < 2:
                    break
                values.append(candidate.strip())
            if not values:
                return None
            if value.startswith(">"):
                folded = " ".join(item for item in values if item)
                return folded or None
            literal = "\n".join(values).strip()
            return literal or None
        value = value.strip("'\"")
        return value or None
    match = re.search(r"(?m)^description:\s*['\"]?(.+?)['\"]?\s*$", frontmatter)
    if match:
        value = match.group(1).strip().strip("'\"")
        return value or None
    return None


def get_codex_rule_description(
    agent: str, language: str, suffix: str = ""
) -> str | None:
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


def ballast_notice() -> str:
    return (
        f"Created by [Ballast](https://github.com/everydaydevopsio/ballast) "
        f"v{ballast_version()}. Do not edit this section."
    )


def repository_facts_section() -> list[str]:
    return [
        "## Repository Facts",
        "",
        "Use this section for durable repo-specific facts that agents repeatedly need. Prefer facts stored here over re-deriving them with shell commands on every task.",
        "",
        "Keep only stable, reviewable metadata here. Do not store secrets, credentials, or ephemeral runtime state.",
        "",
        "Suggested facts to record:",
        "",
        "- Canonical GitHub repo: `<OWNER/REPO>`",
        "- Default branch: `<main>`",
        "- Primary package manager: `<pnpm | npm | yarn | uv | go>`",
        "- Version-file locations agents should check first: `<.nvmrc, packageManager, pyproject.toml, go.mod, etc.>`",
        "- Canonical config files: `<paths agents should read before falling back to discovery>`",
        "- Primary CI workflows: `<workflow filenames>`",
        "- Primary release/publish workflows: `<workflow filenames>`",
        "- Preferred build/test/lint/format/coverage commands: `<commands>`",
        "- Coverage threshold: `<value>`",
        "- Generated or protected paths agents should avoid editing directly: `<paths>`",
        "",
        "Update this section when those facts change. If live runtime state is required, discover it separately instead of treating it as a durable repo fact.",
    ]


def build_codex_agents_md(agents: list[str], skills: list[str], language: str) -> str:
    lines = [
        "# AGENTS.md",
        "",
        "This file provides shared repository guidance for agent tools that read AGENTS.md.",
        "",
        *repository_facts_section(),
        "",
        "## Installed agent rules",
        "",
        ballast_notice(),
        "",
        "Read and follow these rule files in `.codex/rules/` when they apply:",
        "",
    ]
    for agent in agents:
        for suffix in list_rule_suffixes(agent, language):
            basename = rule_basename(agent, language, suffix)
            description = (
                get_codex_rule_description(agent, language, suffix)
                or f"Rules for {basename}"
            )
            lines.append(f"- `.codex/rules/{basename}.md` — {description}")
    if skills:
        lines.extend(
            [
                "",
                "## Installed skills",
                "",
                ballast_notice(),
                "",
                "Read and use these skill files in `.codex/rules/` when they are relevant:",
                "",
            ]
        )
        for skill in skills:
            lines.append(
                f"- `.codex/rules/{skill}.md` — {skill_description(skill, language)}"
            )
    lines.append("")
    return "\n".join(lines)


def build_claude_md(agents: list[str], skills: list[str], language: str) -> str:
    lines = [
        "# CLAUDE.md",
        "",
        "This file provides guidance to Claude Code for working in this repository.",
        "",
        *repository_facts_section(),
        "",
        "## Installed agent rules",
        "",
        ballast_notice(),
        "",
        "Read and follow these rule files in `.claude/rules/` when they apply:",
        "",
    ]
    for agent in agents:
        for suffix in list_rule_suffixes(agent, language):
            basename = rule_basename(agent, language, suffix)
            description = (
                get_codex_rule_description(agent, language, suffix)
                or f"Rules for {basename}"
            )
            lines.append(f"- `.claude/rules/{basename}.md` — {description}")
    if skills:
        lines.extend(
            [
                "",
                "## Installed skills",
                "",
                ballast_notice(),
                "",
                "Read and use these skill files in `.claude/skills/` when they are relevant:",
                "",
            ]
        )
        for skill in skills:
            lines.append(
                f"- `.claude/skills/{skill}.skill` — {skill_description(skill, language)}"
            )
    lines.append("")
    return "\n".join(lines)


def build_gemini_md(agents: list[str], skills: list[str], language: str) -> str:
    lines = [
        "# GEMINI.md",
        "",
        "This file provides guidance to Gemini CLI for working in this repository.",
        "",
        "@./AGENTS.md",
        "",
        "## Installed agent rules",
        "",
        ballast_notice(),
        "",
        "Read and follow these rule files in `.gemini/rules/` when they apply:",
        "",
    ]
    for agent in agents:
        for suffix in list_rule_suffixes(agent, language):
            basename = rule_basename(agent, language, suffix)
            description = (
                get_codex_rule_description(agent, language, suffix)
                or f"Rules for {basename}"
            )
            lines.append(f"- `.gemini/rules/{basename}.md` — {description}")
    if skills:
        lines.extend(
            [
                "",
                "## Installed skills",
                "",
                ballast_notice(),
                "",
                "Read and use these skill files in `.gemini/rules/` when they are relevant:",
                "",
            ]
        )
        for skill in skills:
            lines.append(
                f"- `.gemini/rules/{skill}.md` — {skill_description(skill, language)}"
            )
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


def merge_frontmatter(
    existing_frontmatter: str | None, canonical_frontmatter: str | None
) -> str | None:
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
        end = (
            matches[index + 1].start() if index + 1 < len(matches) else len(normalized)
        )
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

    next_content = existing
    for heading in ("Installed agent rules", "Installed skills"):
        canonical_range = find_markdown_section_range(canonical, heading)
        if not canonical_range:
            continue
        canonical_section = canonical[canonical_range[0] : canonical_range[1]].rstrip()

        existing_range = find_markdown_section_range(next_content, heading)
        if not existing_range:
            next_content = next_content.rstrip() + "\n\n" + canonical_section + "\n"
            continue

        next_content = (
            next_content[: existing_range[0]].rstrip()
            + "\n\n"
            + canonical_section
            + "\n\n"
            + next_content[existing_range[1] :].lstrip()
        ).rstrip() + "\n"

    return next_content


def prompt(question: str) -> str:
    return input(question).strip()


def prompt_yes_no(question: str, default: bool = False) -> bool:
    suffix = " [Y/n]: " if default else " [y/N]: "
    value = prompt(question + suffix).lower()
    if not value:
        return default
    return value in {"y", "yes"}


def prompt_targets() -> list[str]:
    value = prompt(f"AI platform(s) ({', '.join(TARGETS)}, comma-separated): ")
    if value.strip().lower() == "all":
        return list(TARGETS)
    resolved = normalize_target_tokens(value)
    if resolved and all(target in TARGETS for target in resolved):
        return resolved
    print(f"Invalid targets. Choose from: {', '.join(TARGETS)}")
    return prompt_targets()


def prompt_agents(language: str) -> list[str]:
    agent_ids = AGENTS_BY_LANGUAGE[language]
    value = prompt(f'Agents (comma-separated or "all") [{", ".join(agent_ids)}]: ')
    if not value:
        return list(agent_ids)
    tokens = parse_agent_tokens(value, False, language)
    resolved = [t for t in tokens if is_valid_agent(t, language)]
    if resolved:
        return resolved
    print(f"Invalid agents. Use 'all' or comma-separated: {', '.join(agent_ids)}")
    return prompt_agents(language)


def prompt_skills(language: str) -> list[str]:
    skill_ids = SKILLS_BY_LANGUAGE[language]
    if not skill_ids:
        return []
    value = prompt(
        f'Skills (comma-separated, "all", or blank for none) [{", ".join(skill_ids)}]: '
    )
    if not value:
        return []
    tokens = parse_skill_tokens(value, False, language)
    resolved = [t for t in tokens if is_valid_skill(t, language)]
    if resolved:
        return resolved
    print(f"Invalid skills. Use 'all' or comma-separated: {', '.join(skill_ids)}")
    return prompt_skills(language)


def resolve_requested_targets(raw: object) -> list[str]:
    if raw is None:
        return []
    if isinstance(raw, list):
        return normalize_target_tokens([str(item) for item in raw if item is not None])
    if isinstance(raw, str):
        return normalize_target_tokens(raw)
    return normalize_target_tokens(str(raw))


def resolve_target_and_agents(
    args: argparse.Namespace, root: Path, language: str
) -> tuple[list[str], list[str], list[str]] | None:
    cfg = load_config(root, language)
    ci = is_ci_mode() or bool(args.yes)

    target_from_flag = resolve_requested_targets(getattr(args, "target", None))
    agents_from_flag = (
        parse_agent_tokens(args.agent, bool(args.all), language)
        if (args.agent or args.all)
        else None
    )
    skills_from_flag = (
        parse_skill_tokens(args.skill, bool(args.all_skills), language)
        if (getattr(args, "skill", None) or bool(getattr(args, "all_skills", False)))
        else None
    )

    if (
        cfg
        and not target_from_flag
        and agents_from_flag is None
        and skills_from_flag is None
    ):
        return (
            list(cfg["targets"]),
            with_implicit_agents(list(cfg["agents"])),
            list(cfg.get("skills") or []),
        )

    targets = target_from_flag or (list(cfg["targets"]) if cfg else None)
    agents = (
        with_implicit_agents(agents_from_flag)
        if agents_from_flag is not None
        else (with_implicit_agents(list(cfg["agents"])) if cfg else None)
    )
    skills = (
        skills_from_flag
        if skills_from_flag is not None
        else (list(cfg.get("skills") or []) if cfg else [])
    )

    if targets and ((agents and len(agents) > 0) or len(skills) > 0):
        return targets, agents or [], skills

    if ci:
        return None

    resolved_targets = targets if targets else prompt_targets()
    resolved_agents = agents if agents and len(agents) > 0 else prompt_agents(language)
    resolved_skills = skills if skills else prompt_skills(language)
    return resolved_targets, resolved_agents, resolved_skills


def install(
    root: Path,
    target: str,
    agents: list[str],
    skills: list[str],
    language: str,
    force: bool,
    patch: bool,
    persist: bool,
    patch_claude_md: bool = False,
    patch_gemini_md: bool = False,
) -> InstallResult:
    result = InstallResult()
    agents = with_implicit_agents(agents)
    processed_agents: list[str] = []
    processed_skills: list[str] = []
    disable_support_files = os.environ.get("BALLAST_DISABLE_SUPPORT_FILES") == "1"

    try:
        ensure_gitignore_entry(root, ".ballast/")
    except Exception as err:
        result.errors.append(("gitignore", str(err)))

    if persist:
        save_config(root, language, target, agents, skills)

    config_for_support_files = load_config(root, language)
    support_agents = with_implicit_agents(
        config_for_support_files.get("agents", agents)
        if config_for_support_files
        else agents
    )
    support_skills = (
        config_for_support_files.get("skills", skills)
        if config_for_support_files
        else skills
    )

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
                content = build_content(agent, target, language, suffix, root)
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

    for skill in skills:
        if not is_valid_skill(skill, language):
            result.errors.append((skill, "Unknown skill"))
            continue
        try:
            dst = skill_destination(root, target, skill)
            if dst.exists() and not force:
                result.skipped_skills.append(skill)
                processed_skills.append(skill)
                continue
            dst.parent.mkdir(parents=True, exist_ok=True)
            if target == "cursor":
                dst.write_text(
                    build_cursor_skill_format(skill, language), encoding="utf-8"
                )
            elif target == "claude":
                dst.write_bytes(build_claude_skill(skill, language))
            else:
                dst.write_text(build_skill_markdown(skill, language), encoding="utf-8")
            result.installed_skills.append(skill)
            processed_skills.append(skill)
        except Exception as err:
            result.errors.append((skill, str(err)))

    if target == "claude" and not disable_support_files:
        claude_md = root / "CLAUDE.md"
        should_patch_claude_md = patch or patch_claude_md
        if claude_md.exists() and not force and not should_patch_claude_md:
            result.skipped_support_files.append(str(claude_md))
        else:
            try:
                content = build_claude_md(support_agents, support_skills, language)
                next_content = (
                    patch_codex_agents_md(
                        claude_md.read_text(encoding="utf-8"), content
                    )
                    if claude_md.exists() and not force and should_patch_claude_md
                    else content
                )
                claude_md.write_text(next_content, encoding="utf-8")
                result.installed_support_files.append(str(claude_md))
            except Exception as err:
                result.errors.append(("claude", str(err)))

    if target == "gemini" and not disable_support_files:
        gemini_md = root / "GEMINI.md"
        agents_md = root / "AGENTS.md"
        should_patch_gemini_md = patch or patch_gemini_md
        if gemini_md.exists() and not force and not should_patch_gemini_md:
            result.skipped_support_files.append(str(gemini_md))
        else:
            try:
                content = build_gemini_md(support_agents, support_skills, language)
                next_content = (
                    patch_codex_agents_md(
                        gemini_md.read_text(encoding="utf-8"), content
                    )
                    if gemini_md.exists() and not force and should_patch_gemini_md
                    else content
                )
                gemini_md.write_text(next_content, encoding="utf-8")
                result.installed_support_files.append(str(gemini_md))
            except Exception as err:
                result.errors.append(("gemini", str(err)))

        if not agents_md.exists():
            try:
                agents_md.write_text(
                    build_codex_agents_md(support_agents, support_skills, language),
                    encoding="utf-8",
                )
                result.installed_support_files.append(str(agents_md))
            except Exception as err:
                result.errors.append(("codex", str(err)))

    if target == "codex" and not disable_support_files:
        agents_md = root / "AGENTS.md"
        if agents_md.exists() and not force and not patch:
            result.skipped_support_files.append(str(agents_md))
        else:
            try:
                content = build_codex_agents_md(
                    support_agents, support_skills, language
                )
                next_content = (
                    patch_codex_agents_md(
                        agents_md.read_text(encoding="utf-8"), content
                    )
                    if agents_md.exists() and not force and patch
                    else content
                )
                agents_md.write_text(next_content, encoding="utf-8")
                result.installed_support_files.append(str(agents_md))
            except Exception as err:
                result.errors.append(("codex", str(err)))

    return result


def install_for_targets(
    root: Path,
    targets: list[str],
    agents: list[str],
    skills: list[str],
    language: str,
    force: bool,
    patch: bool,
    persist: bool,
    patch_claude_md: bool = False,
    patch_gemini_md: bool = False,
) -> list[tuple[str, InstallResult]]:
    results: list[tuple[str, InstallResult]] = []
    agents = with_implicit_agents(agents)
    if persist:
        save_config(root, language, targets, agents, skills)

    for target in targets:
        results.append(
            (
                target,
                install(
                    root,
                    target,
                    agents,
                    skills,
                    language,
                    force,
                    patch,
                    False,
                    patch_claude_md,
                    patch_gemini_md,
                ),
            )
        )
    return results


def print_install_result(
    root: Path, target: str, result: InstallResult, language: str
) -> None:
    if result.errors:
        for agent, error in result.errors:
            print(f"Error installing {agent}: {error}")
        return

    if result.installed_rules:
        print(f"Installed for {target}: {', '.join(result.installed)}")
        for agent, suffix in result.installed_rules:
            basename = rule_basename(agent, language, suffix)
            print(f"  {basename} -> {destination(root, target, basename)}")
    if result.installed_skills:
        print(f"Installed skills for {target}: {', '.join(result.installed_skills)}")
        for skill in result.installed_skills:
            print(f"  {skill} -> {skill_destination(root, target, skill)}")
    if result.installed_support_files:
        for file in result.installed_support_files:
            label = "AGENTS.md"
            if file.endswith("CLAUDE.md"):
                label = "CLAUDE.md"
            elif file.endswith("GEMINI.md"):
                label = "GEMINI.md"
            print(f"  {label} -> {file}")
    if result.skipped:
        print(
            "Skipped (already present; use --force to overwrite): "
            + ", ".join(result.skipped)
        )
    if result.skipped_skills:
        print(
            "Skipped skills (already present; use --force to overwrite): "
            + ", ".join(result.skipped_skills)
        )
    if result.skipped_support_files:
        print(
            "Skipped support files (already present; use --force to overwrite): "
            + ", ".join(result.skipped_support_files)
        )


def run_install(args: argparse.Namespace) -> int:
    language = (args.language or "python").strip().lower()
    if language not in LANGUAGES:
        print(f"Invalid --language. Use: {', '.join(LANGUAGES)}")
        return 1

    root = resolve_project_root(Path.cwd())
    resolved = resolve_target_and_agents(args, root, language)
    if not resolved:
        print(
            "In CI/non-interactive mode (--yes or CI env), --target and at least one of --agent/--all or --skill/--all-skills are required when config is missing."
        )
        print(
            "Example: ballast-python install --yes --target cursor --agent linting --skill owasp-security-scan"
        )
        return 1

    targets, agents, skills = resolved
    if any(target not in TARGETS for target in targets):
        print(f"Invalid --target. Use: {', '.join(TARGETS)}")
        return 1

    patch_claude_md = False
    if "claude" in targets and (root / "CLAUDE.md").exists() and not bool(args.force):
        if bool(args.patch):
            patch_claude_md = True
        elif not is_ci_mode() and not bool(args.yes):
            patch_claude_md = prompt_yes_no(
                f"Existing CLAUDE.md found at {root / 'CLAUDE.md'}. Patch the Installed agent rules section?"
            )

    patch_gemini_md = False
    if "gemini" in targets and (root / "GEMINI.md").exists() and not bool(args.force):
        if bool(args.patch):
            patch_gemini_md = True
        elif not is_ci_mode() and not bool(args.yes):
            patch_gemini_md = prompt_yes_no(
                f"Existing GEMINI.md found at {root / 'GEMINI.md'}. Patch the Installed agent rules section?"
            )

    if len(targets) == 1:
        per_target_results = [
            (
                targets[0],
                install(
                    root,
                    targets[0],
                    agents,
                    skills,
                    language,
                    bool(args.force),
                    bool(args.patch),
                    True,
                    patch_claude_md,
                    patch_gemini_md,
                ),
            )
        ]
    else:
        per_target_results = install_for_targets(
            root,
            targets,
            agents,
            skills,
            language,
            bool(args.force),
            bool(args.patch),
            True,
            patch_claude_md,
            patch_gemini_md,
        )

    combined = InstallResult()
    for target, result in per_target_results:
        print_install_result(root, target, result, language)
        combined.installed.extend(result.installed)
        combined.installed_rules.extend(result.installed_rules)
        combined.installed_skills.extend(result.installed_skills)
        combined.installed_support_files.extend(result.installed_support_files)
        combined.skipped.extend(result.skipped)
        combined.skipped_skills.extend(result.skipped_skills)
        combined.skipped_support_files.extend(result.skipped_support_files)
        combined.errors.extend(result.errors)

    if combined.errors:
        return 1
    if (
        not combined.installed
        and not combined.installed_skills
        and not combined.skipped
        and not combined.skipped_skills
        and not combined.errors
    ):
        print("Nothing to install.")

    return 0


def parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="ballast-python",
        description="Install Ballast rules for Python projects",
        epilog=(
            "Examples:\n"
            "  ballast-python install --target cursor --agent linting\n"
            "  ballast-python install --target cursor,claude --agent linting --yes\n"
            "  ballast-python install --target cursor --target codex --all --yes"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("--version", action="version", version=cli_version())
    sub = p.add_subparsers(dest="command")
    install_cmd = sub.add_parser("install", help="Install rule files")
    install_cmd.add_argument(
        "--target",
        "-t",
        action="append",
        help="One or more targets (comma-separated or repeated): cursor, claude, opencode, codex, gemini",
    )
    install_cmd.add_argument("--language", "-l", default="python")
    install_cmd.add_argument("--agent", "-a")
    install_cmd.add_argument("--skill", "-s")
    install_cmd.add_argument("--all", action="store_true")
    install_cmd.add_argument("--all-skills", action="store_true")
    install_cmd.add_argument("--force", "-f", action="store_true")
    install_cmd.add_argument("--patch", "-p", action="store_true")
    install_cmd.add_argument("--yes", "-y", action="store_true")
    sub.add_parser(
        "doctor", help="Check local Ballast CLI versions and .rulesrc.json metadata"
    )
    return p


def main(argv: list[str] | None = None) -> int:
    p = parser()
    args = p.parse_args(argv)
    command = args.command or "install"
    if command == "doctor":
        return run_doctor()
    if command != "install":
        print(f"Unknown command: {command}")
        return 1
    return run_install(args)
