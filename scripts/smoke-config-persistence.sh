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

# ─── helpers ─────────────────────────────────────────────────────────────────

pass() { echo "  PASS: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }

# assert_agents <dir> <agent1> [agent2 ...]
# Checks that every listed agent appears in .rulesrc.json "agents" array.
assert_agents() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for agent in "$@"; do
    grep -q "\"${agent}\"" "${rulesrc}" || \
      fail "Expected agent '${agent}' in ${rulesrc} but not found. Content: $(cat "${rulesrc}")"
  done
}

# assert_no_agents <dir> <agent1> [agent2 ...]
# Checks that none of the listed agents appear in .rulesrc.json.
assert_no_agents() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for agent in "$@"; do
    grep -q "\"${agent}\"" "${rulesrc}" && \
      fail "Agent '${agent}' should NOT be in ${rulesrc} but was found. Content: $(cat "${rulesrc}")" || true
  done
}

# assert_skills <dir> <skill1> [skill2 ...]
assert_skills() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for skill in "$@"; do
    grep -q "\"${skill}\"" "${rulesrc}" || \
      fail "Expected skill '${skill}' in ${rulesrc} but not found. Content: $(cat "${rulesrc}")"
  done
}

# assert_no_skills <dir> <skill1> [skill2 ...]
assert_no_skills() {
  local dir="$1"; shift
  local rulesrc="${dir}/.rulesrc.json"
  for skill in "$@"; do
    grep -q "\"${skill}\"" "${rulesrc}" && \
      fail "Skill '${skill}' should NOT be in ${rulesrc} but was found. Content: $(cat "${rulesrc}")" || true
  done
}

# assert_doctor_agents <dir> <agent1> [agent2 ...]
# Checks that every listed agent appears in `ballast doctor` output.
assert_doctor_agents() {
  local dir="$1"; shift
  local output
  output="$(cd "${dir}" && ${CLI} doctor 2>&1)"
  for agent in "$@"; do
    echo "${output}" | grep -q "${agent}" || \
      fail "Doctor did not report agent '${agent}'. Doctor output:\n${output}"
  done
}

# assert_doctor_skills <dir> <skill1> [skill2 ...]
assert_doctor_skills() {
  local dir="$1"; shift
  local output
  output="$(cd "${dir}" && ${CLI} doctor 2>&1)"
  for skill in "$@"; do
    echo "${output}" | grep -q "${skill}" || \
      fail "Doctor did not report skill '${skill}'. Doctor output:\n${output}"
  done
}

make_project() {
  local dir="$1"
  mkdir -p "${dir}"
  echo '{}' > "${dir}/package.json"
}

ballast() {
  local dir="$1"; shift
  (cd "${dir}" && ${CLI} "$@")
}

# ─── test: incremental agent additions ───────────────────────────────────────

test_incremental_agent_additions() {
  local dir="${WORKDIR}/incremental-agents"
  make_project "${dir}"

  echo ""
  echo "▶ test_incremental_agent_additions"

  # Step 1: install linting
  ballast "${dir}" install --target cursor --agent linting --yes
  assert_agents "${dir}" "linting" "git-hooks"
  assert_doctor_agents "${dir}" "linting" "git-hooks"
  pass "step 1: linting installed"

  # Step 2: add testing — linting must be retained
  ballast "${dir}" install --target cursor --agent testing --yes
  assert_agents "${dir}" "linting" "git-hooks" "testing"
  assert_doctor_agents "${dir}" "linting" "git-hooks" "testing"
  pass "step 2: testing added; linting retained"

  # Step 3: add logging — linting and testing must be retained
  ballast "${dir}" install --target cursor --agent logging --yes
  assert_agents "${dir}" "linting" "git-hooks" "testing" "logging"
  assert_doctor_agents "${dir}" "linting" "git-hooks" "testing" "logging"
  pass "step 3: logging added; linting and testing retained"

  echo "  ✓ test_incremental_agent_additions passed"
}

# ─── test: adding all agents ─────────────────────────────────────────────────

