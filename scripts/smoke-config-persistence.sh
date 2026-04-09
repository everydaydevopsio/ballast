#!/usr/bin/env bash
# smoke-config-persistence.sh
#
# Verifies that ballast-typescript retains previously configured agents and
# skills across incremental installs.  The expected behaviour is:
#
#   - Every install is ADDITIVE: new agents/skills merge with existing ones.
#   - Installing only agents never drops previously saved skills.
#   - Installing only skills never drops previously saved agents.
#   - Reinstalling a subset of agents/skills never drops the rest.
#
# Usage:
#   ./scripts/smoke-config-persistence.sh [<repo-root>]
#
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
CLI="node ${REPO_ROOT}/packages/ballast-typescript/dist/cli.js"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
TARGETS=(cursor claude opencode codex)

# ─── helpers ─────────────────────────────────────────────────────────────────

pass() { echo "  PASS: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }

rulesrc_contents() {
  local rulesrc="$1"
  if [ -r "${rulesrc}" ]; then
    cat "${rulesrc}" 2>/dev/null || true
    return
  fi
  printf '<unreadable: %s>' "${rulesrc}"
}

assert_targets() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for target in "$@"; do
    grep -q "\"${target}\"" "${rulesrc}" || \
      fail "Expected target '${target}' in ${rulesrc} but not found. Content: $(rulesrc_contents "${rulesrc}")"
  done
}

# assert_agents <dir> <agent1> [agent2 ...]
# Checks that every listed agent appears in .rulesrc.json "agents" array.
assert_agents() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for agent in "$@"; do
    grep -q "\"${agent}\"" "${rulesrc}" || \
      fail "Expected agent '${agent}' in ${rulesrc} but not found. Content: $(rulesrc_contents "${rulesrc}")"
  done
}

# assert_no_agents <dir> <agent1> [agent2 ...]
# Checks that none of the listed agents appear in .rulesrc.json.
assert_no_agents() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for agent in "$@"; do
    grep -q "\"${agent}\"" "${rulesrc}" && \
      fail "Agent '${agent}' should NOT be in ${rulesrc} but was found. Content: $(rulesrc_contents "${rulesrc}")" || true
  done
}

# assert_skills <dir> <skill1> [skill2 ...]
assert_skills() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for skill in "$@"; do
    grep -q "\"${skill}\"" "${rulesrc}" || \
      fail "Expected skill '${skill}' in ${rulesrc} but not found. Content: $(rulesrc_contents "${rulesrc}")"
  done
}

# assert_no_skills <dir> <skill1> [skill2 ...]
assert_no_skills() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for skill in "$@"; do
    grep -q "\"${skill}\"" "${rulesrc}" && \
      fail "Skill '${skill}' should NOT be in ${rulesrc} but was found. Content: $(rulesrc_contents "${rulesrc}")" || true
  done
}

# assert_doctor_agents <dir> <agent1> [agent2 ...]
# Checks that every listed agent appears in `ballast doctor` output.
assert_doctor_agents() {
  local dir="$1"; shift
  local output
  output="$(cd "${dir}" && ${CLI} doctor 2>&1)"
  for agent in "$@"; do
    grep -Fq "${agent}" <<<"${output}" || \
      fail "Doctor did not report agent '${agent}'. Doctor output:\n${output}"
  done
}

# assert_doctor_skills <dir> <skill1> [skill2 ...]
assert_doctor_skills() {
  local dir="$1"; shift
  local output
  output="$(cd "${dir}" && ${CLI} doctor 2>&1)"
  for skill in "$@"; do
    grep -Fq "${skill}" <<<"${output}" || \
      fail "Doctor did not report skill '${skill}'. Doctor output:\n${output}"
  done
}

make_project() {
  local dir="$1"
  mkdir -p "${dir}"
  echo '{}' > "${dir}/package.json"
}

