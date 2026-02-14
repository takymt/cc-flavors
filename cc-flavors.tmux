#!/bin/sh
set -eu

tmux set-option -gqo @cc_flavors_cmd "claude"
tmux set-option -gqo @cc_flavors_hook_index "99"
tmux set-option -gqo @cc_flavors_scan_interval "5"

script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
if ! command -v cc-flavors >/dev/null 2>&1; then
  tmux display-message "cc-flavors: command not found"
  exit 0
fi

hook_index="$(tmux show-option -gqv @cc_flavors_hook_index)"
if [ -z "$hook_index" ]; then
  hook_index=99
fi

tmux set-hook -g "client-attached[$hook_index]" \
  "run-shell '$script_dir/scripts/scan.sh'"
