#!/bin/sh
set -eu
if git grep -I -n -E '(BEGIN (RSA|OPENSSH|EC) PRIVATE KEY|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{30,})' -- ':!scripts/secret-check.sh'; then
  echo "possible committed secret found" >&2
  exit 1
fi
