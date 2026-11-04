# Triggerfish unattended updates

`./gonf.sh cluster triggerfish triggerfish_unattended_cron` installs the
wrapper, log rotation and root cron jobs through task dependencies. This
target does not apply the router's network recipe or run an upgrade.
The full `triggerfish` aggregate also includes these tasks.

Daily schedules use the router's local timezone: base patches at 04:10,
package updates at 04:40 and the package audit at 05:10. Updates use the
official mirror selected by `/etc/installurl`, without the frontend custom
repository or its high-availability partner checks. These are updates within
the installed OpenBSD release; release upgrades remain manual.

The router never reboots automatically. Base patches and a kernel build that
differs from the booted kernel produce an operator reminder to arrange a
restart or reboot. Package updates restart the currently running `uptimed`
daemon; network services remain running. Review additional package daemons
if any are installed later.

Changes, failures and restart reminders go to root's normal cron mail and
`/var/log/unattended-upgrade-triggerfish.log` (0600; five compressed rotated
logs, 1024 KB threshold). A clean package audit is logged without mail.
An audit checks pending package updates and quirks advisories; it is not a
complete CVE database scan. Overlapping jobs fail visibly. Abandoned lock
directories are reclaimed after two hours.

This recipe uses the existing Triggerfish host inventory and baseline
registration in `gonf/cluster` and `gonf/tasks`.
