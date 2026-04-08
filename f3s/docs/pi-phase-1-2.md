# PI Phase 1.2 Static Content Sync on pi0/pi1

Task 1.2 for the Raspberry Pi cluster was completed on:

- `pi0.lan.buetow.org` as the source content node
- `pi1.lan.buetow.org` as the synchronized target

Completed actions:

- Created `/var/www/html/index.html` on `pi0` with the static farm landing page content
- Synchronized the `pi0` content tree to `pi1` with `rsync`
- Preserved the source content as the reference copy on `pi0`
- Did not install a cron job, because the plan marked it optional and this task did not require it

Verification:

- `curl -fsS http://localhost` on `pi0` and `pi1` returned the same page content hash
- `sha256sum /var/www/html/index.html` on `pi0` and `pi1` returned the same hash
- Both hosts produced `97667da1e299f54b9831532171f1980f214018001770cecbe5de1bc127aa1552`

Notes:

- The source node initially had no `index.html`, so the landing page was created on `pi0` before syncing.
- `pi0` could not resolve `pi1.lan.buetow.org` for an outbound peer-to-peer rsync, so the final sync used the workstation as a relay while still treating `pi0` as the source of truth.
