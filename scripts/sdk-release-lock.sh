#!/usr/bin/env bash

acquire_sdk_source_lock() {
  local lock_path="$1" owner_pid="$2" holder quarantine
  while ! mkdir "$lock_path" 2>/dev/null; do
    holder="$(cat "$lock_path/owner" 2>/dev/null || true)"
    if { [[ "$holder" =~ ^[0-9]+$ ]] && ! kill -0 "$holder" 2>/dev/null; } ||
       { [[ -z "$holder" ]] && [[ -n "$(find "$lock_path" -maxdepth 0 -mmin +1 -print 2>/dev/null)" ]]; }; then
      quarantine="${lock_path}.stale.$$"
      if mv "$lock_path" "$quarantine" 2>/dev/null; then rm -rf "$quarantine"; fi
      continue
    fi
    sleep 0.2
  done
  printf '%s\n' "$owner_pid" > "$lock_path/owner.tmp"
  mv "$lock_path/owner.tmp" "$lock_path/owner"
}

release_sdk_source_lock() {
  local lock_path="$1" owner_pid="$2"
  if [[ "$(cat "$lock_path/owner" 2>/dev/null || true)" == "$owner_pid" ]]; then rm -rf "$lock_path"; fi
}
