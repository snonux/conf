# WebDAV

Apache WebDAV on `https://webdav.f3s.buetow.org/webdav/` (LAN:
`webdav.f3s.lan.buetow.org`). Namespace `services`, ArgoCD app `webdav`.
Serves File Browser's data: it mounts `filebrowser-data-pvc`
(`/data/nfs/k3svolumes/filebrowser/data`) at `/var/www/webdav`, so File
Browser must be deployed first. Runs as UID 65534 (fsGroup 65534).

Auth comes from secret `webdav-htpasswd` (key `webdav.htpasswd`), not in git:

```sh
htpasswd -cb /tmp/webdav.htpasswd USER PASS        # httpd-tools; -b without -c to add users
kubectl delete secret webdav-htpasswd -n services --ignore-not-found
kubectl create secret generic webdav-htpasswd \
    --from-file=webdav.htpasswd=/tmp/webdav.htpasswd -n services
kubectl rollout restart deployment/webdav -n services
rm /tmp/webdav.htpasswd
```

```sh
just status | logs [lines] | port-forward [8080] | sync | argocd-status | restart
```