test_add_all_agents() {
  local dir="${WORKDIR}/all-agents"
  make_project "${dir}"

  echo ""
  echo "▶ test_add_all_agents"

  # Start with a single agent so there is prior state
  ballast "${dir}" install --target cursor --agent linting --yes
  assert_agents "${dir}" "linting"
  pass "step 1: linting installed as initial state"

  # Install --all: every agent must be present
  ballast "${dir}" install --target cursor --all --yes
  for agent in linting git-hooks testing logging cicd local-dev observability publishing docs; do
    assert_agents "${dir}" "${agent}"
  done
  assert_doctor_agents "${dir}" "linting"
  pass "step 2: --all installs every agent"

  echo "  ✓ test_add_all_agents passed"
}

# ─── test: incremental skill additions ───────────────────────────────────────

test_incremental_skill_additions() {
  local dir="${WORKDIR}/incremental-skills"
  make_project "${dir}"

  echo ""
  echo "▶ test_incremental_skill_additions"

  # Establish agents first
  ballast "${dir}" install --target cursor --agent linting --yes

  # Step 1: install first skill — agents must be retained
  ballast "${dir}" install --target cursor --skill owasp-security-scan --yes
  assert_agents "${dir}" "linting" "git-hooks"
  assert_skills "${dir}" "owasp-security-scan"
  assert_doctor_agents "${dir}" "linting" "git-hooks"
  assert_doctor_skills "${dir}" "owasp-security-scan"
  pass "step 1: owasp-security-scan installed; agents retained"

  # Step 2: add a second skill — first skill and agents must be retained
  ballast "${dir}" install --target cursor --skill aws-health-review --yes
  assert_agents "${dir}" "linting" "git-hooks"
  assert_skills "${dir}" "owasp-security-scan" "aws-health-review"
  assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
  pass "step 2: aws-health-review added; owasp-security-scan and agents retained"

  echo "  ✓ test_incremental_skill_additions passed"
}

# ─── test: adding all skills ─────────────────────────────────────────────────

test_add_all_skills() {
  local dir="${WORKDIR}/all-skills"
  make_project "${dir}"

  echo ""
  echo "▶ test_add_all_skills"

  # Establish agents and one skill
  ballast "${dir}" install --target cursor --agent linting --yes
  ballast "${dir}" install --target cursor --skill owasp-security-scan --yes

  # Install --all-skills: all skills must be present and agents must be retained
  ballast "${dir}" install --target cursor --all-skills --yes
  assert_agents "${dir}" "linting" "git-hooks"
  for skill in owasp-security-scan aws-health-review aws-live-health-review aws-weekly-security-review; do
    assert_skills "${dir}" "${skill}"
  done
  assert_doctor_agents "${dir}" "linting"
  assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
  pass "step 1: --all-skills installs every skill; agents retained"

  echo "  ✓ test_add_all_skills passed"
}

# ─── test: agent install does not drop skills ─────────────────────────────────

test_agent_install_retains_skills() {
  local dir="${WORKDIR}/agent-retains-skills"
  make_project "${dir}"

  echo ""
  echo "▶ test_agent_install_retains_skills"

  # Establish agents and skills
  ballast "${dir}" install --target cursor --agent linting --yes
  ballast "${dir}" install --target cursor --skill owasp-security-scan --yes
  assert_skills "${dir}" "owasp-security-scan"

  # Add an agent without a skill flag — the skill must survive
  ballast "${dir}" install --target cursor --agent testing --yes
  assert_agents "${dir}" "linting" "git-hooks" "testing"
  assert_skills "${dir}" "owasp-security-scan"
  assert_doctor_skills "${dir}" "owasp-security-scan"
  pass "installing an agent does not drop existing skills"

  echo "  ✓ test_agent_install_retains_skills passed"
}

# ─── test: skill install does not drop agents ─────────────────────────────────

