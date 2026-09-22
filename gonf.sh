#!/bin/sh
#
# Run the conf Gonf recipes: ./gonf.sh -list, ./gonf.sh plan -o DIR TASK, ...
#
# Works from any working directory. It changes into this checkout's gonf
# module first, because api.MustSecret/OptionalSecret resolve the secrets
# root (gonf/secrets) relative to the recipe process's working directory.
# Relative path arguments such as "plan -o out" are therefore relative to
# that gonf directory, as before. Arguments are passed through verbatim.
#
# Asset root: the recipes default to ~/git/conf (Home, the logical $HOME
# path). Only when this wrapper lives in a different checkout (a second
# worktree) does it export GONF_CONF_ROOT, as the logical path it was
# invoked by; an existing GONF_CONF_ROOT wins. Symlinks are never resolved
# into physical paths (FreeBSD /home -> /usr/home), so recorded controller
# paths stay as before. The checkout is the directory of the path the
# wrapper was invoked by: calling it through a symlink to gonf.sh uses the
# symlink's directory.
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -L)
if [ -z "${GONF_CONF_ROOT:-}" ] && ! [ "$repo_dir" -ef "${HOME:-}/git/conf" ]; then
	GONF_CONF_ROOT=$repo_dir
	export GONF_CONF_ROOT
fi
cd "$repo_dir/gonf"
exec go run ./cmd/gonf "$@"
