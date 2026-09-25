# f3sctl agent on the f-hosts

[`f3sctl`](https://github.com/snonux/f3sctl) on pi0/pi1 SSHes into f0-f3 to
stop bhyve guests, power the host off and export the `zusb` backup pool.
This sets up the restricted `f3sctl` account it logs in as (replacing the old
`wol-f3s` heredoc-into-`doas sh` approach).

The account can only run `/usr/local/bin/f3sctl agent`, which takes one bare
verb and never hands anything to a shell:

| Verb | Root |
|---|---|
| `probe` | yes (`vm list`) |
| `zusb-status` | no |
| `zusb-unload` | yes |
| `poweroff` | yes |

Layers:

1. Unprivileged user `f3sctl`, home `/var/db/f3sctl`, no `~/.ssh`. The key is
   in root-owned `/etc/ssh/authorized_keys.d/f3sctl`.
2. `from="192.168.1.125,192.168.1.126,192.168.2.203,192.168.2.204"` on the
   key (pi0/pi1, LAN and WireGuard).
3. `ForceCommand` in a `Match User f3sctl` block in `sshd_config` (the
   authoritative one; beats any `command=`).
4. `doas` rules on exact argv for the three root verbs. `agent-root` refuses
   to run unless euid is 0.

The login shell must stay `/bin/sh`: sshd runs `ForceCommand` through it,
`nologin` would break the agent.

## Install

```sh
doas pkg install -y f3sctl
doas sh setup-agent-freebsd.sh 'ssh-ed25519 AAAA... f3sctl@pi'
```

Idempotent, runs `sshd -t` before reloading, re-run to rotate the key. Keep a
second root session open: it reloads sshd, and the fallback is the console.

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
