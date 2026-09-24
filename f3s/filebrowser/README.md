# File Browser

`https://filebrowser.f3s.buetow.org` (LAN: `filebrowser.f3s.lan.buetow.org`).
Namespace `services`, ArgoCD app `filebrowser`. First login `admin`/`admin`;
change it.

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