test_skill_install_retains_agents() {
  local dir="${WORKDIR}/skill-retains-agents"
  make_project "${dir}"

  echo ""
  echo "▶ test_skill_install_retains_agents"

  # Establish multiple agents
  ballast "${dir}" install --target cursor --agent linting --yes
  ballast "${dir}" install --target cursor --agent testing --yes

  # Add a skill without an agent flag — agents must survive
  ballast "${dir}" install --target cursor --skill owasp-security-scan --yes
  assert_agents "${dir}" "linting" "git-hooks" "testing"
  assert_skills "${dir}" "owasp-security-scan"
  assert_doctor_agents "${dir}" "linting" "git-hooks" "testing"
  pass "installing a skill does not drop existing agents"

  echo "  ✓ test_skill_install_retains_agents passed"
}

# ─── test: reinstalling a subset of agents does not drop the rest ────────────
# This is the "removing 1 agent" scenario: specifying fewer agents in a
# subsequent install must not drop the previously configured agents.

test_reinstall_subset_agents_retained() {
  local dir="${WORKDIR}/subset-agents"
  make_project "${dir}"

  echo ""
  echo "▶ test_reinstall_subset_agents_retained"

  # Install multiple agents
  ballast "${dir}" install --target cursor --agent linting --yes
  ballast "${dir}" install --target cursor --agent testing --yes
  ballast "${dir}" install --target cursor --agent logging --yes
  assert_agents "${dir}" "linting" "git-hooks" "testing" "logging"
  pass "step 1: linting, testing, logging installed"

  # Reinstall specifying only linting — testing and logging must NOT be dropped
  ballast "${dir}" install --target cursor --agent linting --yes
  assert_agents "${dir}" "linting" "git-hooks" "testing" "logging"
  assert_doctor_agents "${dir}" "linting" "testing" "logging"
  pass "step 2: reinstalling linting does not drop testing or logging"

  echo "  ✓ test_reinstall_subset_agents_retained passed"
}

# ─── test: reinstalling a subset of skills does not drop the rest ────────────
# This is the "removing 1 skill" scenario.

test_reinstall_subset_skills_retained() {
  local dir="${WORKDIR}/subset-skills"
  make_project "${dir}"

  echo ""
  echo "▶ test_reinstall_subset_skills_retained"

  # Install multiple skills
  ballast "${dir}" install --target cursor --agent linting --yes
  ballast "${dir}" install --target cursor --skill owasp-security-scan --yes
  ballast "${dir}" install --target cursor --skill aws-health-review --yes
  assert_skills "${dir}" "owasp-security-scan" "aws-health-review"
  pass "step 1: owasp-security-scan and aws-health-review installed"

  # Reinstall specifying only owasp — aws-health-review must NOT be dropped
  ballast "${dir}" install --target cursor --skill owasp-security-scan --yes
  assert_agents "${dir}" "linting" "git-hooks"
  assert_skills "${dir}" "owasp-security-scan" "aws-health-review"
  assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
  pass "step 2: reinstalling owasp does not drop aws-health-review"

  echo "  ✓ test_reinstall_subset_skills_retained passed"
}

# ─── test: full sequential scenario ──────────────────────────────────────────
# End-to-end: add all agents → add all skills → reinstall subset of agents →
# reinstall subset of skills.  At every step verify prior state is intact.

