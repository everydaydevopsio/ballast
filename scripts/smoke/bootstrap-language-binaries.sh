#!/usr/bin/env bash
set -euo pipefail

MODE="from-local"
SOURCE_ROOT=""
ONLY=""
INSTALL_ROOT="/usr/local/lib/ballast/bin"
BIN_ROOT="/usr/local/bin"
REMOTE_REPO_URL="${BALLAST_REMOTE_REPO_URL:-https://github.com/everydaydevopsio/ballast.git}"
REMOTE_REF="${BALLAST_REMOTE_REF:-main}"
REMOTE_CLONE_DIR="/usr/local/lib/ballast/sources/ballast"

usage() {
  cat <<USAGE
Usage: bootstrap-language-binaries [--from-local <path> | --from-remote] [--only <typescript|python|go>]

Options:
  --from-local <path>  Install language CLIs from a local ballast checkout.
  --from-remote        Clone ballast from GitHub and install CLIs from that source.
  --only <language>    Install only one language CLI.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from-local)
      MODE="from-local"
      SOURCE_ROOT="${2:-}"
      shift 2
      ;;
    --from-remote)
      MODE="from-remote"
      shift
      ;;
    --only)
      ONLY="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -n "$ONLY" && "$ONLY" != "typescript" && "$ONLY" != "python" && "$ONLY" != "go" ]]; then
  echo "Invalid --only value: $ONLY" >&2
  exit 1
fi

mkdir -p "$INSTALL_ROOT" "$BIN_ROOT"

prepare_source() {
  if [[ "$MODE" == "from-local" ]]; then
    if [[ -z "$SOURCE_ROOT" ]]; then
      echo "--from-local requires a path" >&2
      exit 1
    fi
    if [[ ! -d "$SOURCE_ROOT" ]]; then
      echo "Local source path not found: $SOURCE_ROOT" >&2
      exit 1
    fi
    echo "$SOURCE_ROOT"
    return
  fi

  mkdir -p "$(dirname "$REMOTE_CLONE_DIR")"
  if [[ -d "$REMOTE_CLONE_DIR/.git" ]]; then
    git -C "$REMOTE_CLONE_DIR" fetch --depth 1 origin "$REMOTE_REF"
    git -C "$REMOTE_CLONE_DIR" checkout --force FETCH_HEAD
  else
    git clone --depth 1 --branch "$REMOTE_REF" "$REMOTE_REPO_URL" "$REMOTE_CLONE_DIR"
  fi
  echo "$REMOTE_CLONE_DIR"
}

install_typescript() {
  local source_root="$1"
  local target="$INSTALL_ROOT/ballast-typescript"

  if [[ -x "$target" ]]; then
    return
  fi

  CI=true pnpm --dir "$source_root" install --frozen-lockfile
  CI=true pnpm --dir "$source_root" --filter @everydaydevopsio/ballast run build

  cat > "$target" <<SCRIPT
#!/usr/bin/env bash
set -euo pipefail
node "$source_root/packages/ballast-typescript/dist/cli.js" "\$@"
SCRIPT
  chmod +x "$target"
}

install_python() {
  local source_root="$1"
  local target="$INSTALL_ROOT/ballast-python"

  if [[ -x "$target" ]]; then
    return
  fi

  cat > "$target" <<SCRIPT
#!/usr/bin/env bash
set -euo pipefail
PYTHONPATH="$source_root/packages/ballast-python:\${PYTHONPATH:-}" \\
  python3 -c "from ballast.cli import main; import sys; raise SystemExit(main(sys.argv[1:]))" "\$@"
SCRIPT
  chmod +x "$target"
}

install_go() {
  local source_root="$1"
  local target="$INSTALL_ROOT/ballast-go"
  local module_root="$source_root/packages/ballast-go"
  local cmd_path="./cmd/ballast-go"
  local go_bin

  go_bin="$(command -v go || true)"
  if [[ -z "$go_bin" && -x /usr/local/go/bin/go ]]; then
    go_bin="/usr/local/go/bin/go"
  fi
  if [[ ! -d "$module_root/cmd/ballast-go" && -d "$module_root/cmd/ballast" ]]; then
    cmd_path="./cmd/ballast"
  fi

  if [[ -x "$target" ]]; then
    return
  fi
  if [[ -z "$go_bin" ]]; then
    echo "go binary not found on PATH and /usr/local/go/bin/go is missing" >&2
    exit 1
  fi

  "$go_bin" build -C "$module_root" -o "$target" "$cmd_path"
  chmod +x "$target"
}

link_binary() {
  local name="$1"
  ln -sf "$INSTALL_ROOT/$name" "$BIN_ROOT/$name"
}

SOURCE_PATH="$(prepare_source)"

if [[ -n "$ONLY" ]]; then
  case "$ONLY" in
    typescript)
      install_typescript "$SOURCE_PATH"
      link_binary ballast-typescript
      ;;
    python)
      install_python "$SOURCE_PATH"
      link_binary ballast-python
      ;;
    go)
      install_go "$SOURCE_PATH"
      link_binary ballast-go
      ;;
  esac
  exit 0
fi

install_typescript "$SOURCE_PATH"
install_python "$SOURCE_PATH"
install_go "$SOURCE_PATH"

link_binary ballast-typescript
link_binary ballast-python
link_binary ballast-go
