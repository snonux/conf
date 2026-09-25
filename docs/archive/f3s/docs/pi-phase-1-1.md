# PI Phase 1.1 lighttpd on pi0/pi1

Task 1.1 for the Raspberry Pi cluster was completed on:

- `pi0.lan.buetow.org`
- `pi1.lan.buetow.org`

Completed actions:

- `lighttpd` installed
- `/var/www/html` created and owned by `root:root`
- `/etc/lighttpd/lighttpd.conf` set to the plan:
  - `server.port = 80`
  - `server.document-root = "/var/www/html"`
  - `server.errorlog = "/var/log/lighttpd/error.log"`
  - `accesslog.filename = "/var/log/lighttpd/access.log"`
  - `dir-listing.activate = "enable"`
  - `index-file.names = ( "index.html", "index.htm" )`
  - `server.modules = ( "mod_access", "mod_accesslog", "mod_dirlisting", "mod_staticfile" )`
- `lighttpd` enabled and started via systemd
- `firewalld` was already running on both hosts, so the `http` service was added and reloaded

Verification:

- `systemctl status lighttpd` reported `active (running)`
- `ss -tlnp | grep :80` showed `lighttpd` listening on `0.0.0.0:80`
- `curl -I localhost` returned `HTTP/1.1 200 OK`

Firewall note:

- `firewall-cmd --state` was checked before any firewall changes.
- No firewall changes were attempted unless `firewalld` was confirmed running.