test_full_sequential() {
  local dir="${WORKDIR}/full-sequential"
  make_project "${dir}"

  echo ""
  echo "▶ test_full_sequential"

  # 1. Add all agents
  ballast "${dir}" install --target cursor --all --yes
  for agent in linting git-hooks testing logging cicd local-dev observability publishing docs; do
    assert_agents "${dir}" "${agent}"
  done
  pass "step 1: --all agents installed"

  # 2. Add all skills — all agents must remain
  ballast "${dir}" install --target cursor --all-skills --yes
  for agent in linting git-hooks testing logging; do
    assert_agents "${dir}" "${agent}"
  done
  for skill in owasp-security-scan aws-health-review aws-live-health-review aws-weekly-security-review; do
    assert_skills "${dir}" "${skill}"
  done
  assert_doctor_agents "${dir}" "linting" "testing"
  assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
  pass "step 2: --all-skills added; all agents retained"

  # 3. Reinstall 1 agent only — all other agents and all skills must remain
  ballast "${dir}" install --target cursor --agent linting --yes
  for agent in linting git-hooks testing logging cicd local-dev; do
    assert_agents "${dir}" "${agent}"
  done
  for skill in owasp-security-scan aws-health-review; do
    assert_skills "${dir}" "${skill}"
  done
  pass "step 3: reinstall linting only; all other agents and skills retained"

  # 4. Reinstall 1 skill only — all agents and other skills must remain
  ballast "${dir}" install --target cursor --skill owasp-security-scan --yes
  for agent in linting git-hooks testing logging; do
    assert_agents "${dir}" "${agent}"
  done
  for skill in owasp-security-scan aws-health-review aws-live-health-review aws-weekly-security-review; do
    assert_skills "${dir}" "${skill}"
  done
  assert_doctor_agents "${dir}" "linting" "testing"
  assert_doctor_skills "${dir}" "owasp-security-scan" "aws-health-review"
  pass "step 4: reinstall owasp only; all agents and other skills retained"

  echo "  ✓ test_full_sequential passed"
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
    echo "${output}" | grep -q "${agent}" || \
      fail "ballast doctor did not report agent '${agent}'. Output:\n${output}"
  done
}

assert_ballast_doctor_skills() {
  local dir="$1"; shift
  local output
  output="$(cd "${dir}" && "${BALLAST_CLI}" doctor 2>&1)"
  for skill in "$@"; do
    echo "${output}" | grep -q "${skill}" || \
      fail "ballast doctor did not report skill '${skill}'. Output:\n${output}"
  done
}

test_ballast_cli_skill_retains_agents() {
  if [ ! -x "${BALLAST_CLI}" ]; then
    echo "  SKIP: ballast binary not found at ${BALLAST_CLI}"
    return
  fi

  local dir="${WORKDIR}/ballast-cli-skill-retains-agents"
  make_project "${dir}"

  echo ""
  echo "▶ test_ballast_cli_skill_retains_agents"

  ballast_cli "${dir}" install --target cursor --agent linting --yes
  assert_agents "${dir}" "linting" "git-hooks"
  assert_no_skills "${dir}" "owasp-security-scan"
  pass "step 1: linting installed via ballast CLI"

  ballast_cli "${dir}" install --target cursor --skill owasp-security-scan --yes
  assert_agents "${dir}" "linting" "git-hooks"
  assert_skills "${dir}" "owasp-security-scan"
  assert_ballast_doctor_agents "${dir}" "linting" "git-hooks"
  assert_ballast_doctor_skills "${dir}" "owasp-security-scan"
  pass "step 2: skill added via ballast CLI; agents preserved"

  echo "  ✓ test_ballast_cli_skill_retains_agents passed"
}

test_ballast_cli_agent_retains_skills() {
  if [ ! -x "${BALLAST_CLI}" ]; then
    echo "  SKIP: ballast binary not found at ${BALLAST_CLI}"
    return
  fi

  local dir="${WORKDIR}/ballast-cli-agent-retains-skills"
  make_project "${dir}"

  echo ""
  echo "▶ test_ballast_cli_agent_retains_skills"

  ballast_cli "${dir}" install --target cursor --skill owasp-security-scan --yes
  assert_skills "${dir}" "owasp-security-scan"
  pass "step 1: skill installed via ballast CLI"

  ballast_cli "${dir}" install --target cursor --agent linting --yes
  assert_skills "${dir}" "owasp-security-scan"
  assert_agents "${dir}" "linting" "git-hooks"
  assert_ballast_doctor_agents "${dir}" "linting" "git-hooks"
  assert_ballast_doctor_skills "${dir}" "owasp-security-scan"
  pass "step 2: agent added via ballast CLI; skills preserved"

  echo "  ✓ test_ballast_cli_agent_retains_skills passed"
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