# Creates a minimal 2-language monorepo fixture with TypeScript and Python profiles.
# Required for exercising the Go CLI's monorepo configToSave logic.
make_monorepo_2lang() {
  local dir="$1"
  mkdir -p "${dir}"

  # TypeScript profile
  mkdir -p "${dir}/apps/frontend"
  echo '{}' > "${dir}/apps/frontend/package.json"
  echo '{}' > "${dir}/apps/frontend/tsconfig.json"

  # Python profile
  mkdir -p "${dir}/services/api"
  echo '{}' > "${dir}/services/api/pyproject.toml"
  echo '{}' > "${dir}/services/api/setup.py"
}

ballast() {
  local dir="$1"; shift
  (cd "${dir}" && ${CLI} "$@")
}

# ─── test: incremental agent additions ───────────────────────────────────────

test_incremental_agent_additions() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/incremental-agents-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_incremental_agent_additions (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks"
    assert_doctor_agents "${dir}" "linting" "git-hooks"
    pass "step 1 (${target}): linting installed"

    ballast "${dir}" install --target "${target}" --agent testing --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks" "testing"
    assert_doctor_agents "${dir}" "linting" "git-hooks" "testing"
    pass "step 2 (${target}): testing added; linting retained"

    ballast "${dir}" install --target "${target}" --agent logging --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks" "testing" "logging"
    assert_doctor_agents "${dir}" "linting" "git-hooks" "testing" "logging"
    pass "step 3 (${target}): logging added; linting and testing retained"
  done

  echo "  ✓ test_incremental_agent_additions passed for all targets"
}

# ─── test: adding all agents ─────────────────────────────────────────────────

test_add_all_agents() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/all-agents-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_add_all_agents (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting"
    pass "step 1 (${target}): linting installed as initial state"

    ballast "${dir}" install --target "${target}" --all --yes
    assert_targets "${dir}" "${target}"
    for agent in linting git-hooks testing logging cicd local-dev observability publishing docs; do
      assert_agents "${dir}" "${agent}"
    done
    assert_doctor_agents "${dir}" "linting"
    pass "step 2 (${target}): --all installs every agent"
  done

  echo "  ✓ test_add_all_agents passed for all targets"
}

# ─── test: incremental skill additions ───────────────────────────────────────

test_incremental_skill_additions() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/incremental-skills-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_incremental_skill_additions (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes

    ballast "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks"
    assert_skills "${dir}" "owasp-security-scan"
    assert_doctor_agents "${dir}" "linting" "git-hooks"
    assert_doctor_skills "${dir}" "owasp-security-scan"
    pass "step 1 (${target}): owasp-security-scan installed; agents retained"

    ballast "${dir}" install --target "${target}" --skill aws-health-review --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks"
    assert_skills "${dir}" "owasp-security-scan" "aws-health-review"
    assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
    pass "step 2 (${target}): aws-health-review added; owasp-security-scan and agents retained"
  done

  echo "  ✓ test_incremental_skill_additions passed for all targets"
}

# ─── test: adding all skills ─────────────────────────────────────────────────

test_add_all_skills() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/all-skills-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_add_all_skills (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    ballast "${dir}" install --target "${target}" --skill owasp-security-scan --yes

    ballast "${dir}" install --target "${target}" --all-skills --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks"
    for skill in owasp-security-scan aws-health-review aws-live-health-review aws-weekly-security-review; do
      assert_skills "${dir}" "${skill}"
    done
    assert_doctor_agents "${dir}" "linting"
    assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
    pass "step 1 (${target}): --all-skills installs every skill; agents retained"
  done

  echo "  ✓ test_add_all_skills passed for all targets"
}

# ─── test: agent install does not drop skills ─────────────────────────────────

test_agent_install_retains_skills() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/agent-retains-skills-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_agent_install_retains_skills (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    ballast "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    assert_targets "${dir}" "${target}"
    assert_skills "${dir}" "owasp-security-scan"

    ballast "${dir}" install --target "${target}" --agent testing --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks" "testing"
    assert_skills "${dir}" "owasp-security-scan"
    assert_doctor_skills "${dir}" "owasp-security-scan"
    pass "installing an agent does not drop existing skills (${target})"
  done

  echo "  ✓ test_agent_install_retains_skills passed for all targets"
}

# ─── test: skill install does not drop agents ─────────────────────────────────

