# f3sctl agent on the f-hosts

[`f3sctl`](https://github.com/snonux/f3sctl) on pi0/pi1 SSHes into f0-f3 to
stop bhyve guests, power the host off and export the `zusb` backup pool.
It logs in as the restricted `f3sctl` account (replacing the old `wol-f3s`
heredoc-into-`doas sh` approach), which gonf manages: tasks `freebsd_f3sctl_*`
in [`gonf/freebsd/f3sctl.go`](../../../gonf/freebsd/f3sctl.go), doas rules in
`freebsd_base_doas` ([`../base/doas.conf`](../base/doas.conf)).

The account can only run `/usr/local/bin/f3sctl agent`, which takes one bare
verb and never hands anything to a shell:

| Verb | Root |
|---|---|
| `probe` | yes (`vm list`) |
| `zusb-status` | no |
| `zusb-unload` | yes |
| `carp-quiesce` | yes |
| `poweroff` | yes |

Layers:

1. Unprivileged user `f3sctl`, home `/var/db/f3sctl`, no `~/.ssh`. The key is
   in root-owned `/etc/ssh/authorized_keys.d/f3sctl`.
2. `from="192.168.1.125,192.168.1.126,192.168.2.203,192.168.2.204"` on the
   key (pi0/pi1, LAN and WireGuard). The public key line is
   [`authorized_keys`](authorized_keys) here; the private key stays on pi0/pi1.
3. `ForceCommand` in a `Match User f3sctl` block in `sshd_config` (the
   authoritative one; beats any `command=`). gonf owns the whole
   [`sshd_config`](sshd_config) (stock FreeBSD 15.1 file plus that block).
4. `doas` rules on exact argv for the four root verbs. `agent-root` refuses
   to run unless euid is 0.

The login shell must stay `/bin/sh`: sshd runs `ForceCommand` through it,
`nologin` would break the agent.

## Install

```sh
./gonf.sh -n cluster freebsd-hosts freebsd_f3sctl_package freebsd_f3sctl_account \
    freebsd_f3sctl_key freebsd_f3sctl_sshd freebsd_base_doas   # dry-run first
./gonf.sh cluster freebsd-hosts freebsd_f3sctl_package freebsd_f3sctl_account \
    freebsd_f3sctl_key freebsd_f3sctl_sshd freebsd_base_doas
```

(`./gonf.sh cluster freebsd-hosts freebsd` includes them.) The `f3sctl`
package comes from the custom pkg repo. gonf validates the `sshd_config`
candidate with `sshd -t -f` and `doas.conf` with `doas -C` before
installing, and reloads (never restarts) sshd only when `sshd_config`
changed. Still keep a second session open and test a new login afterwards:
the fallback is the console. To rotate the key, replace
[`authorized_keys`](authorized_keys) and deploy `freebsd_f3sctl_key`.
After a FreeBSD upgrade merged a new stock `sshd_config`, refresh
[`sshd_config`](sshd_config) from a host first, or gonf reverts the merge.

## Verify

From pi0, as the CGI user:

```sh
doas -u _httpd ssh -i /var/db/f3sctl/id_ed25519 -o IdentitiesOnly=yes \
    f3sctl@192.168.1.130 zusb-status      # "absent" or "loaded"
```

Must be refused: `'echo pwned'` and `'zusb-status; echo pwned'` (expected
exactly one verb), no command (no verb requested), `ssh -N -L
9999:127.0.0.1:22 ...` (no forwarding), and the same key from earth (`from=`).

Related: OpenBSD gateway equivalent (mute marker only, no doas)
[`frontends/scripts/setup-f3sctl-agent-openbsd.sh`](../../../frontends/scripts/setup-f3sctl-agent-openbsd.sh),
[`../zusb/README.md`](../zusb/README.md),
[`../shelly-fans/README.md`](../shelly-fans/README.md).
