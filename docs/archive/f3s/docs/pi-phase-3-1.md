# PI Phase 3.1 End-to-End Verification

Task 3.1 for the Raspberry Pi cluster was verified across:

- `pi0.lan.buetow.org`
- `pi1.lan.buetow.org`
- `pi2.lan.buetow.org`
- `pi3.lan.buetow.org`

Client-side verification was performed from this workstation on the LAN, not from a separate unmanaged client. That was sufficient to validate LAN reachability, but browser-specific behavior on a distinct client remains an environmental limitation.

Verified behavior:

- Static farm content from `pi0` and `pi1` matched byte-for-byte with the same SHA-256 hash.
- `pi0` and `pi1` both served the static landing page over HTTP.
- Pi-hole DNS on `pi2` and `pi3` resolved `google.com` successfully.
- Pi-hole block behavior returned `0.0.0.0` for `doubleclick.net` on both DNS nodes.
- Pi-hole admin UI on both DNS nodes returned `302 Found` to `/admin/login`.
- Taking `lighttpd` down on `pi0` caused `pi0` HTTP to fail while `pi1` continued serving the same content.
- Taking Pi-hole down on `pi2` caused DNS and admin access on `pi2` to fail while `pi3` continued answering queries and serving the admin UI.
- `lighttpd` was restored on `pi0` and the Pi-hole stack was restored on `pi2` before finishing.

Resource and stability checks:

- `lighttpd` on `pi0` and `pi1` stayed lightweight during normal checks, with both hosts showing roughly 780 MiB available RAM.
- Pi-hole on `pi2` and `pi3` stayed healthy after restart and reported normal startup logs.
- Recent `journalctl -u lighttpd` output on `pi0` showed only the intentional stop/start events from the redundancy test.
- Recent Pi-hole container logs on `pi2` and `pi3` showed normal startup and web server initialization messages.

LAN port scan:

- `nmap -Pn -p 80,53 pi0 pi1 pi2 pi3` showed `80/tcp` open on all four Pis.
- `53/tcp` was filtered on `pi0` and `pi1`.
- `53/tcp` was open on `pi2` and `pi3`.

Notes:

- The workstation initially lacked `nmap`, so it was installed locally before the LAN scan.
- Browser rendering and a truly separate LAN client were not available in this environment, so the verification used direct HTTP, DNS, and port-scan checks from the LAN workstation instead.