test_skill_install_retains_agents() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/skill-retains-agents-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_skill_install_retains_agents (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    ballast "${dir}" install --target "${target}" --agent testing --yes

    ballast "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks" "testing"
    assert_skills "${dir}" "owasp-security-scan"
    assert_doctor_agents "${dir}" "linting" "git-hooks" "testing"
    pass "installing a skill does not drop existing agents (${target})"
  done

  echo "  ✓ test_skill_install_retains_agents passed for all targets"
}

# ─── test: reinstalling a subset of agents does not drop the rest ────────────
# This is the "removing 1 agent" scenario: specifying fewer agents in a
# subsequent install must not drop the previously configured agents.

test_reinstall_subset_agents_retained() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/subset-agents-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_reinstall_subset_agents_retained (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    ballast "${dir}" install --target "${target}" --agent testing --yes
    ballast "${dir}" install --target "${target}" --agent logging --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks" "testing" "logging"
    pass "step 1 (${target}): linting, testing, logging installed"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks" "testing" "logging"
    assert_doctor_agents "${dir}" "linting" "testing" "logging"
    pass "step 2 (${target}): reinstalling linting does not drop testing or logging"
  done

  echo "  ✓ test_reinstall_subset_agents_retained passed for all targets"
}

# ─── test: reinstalling a subset of skills does not drop the rest ────────────
# This is the "removing 1 skill" scenario.

test_reinstall_subset_skills_retained() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/subset-skills-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_reinstall_subset_skills_retained (${target})"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    ballast "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    ballast "${dir}" install --target "${target}" --skill aws-health-review --yes
    assert_targets "${dir}" "${target}"
    assert_skills "${dir}" "owasp-security-scan" "aws-health-review"
    pass "step 1 (${target}): owasp-security-scan and aws-health-review installed"

    ballast "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting" "git-hooks"
    assert_skills "${dir}" "owasp-security-scan" "aws-health-review"
    assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
    pass "step 2 (${target}): reinstalling owasp does not drop aws-health-review"
  done

  echo "  ✓ test_reinstall_subset_skills_retained passed for all targets"
}

# ─── test: full sequential scenario ──────────────────────────────────────────
# End-to-end: add all agents → add all skills → reinstall subset of agents →
# reinstall subset of skills.  At every step verify prior state is intact.

test_full_sequential() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/full-sequential-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_full_sequential (${target})"

    ballast "${dir}" install --target "${target}" --all --yes
    assert_targets "${dir}" "${target}"
    for agent in linting git-hooks testing logging cicd local-dev observability publishing docs; do
      assert_agents "${dir}" "${agent}"
    done
    pass "step 1 (${target}): --all agents installed"

    ballast "${dir}" install --target "${target}" --all-skills --yes
    assert_targets "${dir}" "${target}"
    for agent in linting git-hooks testing logging; do
      assert_agents "${dir}" "${agent}"
    done
    for skill in owasp-security-scan aws-health-review aws-live-health-review aws-weekly-security-review; do
      assert_skills "${dir}" "${skill}"
    done
    assert_doctor_agents "${dir}" "linting" "testing"
    assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
    pass "step 2 (${target}): --all-skills added; all agents retained"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    assert_targets "${dir}" "${target}"
    for agent in linting git-hooks testing logging cicd local-dev; do
      assert_agents "${dir}" "${agent}"
    done
    for skill in owasp-security-scan aws-health-review; do
      assert_skills "${dir}" "${skill}"
    done
    pass "step 3 (${target}): reinstall linting only; all other agents and skills retained"

    ballast "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    assert_targets "${dir}" "${target}"
    for agent in linting git-hooks testing logging; do
      assert_agents "${dir}" "${agent}"
    done
    for skill in owasp-security-scan aws-health-review aws-live-health-review aws-weekly-security-review; do
      assert_skills "${dir}" "${skill}"
    done
    assert_doctor_agents "${dir}" "linting" "testing"
    assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
    pass "step 4 (${target}): reinstall owasp only; all agents and other skills retained"
  done

  echo "  ✓ test_full_sequential passed for all targets"
}

