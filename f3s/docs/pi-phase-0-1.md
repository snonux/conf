# PI Phase 0.1 Baseline

Task 0.1 for the Raspberry Pi cluster was completed on:

- `pi0.lan.buetow.org`
- `pi1.lan.buetow.org`
- `pi2.lan.buetow.org`
- `pi3.lan.buetow.org`

Completed actions:

- `dnf update -y`
- `dnf upgrade -y`
- `epel-release` installed
- `crb` enabled
- `htop`, `vim`, `curl`, `rsync`, `net-tools`, `firewalld`, and `policycoreutils-python-utils` installed
- `firewalld` enabled and running

Verification:

- `hostnamectl --static` matched each target hostname
- `free -h` reported about 909 MiB total RAM on each Pi
- `rpm -q` confirmed the requested packages were installed
- `dnf repolist enabled` showed `crb` and `epel`
- `firewall-cmd --state` reported `running`
- `dnf -q check-update` reported no pending updates

Firewall note:

- No firewall rule changes were made in this phase.
- `firewall-cmd` was only checked after confirming `firewalld` was running.
