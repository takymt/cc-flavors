#!/bin/sh
set -eu

msg_file=${1:-}
if [ -z "$msg_file" ] || [ ! -f "$msg_file" ]; then
  echo "commit-msg hook: message file not found" >&2
  exit 1
fi

first_line=$(head -n 1 "$msg_file" | tr -d '\r')

case "$first_line" in
  Merge*|Revert*)
    exit 0
    ;;
esac

types="feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert"
pattern="^(${types})(\([^)]+\))?(!)?: .+"

if ! printf '%s' "$first_line" | grep -Eq "$pattern"; then
  cat >&2 <<'EOF'
Invalid commit message. Use Conventional Commits:
  <type>(<scope>)!: <subject>
Examples:
  feat: add ingest command
  fix(db): handle close error
  chore!: drop legacy flag
EOF
  exit 1
fi

exit 0