# ─── test: ballast CLI (Go wrapper) agent/skill isolation ─────────────────────
#
# The ballast Go CLI writes its own configToSave after calling backends.
# These tests verify that the Go CLI does not clear agents when adding a skill
# and does not clear skills when adding an agent.

BALLAST_CLI="${REPO_ROOT}/.ballast/bin/ballast"

ballast_cli() {
  local dir="$1"; shift
  (cd "${dir}" && "${BALLAST_CLI}" "$@")
}

assert_ballast_doctor_agents() {
  local dir="$1"; shift
  local output
  output="$(cd "${dir}" && "${BALLAST_CLI}" doctor 2>&1)"
  for agent in "$@"; do
    grep -Fq "${agent}" <<<"${output}" || \
      fail "ballast doctor did not report agent '${agent}'. Output:\n${output}"
  done
}

assert_ballast_doctor_skills() {
  local dir="$1"; shift
  local output
  output="$(cd "${dir}" && "${BALLAST_CLI}" doctor 2>&1)"
  for skill in "$@"; do
    grep -Fq "${skill}" <<<"${output}" || \
      fail "ballast doctor did not report skill '${skill}'. Output:\n${output}"
  done
}

test_ballast_cli_skill_retains_agents() {
  if [ ! -x "${BALLAST_CLI}" ]; then
    echo "  SKIP: ballast binary not found at ${BALLAST_CLI}"
    return
  fi

  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/ballast-cli-skill-retains-agents-${target}"
    make_monorepo_2lang "${dir}"

    echo ""
    echo "▶ test_ballast_cli_skill_retains_agents (${target})"

    ballast_cli "${dir}" install --target "${target}" --agent linting --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting"
    assert_no_skills "${dir}" "owasp-security-scan"
    pass "step 1 (${target}): linting installed via ballast CLI"

    ballast_cli "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    assert_targets "${dir}" "${target}"
    assert_agents "${dir}" "linting"
    assert_skills "${dir}" "owasp-security-scan"
    assert_ballast_doctor_agents "${dir}" "linting"
    assert_ballast_doctor_skills "${dir}" "owasp-security-scan"
    pass "step 2 (${target}): skill added via ballast CLI; agents preserved"
  done

  echo "  ✓ test_ballast_cli_skill_retains_agents passed for all targets"
}

test_ballast_cli_agent_retains_skills() {
  if [ ! -x "${BALLAST_CLI}" ]; then
    echo "  SKIP: ballast binary not found at ${BALLAST_CLI}"
    return
  fi

  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/ballast-cli-agent-retains-skills-${target}"
    make_monorepo_2lang "${dir}"

    echo ""
    echo "▶ test_ballast_cli_agent_retains_skills (${target})"

    ballast_cli "${dir}" install --target "${target}" --skill owasp-security-scan --yes
    assert_targets "${dir}" "${target}"
    assert_skills "${dir}" "owasp-security-scan"
    pass "step 1 (${target}): skill installed via ballast CLI"

    ballast_cli "${dir}" install --target "${target}" --agent linting --yes
    assert_targets "${dir}" "${target}"
    assert_skills "${dir}" "owasp-security-scan"
    assert_agents "${dir}" "linting"
    assert_ballast_doctor_agents "${dir}" "linting"
    assert_ballast_doctor_skills "${dir}" "owasp-security-scan"
    pass "step 2 (${target}): agent added via ballast CLI; skills preserved"
  done

  echo "  ✓ test_ballast_cli_agent_retains_skills passed for all targets"
}

# ─── run all tests ────────────────────────────────────────────────────────────

main() {
  echo "Running config-persistence smoke tests..."
  echo "CLI: ${CLI}"
  echo ""

  test_incremental_agent_additions
  test_add_all_agents
  test_incremental_skill_additions
  test_add_all_skills
  test_agent_install_retains_skills
  test_skill_install_retains_agents
  test_reinstall_subset_agents_retained
  test_reinstall_subset_skills_retained
  test_full_sequential
  test_ballast_cli_skill_retains_agents
  test_ballast_cli_agent_retains_skills

  echo ""
  echo "All config-persistence smoke tests passed."
}

main "$@"
