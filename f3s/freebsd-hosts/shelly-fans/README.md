# f3s Rack Fans (Shelly Plug)

The rack fans are powered by a Shelly Plug M Gen 3 at `10.30.0.28`. Each
f-host (f0/f1/f2/f3) turns the plug on at boot so the fans always run while any
host is up. Turning the plug *off* is handled centrally by `f3sctl power off` /
`f3sctl fans off` (on earth / pi0 / pi1), not by the hosts.

The plug has authentication enabled (HTTP digest, user `admin`). Since p34
(2026-11-09) the helper reaches it over the plug's native HTTPS (TCP443) and
verifies its certificate against the private f3s Shelly CA (`--cacert`,
never `-k`). A missing or wrong CA fails closed; the helper never falls back
to plain HTTP. The password goes to curl through a config on stdin, not the
command line, and `--fail` turns a 401 into a failure. Every failed attempt is
logged with curl's exit code (7 unreachable, 22 HTTP error/wrong password,
60 certificate, 90 pin mismatch).

**Clock-independent fallback (p34 review B1).** The helper runs early in rc,
before ntpd. After an RTC/CMOS reset the clock can be before the plug
certificate's notBefore, and full verification then fails (60) on every try.
Only when curl exits 60 **and** the clock is before that notBefore does the
helper retry with `-k --pinnedpubkey sha256//<SPKI hash of the shelly1 leaf>`.
curl still refuses any server with another public key, so the plug stays
authenticated; only the time check is skipped. It logs "clock implausible ...,
using pinned key". Without a well-formed pin there is no `-k` attempt at all.

## Installed Files

- `/usr/local/sbin/shelly-fans-on` calls the Shelly RPC API to switch the plug
  on, retrying for ~60s in case networking or the plug is briefly unavailable.
  `shelly-fans-on status` makes the same call with the read-only
  `Switch.GetStatus`, once.
- `/usr/local/etc/shellyfans/shelly-ca.pem` is the public f3s Shelly CA that the
  helper pins (`shelly-ca.pem` here; the same certificate as
  `gonf/netbsd/assets/shelly-ca.pem`).
- `/usr/local/etc/shellyfans/shelly1.pin` holds `SHELLY_PIN=sha256//...` and
  `SHELLY_PIN_NOT_BEFORE=<epoch>`. gonf generates it at install time from the
  public leaf `shelly1-leaf.pem` here, and refuses a leaf that does not chain to
  the CA or is not for `10.30.0.28`.

**Renewing the shelly1 leaf:** commit the new public leaf as `shelly1-leaf.pem`
and redeploy `freebsd_shellyfans_scripts` (one host at a time). That updates the
pin and notBefore; the plug can switch to the new leaf afterwards. Consider
backdating the new leaf's notBefore by a few days at issuance, so a slightly
wrong clock still passes full verification.
- `/usr/local/etc/rc.d/shellyfans` runs the helper at boot, after networking
  and after `f3skeys` (so the `/keys` USB stick is already mounted).
- `/keys/shelly_plug.secret` holds the plug password (first line). It lives on
  the UFS USB key stick alongside the ZFS encryption keys, not in git and
  not on the host's own disk. `/keys` is mounted read-only at boot, so adding
  the file requires temporarily remounting read-write.

## Host Configuration

gonf installs both scripts (0555 root:wheel) and sets
`shellyfans_enable="YES"` on every f-host (`gonf/freebsd/shellyfans.go`,
tasks `freebsd_shellyfans_*`). It never runs the helper or the service, so a
deploy does not switch the plug:

```sh
./gonf.sh -n cluster freebsd-hosts freebsd_shellyfans_scripts freebsd_shellyfans_rc_conf
./gonf.sh cluster freebsd-hosts freebsd_shellyfans_scripts freebsd_shellyfans_rc_conf
```

Put the plug password on the USB key stick (mounted read-only at `/keys`):

```sh
doas mount -u -o rw /keys
printf '%s\n' '<password>' | doas tee /keys/shelly_plug.secret >/dev/null
doas chmod 0400 /keys/shelly_plug.secret
doas chown root:wheel /keys/shelly_plug.secret
doas mount -u -o ro /keys
```

The same password file is placed on every host's stick (one shared plug).

## Verification

Read-only, over the same verified HTTPS path (does not touch the relay):

```sh
doas /usr/local/sbin/shelly-fans-on status          # expect "output":true
doas env SHELLY_CA_FILE=/tmp/other-ca.pem /usr/local/sbin/shelly-fans-on status  # wrong CA: curl exit 60
```

Offline fixture for the helper (local TLS test servers, never the plugs):
`python3 -I check-helper.py` covers the normal path, the pinned fallback, a
wrong pinned key, no fallback for a plausible clock, and no `-k` without a
pin.

`doas service shellyfans start` switches the plug on (logged as
"Rack fans switched on"); only run it when the fans should be on.
