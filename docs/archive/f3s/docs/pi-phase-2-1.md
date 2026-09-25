# PI Phase 2.1 Docker CE on pi2/pi3

Task 2.1 for the Raspberry Pi cluster was completed on:

- `pi2.lan.buetow.org`
- `pi3.lan.buetow.org`

Completed actions:

- `podman` and `buildah` were confirmed absent, so no removal was needed.
- `yum-utils` was installed to provide `yum-config-manager`.
- The Docker CE repository was added from `https://download.docker.com/linux/centos/docker-ce.repo`.
- `docker-ce`, `docker-ce-cli`, `containerd.io`, `docker-buildx-plugin`, and `docker-compose-plugin` were installed.
- `docker` was enabled and started via systemd.
- `paul` was added to the `docker` group.

Verification:

- `rpm -q docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin` succeeded on both hosts.
- `systemctl is-enabled docker` returned `enabled` on both hosts.
- `systemctl is-active docker` returned `active` on both hosts.
- `id -nG` showed `docker` in the group list for `paul`.
- `docker run --rm hello-world` completed successfully on both hosts.

Notes:

- The Docker installation pulled SELinux-related dependencies on Rocky Linux 9 aarch64, including `container-selinux`, `passt`, and `fuse-overlayfs`.
- No firewall changes were made in this phase because Docker setup did not require any.
