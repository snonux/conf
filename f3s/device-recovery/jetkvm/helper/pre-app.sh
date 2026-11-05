#!/bin/sh
# Proposed /userdata/init.d/S??jkrecover wrapper; not installed by this repository.
# RkLunch ignores hook exit codes. Never return an error and continue target startup.
# Fill the pinned hardware ID during root-reviewed native installation.
while ! /userdata/jkrecover/jkrecover -device PINNED_DEVICE_ID pre-app; do
    echo 'jkrecover: pre-app restoration/readiness failed; startup blocked' >&2
    /bin/busybox sleep 1
done
