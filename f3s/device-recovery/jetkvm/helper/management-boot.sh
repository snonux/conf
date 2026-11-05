#!/bin/sh
# Proposed bounded management-only boot canary; not installed by this repository.
# Replace PINNED_DEVICE_ID during root-reviewed installation. This hook cannot
# authorize a primary-network transaction; install pre-app.sh before any main arm.
while ! /userdata/jkrecover/jkrecover -device PINNED_DEVICE_ID management-boot; do
    echo 'jkrecover: bounded management boot guard failed; startup blocked' >&2
    /bin/busybox sleep 1
done
