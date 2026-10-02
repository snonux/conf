# File Browser

`https://filebrowser.f3s.buetow.org` (LAN: `filebrowser.f3s.lan.buetow.org`).
Namespace `services`, ArgoCD app `filebrowser`. First login `admin`/`admin`;
change it.

Runs a patched build of the fork `~/git/filebrowser` (`snonux/filebrowser`),
not the upstream image: upstream was archived on 2026-09-01 and left published
security advisories unfixed. The image lives in the f3s registry, tagged with
the fork's git commit.

Volumes under `/data/nfs/k3svolumes/filebrowser/`: `data` (50Gi, also served
by [WebDAV](../webdav/README.md)), `database` (1Gi), `config` (1Gi). Create
them once, owned 1000:1000, mode 775:

```sh
mkdir -p /data/nfs/k3svolumes/filebrowser/{data,database,config}
chown -R 1000:1000 /data/nfs/k3svolumes/filebrowser/
chmod -R 775 /data/nfs/k3svolumes/filebrowser/
```

```sh
just status | logs [lines] | port-forward [8080] | sync | argocd-status | restart
```

## Build and deploy a new image

The frontend needs Node >= 24 and pnpm, so it is built in a container; the
`Dockerfile` then packages the locally built binary.

```sh
cd ~/git/filebrowser
go test ./... && go vet ./...

podman run --rm -v "$PWD/frontend:/src:Z" -w /src docker.io/library/node:24-alpine \
  sh -c 'corepack enable && pnpm install --frozen-lockfile && pnpm run build'

TAG=$(git rev-parse --short HEAD)
CGO_ENABLED=0 go build -trimpath -o filebrowser -ldflags="-s -w \
  -X github.com/filebrowser/filebrowser/v2/version.Version=2.63.23-snonux.1 \
  -X github.com/filebrowser/filebrowser/v2/version.CommitSHA=$TAG" .

podman build -t filebrowser:$TAG .
podman tag filebrowser:$TAG r0.lan.buetow.org:30001/filebrowser:$TAG
podman push --tls-verify=false r0.lan.buetow.org:30001/filebrowser:$TAG
```

Then set the tag in `helm-chart/Chart.yaml` (`appVersion`) and
`helm-chart/templates/deployment.yaml` (`image`), commit, push to the `master`
and `forgejo` remotes, and run `just sync`. Running the image locally as a
non-root user needs `--sysctl net.ipv4.ip_unprivileged_port_start=0` for port 80.
