#!/usr/bin/env bash
# Enforce Conventional Commits on the first line of the commit message.
# Pass the path to the commit-msg file as $1 (lefthook substitutes {1}).
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "check-commit-msg: expected commit-msg path as argument" >&2
  exit 1
fi

msg_file="$1"
first_line="$(head -n 1 "$msg_file")"

# Allow auto-generated merge / revert commits to bypass the rule.
if [[ "$first_line" =~ ^(Merge|Revert)\  ]]; then
  exit 0
fi

# Conventional Commits: type(scope)?!?: subject (subject 1..72 chars).
pattern='^(feat|fix|chore|refactor|docs|test|build|ci|perf|revert)(\([a-z0-9/_-]+\))?(!)?: .{1,72}$'

if [[ ! "$first_line" =~ $pattern ]]; then
  cat >&2 <<'SPEC'
Rejected: commit subject does not match Conventional Commits.

Expected:  <type>(<scope>)?: <subject>
Types:     feat fix chore refactor docs test build ci perf revert
Scope:     lowercase, alphanum, slash, underscore, hyphen (optional)
Breaking:  add `!` after type/scope, e.g. feat(api)!: drop v1 endpoint
Subject:   1..72 chars, imperative, no trailing period

Examples:
  feat(identity): add JWT verifier
  fix(pkg/postgres): rollback on panic
  revert(pkg/otel): undo provider swap
  chore!: bump go to 1.26
SPEC
  echo >&2
  echo "Got: $first_line" >&2
  exit 1
fi
