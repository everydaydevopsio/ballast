#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_root="$(cd "${script_dir}/.." && pwd)"
repo_root="$(cd "${package_root}/../.." && pwd)"

source_agents="${repo_root}/agents"
source_skills="${repo_root}/skills"

if [[ ! -d "${source_agents}" ]]; then
  echo "Missing source agents directory: ${source_agents}" >&2
  exit 1
fi

if [[ ! -d "${source_skills}" ]]; then
  echo "Missing source skills directory: ${source_skills}" >&2
  exit 1
fi

copy_tree() {
  local src="$1"
  local dest="$2"

  mkdir -p "${dest}"
  find "${dest}" -mindepth 1 -delete

  (
    cd "${src}"
    find . -type d -exec mkdir -p "${dest}/{}" \;
    while IFS= read -r -d '' rel; do
      local clean_rel="${rel#./}"
      mkdir -p "${dest}/$(dirname "${clean_rel}")"
      cat "${clean_rel}" > "${dest}/${clean_rel}"
    done < <(find . -type f -print0)
  )
}

copy_tree "${source_agents}" "${package_root}/agents"
copy_tree "${source_skills}" "${package_root}/skills"
copy_tree "${source_agents}" "${repo_root}/packages/ballast-python/ballast/agents"
copy_tree "${source_skills}" "${repo_root}/packages/ballast-python/ballast/skills"
copy_tree "${source_agents}" "${repo_root}/packages/ballast-go/cmd/ballast-go/agents"
copy_tree "${source_skills}" "${repo_root}/packages/ballast-go/cmd/ballast-go/skills"
copy_tree "${source_agents}" "${repo_root}/packages/ballast-go/cmd/ballast/agents"
copy_tree "${source_skills}" "${repo_root}/packages/ballast-go/cmd/ballast/skills"
