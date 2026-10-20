#!/usr/bin/env bash
# Repository policy checks: floating dependency versions, unpinned GitHub
# Actions, floating container image tags and high-signal secret patterns.
#
# SCOPE AND LIMITATIONS (read before trusting a PASS):
#   These are narrow, regex-based heuristics. They catch common mistakes; they
#   are NOT exhaustive security assurance and do not replace dependency
#   auditing, secret scanning services or human review. A clean run means these
#   specific patterns were not found, not that the repository is secure.
#
# Usage: scripts/policy-check.sh   (exit 0 = pass, 1 = violation)
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

fail=0

echo "== floating dependency versions =="
# Ranges, dist-tags and non-exact specifiers.
if grep -nE '"[^"]+"[[:space:]]*:[[:space:]]*"(latest|stable|next|canary|\*|x|X|~|\^|>=|<=|>|<)' ui/package.json; then
  echo "::error::floating version specifier found in ui/package.json"
  fail=1
fi
# Wildcards such as 1.x, 1.2.x or 1.2.* (exact versions must not match).
if grep -nE '"[^"]+"[[:space:]]*:[[:space:]]*"[0-9]+(\.[0-9xX*]+)*\.[xX*][0-9xX*]*"' ui/package.json; then
  echo "::error::non-exact (wildcard) version found in ui/package.json"
  fail=1
fi
# Bare-major versions such as "1" or "24" are not exact pins.
if grep -nE '"[^"]+"[[:space:]]*:[[:space:]]*"[0-9]+"' ui/package.json; then
  echo "::error::bare-major version found in ui/package.json"
  fail=1
fi
if grep -nE '^\s*go\s+[0-9]+\.[0-9]+$' backend/go.mod; then
  echo "::error::go directive must pin the patch version"
  fail=1
fi

echo "== unpinned GitHub Actions =="
# Every third-party action must be pinned to a full 40-hex commit SHA.
unpinned="$(grep -rInE 'uses:[[:space:]]*[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@' .github/workflows \
  | grep -vE '@[0-9a-f]{40}' || true)"
if [ -n "$unpinned" ]; then
  echo "$unpinned"
  echo "::error::action not pinned to a full commit SHA"
  fail=1
fi

echo "== floating container image tags =="
matches="$(grep -rInE 'image:[[:space:]]*[^[:space:]]+:latest([[:space:]]|$)' \
  --include='*.yml' --include='*.yaml' . | grep -v 'grep ' || true)"
if [ -n "$matches" ]; then
  echo "$matches"
  echo "::error::floating container image tag (:latest) found"
  fail=1
fi

echo "== committed secret material (high-signal patterns) =="
# High-signal patterns only; broad keyword matching would false-positive on
# documentation that merely discusses secrets.
secrets="$(grep -rInE \
  'BEGIN [A-Z ]*PRIVATE KEY|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}' \
  --include='*.go' --include='*.ts' --include='*.tsx' --include='*.js' \
  --include='*.json' --include='*.yml' --include='*.yaml' \
  --include='*.sh' --include='*.toml' --include='*.env' \
  . || true)"
if [ -n "$secrets" ]; then
  echo "$secrets"
  echo "::error::possible secret material committed"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "policy-check: PASS (limited regex checks only)"
else
  echo "policy-check: FAIL"
fi
exit "$fail"
