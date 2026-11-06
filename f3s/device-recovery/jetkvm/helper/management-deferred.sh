#!/bin/sh
# SOURCE CANDIDATE ONLY: not installed by this repository.
# Replace PINNED_DEVICE_ID only in a reviewed, separately authorized boot trial.
# Management-only startup must always return to stock RkLunch/vendor networking.
# BusyBox timeout bounds the foreground launcher, not its process descendants.
# The detached continuation has its own immutable <=30s claim; the independent
# canary watchdog retains the ORIGINAL 120s expiry even if this launcher dies.
# Never substitute this wrapper for the fail-closed full-primary pre-app guard.
if ! /bin/busybox timeout -k 1 12 /userdata/jkrecover/jkrecover -device PINNED_DEVICE_ID management-deferred; then
    echo 'jkrecover: deferred management startup not accepted; vendor startup continues' >&2
fi
exit 0
