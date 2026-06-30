# f3s Rack Fans (Shelly Plug)

The rack fans are powered by a **Shelly Plug M Gen 3** at `192.168.1.28`. Each
f-host (f0/f1/f2/f3) turns the plug on at boot so the fans always run while any
host is up. Turning the plug *off* is handled centrally by `wol-f3s shutdown-all`
(on earth / the Pis), not by the hosts.

The plug has authentication enabled (HTTP digest, user `admin`).

## Installed Files

- `/usr/local/sbin/shelly-fans-on` calls the Shelly RPC API to switch the plug
  on, retrying for ~60s in case networking or the plug is briefly unavailable.
- `/usr/local/etc/rc.d/shellyfans` runs the helper at boot, after networking
  and after `f3skeys` (so the `/keys` USB stick is already mounted).
- `/keys/shelly_plug.secret` holds the plug password (first line). It lives on
  the UFS USB key stick alongside the ZFS encryption keys — **not** in git and
  not on the host's own disk. `/keys` is mounted read-only at boot, so adding
  the file requires temporarily remounting read-write.

## Host Configuration

On each f-host install the scripts and enable the service:

```sh
doas install -o root -g wheel -m 0555 shelly-fans-on /usr/local/sbin/shelly-fans-on
doas install -o root -g wheel -m 0555 shellyfans.rc /usr/local/etc/rc.d/shellyfans
doas sysrc shellyfans_enable=YES
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

```sh
doas service shellyfans start
tail -f /var/log/messages | grep shellyfans   # expect "Rack fans switched on"
```

After a reboot, confirm the plug reports `output:true`:

```sh
curl -s --digest -u admin:"$(head -n1 /keys/shelly_plug.secret)" \
  'http://192.168.1.28/rpc/Switch.GetStatus?id=0'
```
