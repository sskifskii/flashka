#!/bin/sh
# This is a lightweight pattern check, not a full credential audit.
set -eu

# Print filenames only: a matching secret must never appear in public CI logs.
pattern='(BEGIN ([A-Z0-9]+ )?PRIVATE KEY|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,}|AIza[0-9A-Za-z_-]{35}|xox[baprs]-[0-9A-Za-z-]{20,}|(sk|rk)_live_[0-9A-Za-z]{20,}|sk-(proj-|svcacct-)?[A-Za-z0-9_-]{40,})'

if git grep -I -l -E "$pattern" -- ':!scripts/secret-check.sh'; then
  echo "possible committed secret found (filenames above; values suppressed)" >&2
  exit 1
else
  status=$?
  if [ "$status" -ne 1 ]; then
    exit "$status"
  fi
fi
