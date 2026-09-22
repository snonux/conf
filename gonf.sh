#!/bin/sh
#
# Run the conf Gonf recipes: ./gonf.sh -list, ./gonf.sh plan -o DIR TASK, ...
#
# Works from any working directory. It changes into this checkout's gonf
# module first, because api.MustSecret/OptionalSecret resolve the secrets
# root (gonf/secrets) relative to the recipe process's working directory.
# Relative path arguments such as "plan -o out" are therefore relative to
# that gonf directory, as before. GONF_CONF_ROOT is set to this checkout (an
# existing value wins), so recipes read the assets of the checkout that runs
# them rather than always ~/git/conf. Arguments are passed through verbatim.
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
GONF_CONF_ROOT=${GONF_CONF_ROOT:-$repo_dir}
export GONF_CONF_ROOT
cd "$repo_dir/gonf"
exec go run ./cmd/gonf "$@"
