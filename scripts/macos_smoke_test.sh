#!/usr/bin/env bash

# macOS smoke test harness for healrun.
# Runs a sequence of commands and reports PASS/FIXED/FAILED.

set -u

HEALRUN_BIN="${HEALRUN_BIN:-./healrun}"

# In a non-interactive harness, auto-approve is required.
AUTO_APPROVE_FLAG="${AUTO_APPROVE_FLAG:---auto-approve}"

if [[ ! -x "$HEALRUN_BIN" ]]; then
  echo "healrun binary not found/executable: $HEALRUN_BIN" >&2
  echo "Hint: cd healrun && go build -o healrun ./cmd/healrun" >&2
  exit 2
fi

CMDS=(
  "wget https://example.com"
  "jq . <<< '{\"a\":1}'"
  "tree"
  "htop"
  "python3 -c 'import yaml'"
  "python3 -c 'import requests'"
  "python3 -c 'import numpy'"
  "node -e 'require(\"axios\")'"
  "npm install left-pad"
  "git --version"
  "curl --version"
  "sqlite3 --version"
  "ffmpeg -version"
  "convert --version"
  "gs --version"
  "watch ls"
  "parallel echo ::: 1 2 3"
  "rg test"
  "fd ."
  "python3 -c 'import lxml'"
)

pass=0
fixed=0
failed=0

FAILED_CMDS=()
FAILED_REASONS=()

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t healrun-smoke)"
cleanup() {
  rm -rf "$tmpdir" >/dev/null 2>&1 || true
}
trap cleanup EXIT

extract_reason() {
  local file="$1"
  local reason
  reason="$(grep -E "repair stopped|max retries|no fixes suggested|no fixes could be applied|command failed with exit code|error getting fix suggestions" "$file" 2>/dev/null | tail -1)"
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

echo "[healrun-smoke] Running ${#CMDS[@]} commands"
echo

for cmd in "${CMDS[@]}"; do
  echo "[healrun-smoke] >>> $cmd"

  outfile="$tmpdir/out.$(date +%s%N).log"

  # Run healrun and tee output. Capture healrun exit status via PIPESTATUS.
  set +e
  "$HEALRUN_BIN" $AUTO_APPROVE_FLAG "$cmd" 2>&1 | tee "$outfile"
  rc=${PIPESTATUS[0]}
  set -e

  if [[ $rc -eq 0 ]]; then
    if is_fixed_run "$outfile"; then
      fixed=$((fixed + 1))
      echo "[healrun-smoke] FIXED"
    else
      pass=$((pass + 1))
      echo "[healrun-smoke] PASS"
    fi
  else
    failed=$((failed + 1))
    FAILED_CMDS+=("$cmd")
    FAILED_REASONS+=("$(extract_reason "$outfile")")
    echo "[healrun-smoke] FAILED (exit $rc)"
  fi

  echo
done

echo "[healrun-smoke] Final report"
echo "PASS:  $pass"
echo "FIXED: $fixed"
echo "FAILED: $failed"

if [[ $failed -gt 0 ]]; then
  echo
  echo "[healrun-smoke] Failures"
  for i in "${!FAILED_CMDS[@]}"; do
    echo "- ${FAILED_CMDS[$i]}"
    echo "  ${FAILED_REASONS[$i]}"
  done
fi

exit 0
