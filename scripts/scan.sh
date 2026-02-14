#!/bin/sh
set -eu

read_tmux_option() {
  tmux show-option -gqv "$1"
}

init_paths() {
  data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
  data_dir="$data_home/cc-flavors"
  panes_file="$data_dir/panes"
  raw_log="$data_dir/raw.log"
  pid_file="$data_dir/scan.pid"
}

ensure_data_dir() {
  mkdir -p "$data_dir"
  touch "$panes_file" "$raw_log"
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
    scan_interval=5
  fi
}

get_cmd() {
  cmd="$(read_tmux_option @cc_flavors_cmd)"
  if [ -z "$cmd" ]; then
    cmd="claude"
  fi
}

attach_pipes() {
  tmp_new="$(mktemp)"
  # List all panes: id command pipe
  tmux list-panes -a -F "#{pane_id} #{pane_current_command}" |
    while read -r pane_id pane_cmd; do
      if [ "$pane_cmd" = "$cmd" ]; then
        tmux pipe-pane -o -t "$pane_id" "cc-flavors ingest"
        printf "%s\n" "$pane_id" >>"$tmp_new"
      fi
    done
}

detach_missing_panes() {
  if [ -s "$panes_file" ]; then
    while read -r pane_id; do
      if ! grep -Fqx "$pane_id" "$tmp_new"; then
        tmux pipe-pane -t "$pane_id" >/dev/null 2>&1 || true
      fi
    done <"$panes_file"
  fi
}

commit_panes() {
  sort -u "$tmp_new" >"$panes_file"
  rm -f "$tmp_new"
}

scan_once() {
  get_cmd
  attach_pipes
  detach_missing_panes
  commit_panes
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
