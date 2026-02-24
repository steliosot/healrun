#!/usr/bin/env bash

# 25-command macOS test harness for healrun.
# - Runs commands sequentially
# - Expects healrun to auto-fix (uses --auto-approve)
# - Continues even if some commands fail
# - Prints PASS/FIXED/FAILED summary
# - Cleans up local artifacts by running inside a temp directory
# - Does NOT uninstall Homebrew formulas by default

set -u

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." >/dev/null 2>&1 && pwd)"
HEALRUN_BIN="${HEALRUN_BIN:-${ROOT_DIR}/healrun}"
AUTO_APPROVE_FLAG="${AUTO_APPROVE_FLAG:---auto-approve}"
DEBUG_FLAG="${DEBUG_FLAG:-}"

if [[ ! -x "$HEALRUN_BIN" ]]; then
  echo "healrun binary not found/executable: $HEALRUN_BIN" >&2
  exit 2
fi

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t healrun-tests)"

cleanup() {
  rm -rf "$tmpdir" >/dev/null 2>&1 || true
}
trap cleanup EXIT

before_formulas="$tmpdir/brew.before"
after_formulas="$tmpdir/brew.after"

if command -v brew >/dev/null 2>&1; then
  brew list --formula 2>/dev/null | sort > "$before_formulas" || true
fi

CMDS=(
  "wget https://example.com"
  "jq . <<< '{\"a\":1}'"
  "tree"
  "htop"
  "python3 -c 'import yaml'"
  "python3 -c 'import requests'"
  "python3 -c 'import numpy'"
  "python3 -c 'import lxml'"
  "python3 -c 'import bs4'"
  "node -e 'require(\"axios\")'"
  "node -e 'require(\"left-pad\")'"
  "npm --version"
  "npm install left-pad"
  "git --version"
  "curl --version"
  "sqlite3 --version"
  "ffmpeg -version"
  "convert --version"
  "gs --version"
  "watch ls"
  "parallel echo ::: 1 2 3"
  "rg --version"
  "fd --version"
  "python3 -m pip --version"
  "python3 -c 'import ssl; print(ssl.OPENSSL_VERSION)'"
)

pass=0
fixed=0
failed=0

FAILED_CMDS=()
FAILED_REASONS=()

extract_reason() {
  local file="$1"
  local reason
  reason="$(grep -E "Detected: |repair stopped|max retries|no fixes suggested|no fixes could be applied|command failed with exit code|Error getting fix suggestions" "$file" 2>/dev/null | tail -1)"
  if [[ -z "$reason" ]]; then
    reason="$(tail -3 "$file" 2>/dev/null | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g')"
  fi
  echo "$reason"
}

is_fixed_run() {
  local file="$1"
  if grep -q "Success after repair" "$file" 2>/dev/null; then
    return 0
  fi
  if grep -q "Applying fixes" "$file" 2>/dev/null; then
    return 0
  fi
  if grep -q "Fix applied" "$file" 2>/dev/null; then
    return 0
  fi
  return 1
}

echo "[healrun-25] Workdir: $tmpdir"
echo "[healrun-25] Running ${#CMDS[@]} commands"
echo

set +e
for cmd in "${CMDS[@]}"; do
  echo "[healrun-25] >>> $cmd"
  outfile="$tmpdir/out.$(date +%s%N).log"

  # Run in temp dir so local artifacts (node_modules/.healrun-venv/index.html) are cleaned.
  (cd "$tmpdir" && "$HEALRUN_BIN" $AUTO_APPROVE_FLAG $DEBUG_FLAG "$cmd" 2>&1 | tee "$outfile")
  rc=${PIPESTATUS[0]}

  if [[ $rc -eq 0 ]]; then
    if is_fixed_run "$outfile"; then
      fixed=$((fixed + 1))
      echo "[healrun-25] FIXED"
    else
      pass=$((pass + 1))
      echo "[healrun-25] PASS"
    fi
  else
    failed=$((failed + 1))
    FAILED_CMDS+=("$cmd")
    FAILED_REASONS+=("$(extract_reason "$outfile")")
    echo "[healrun-25] FAILED (exit $rc)"
  fi
  echo
done
set -e

echo "[healrun-25] Final report"
echo "PASS:  $pass"
echo "FIXED: $fixed"
echo "FAILED: $failed"

if [[ $failed -gt 0 ]]; then
  echo
  echo "[healrun-25] Failures"
  for i in "${!FAILED_CMDS[@]}"; do
    echo "- ${FAILED_CMDS[$i]}"
    echo "  ${FAILED_REASONS[$i]}"
  done
fi

if command -v brew >/dev/null 2>&1; then
  brew list --formula 2>/dev/null | sort > "$after_formulas" || true
  if [[ -s "$before_formulas" && -s "$after_formulas" ]]; then
    newly="$tmpdir/brew.delta"
    comm -13 "$before_formulas" "$after_formulas" > "$newly" || true
    if [[ -s "$newly" ]]; then
      echo
      echo "[healrun-25] Homebrew formulas installed during this run:"
      cat "$newly" | sed 's/^/- /'
      echo
      echo "[healrun-25] Cleanup (optional, run manually if you want):"
      echo "brew uninstall $(tr '\n' ' ' < "$newly")"
    fi
  fi
fi

echo
echo "[healrun-25] Local artifacts were isolated to: $tmpdir (auto-deleted at end)"

exit 0
