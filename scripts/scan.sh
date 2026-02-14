#!/bin/sh
set -eu

read_tmux_option() {
  tmux show-option -gqv "$1"
}

init_paths() {
  data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
  data_dir="$data_home/cc-flavors"
  pid_file="$data_dir/scan.pid"
}

ensure_data_dir() {
  mkdir -p "$data_dir"
}

acquire_lock() {
  if [ -f "$pid_file" ]; then
    old_pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [ -n "$old_pid" ] && kill -0 "$old_pid" 2>/dev/null; then
      exit 0
    fi
  fi

  echo "$$" >"$pid_file"
  trap 'rm -f "$pid_file"' EXIT INT TERM
}

get_scan_interval() {
  scan_interval="$(read_tmux_option @cc_flavors_scan_interval)"
  if [ -z "$scan_interval" ]; then
    scan_interval=1
  fi
}

get_cmd() {
  cmd="$(read_tmux_option @cc_flavors_cmd)"
  if [ -z "$cmd" ]; then
    cmd="claude"
  fi
}

resolve_cc_flavors() {
  cc_flavors="$(command -v cc-flavors 2>/dev/null || true)"
  if [ -z "$cc_flavors" ] && [ -n "${HOME:-}" ]; then
    if [ -x "$HOME/go/bin/cc-flavors" ]; then
      cc_flavors="$HOME/go/bin/cc-flavors"
    elif [ -x "$HOME/.local/go/bin/cc-flavors" ]; then
      cc_flavors="$HOME/.local/go/bin/cc-flavors"
    fi
  fi
}

capture_flavor() {
  pane_id="$1"
  pane_key="${pane_id#%}"
  last_file="$data_dir/last_${pane_key}"

  text="$(tmux capture-pane -p -t "$pane_id" -S -100)"
  match="$(printf "%s\n" "$text" | tr '\r' '\n' | sed -E 's/\x1b\[[0-9;?]*[a-zA-Z]//g' | grep -Eo '[A-Z][A-Za-z]*ing…' | tail -n 1)"
  if [ -z "$match" ]; then
    return 0
  fi

  last=""
  if [ -f "$last_file" ]; then
    last="$(cat "$last_file" 2>/dev/null || true)"
  fi

  if [ "$match" != "$last" ]; then
    printf "%s\n" "$match" | "$cc_flavors" ingest
    printf "%s" "$match" >"$last_file"
  fi
}

scan_once() {
  get_cmd
  resolve_cc_flavors
  if [ -z "${cc_flavors:-}" ]; then
    tmux display-message "cc-flavors: command not found"
    return 0
  fi

  tmux list-panes -a -F "#{pane_id} #{pane_current_command}" |
    while read -r pane_id pane_cmd; do
      if [ "$pane_cmd" = "$cmd" ]; then
        capture_flavor "$pane_id"
      fi
    done
}

main() {
  init_paths
  ensure_data_dir
  acquire_lock
  get_scan_interval

  while :; do
    scan_once
    sleep "$scan_interval"
  done
}

main
