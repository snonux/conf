# Private Docker registry

Plain-HTTP registry, NodePort 30001 on every k3s node, ArgoCD app
`registry` in namespace `infra`, selector `app=docker-registry`. Data at
`/data/nfs/k3svolumes/registry` (create once; it carries an NFS sentinel).
Inside the cluster it is also `docker-registry-service:5000`. relayd on the
OpenBSD frontends forwards `registry.f3s.buetow.org` to NodePort 30001
(`f3s_registry` table).

## Push from a workstation

```sh
sudo bash -c 'echo "{ \"insecure-registries\": [\"r0.lan.buetow.org:30001\",\"r1.lan.buetow.org:30001\",\"r2.lan.buetow.org:30001\"] }" > /etc/docker/daemon.json'
sudo systemctl restart docker
docker tag <image> r0.lan.buetow.org:30001/<image>
docker push r0.lan.buetow.org:30001/<image>
```

Charts pull `registry.lan.buetow.org:30001/<image>`.

## k3s node setup (r0, r1, r2)

Managed by gonf: `rnodes_base_hosts` maps `registry.lan.buetow.org` to
127.0.0.1 in `/etc/hosts`, and `rnodes_k3s_registries` installs
`/etc/rancher/k3s/registries.yaml` (mirror `registry.lan.buetow.org:30001` ->
`http://localhost:30001`, asset `f3s/r-nodes/k3s/registries.yaml`):

```sh
./gonf.sh cluster rocky-k3s rnodes_base_hosts rnodes_k3s_registries
```

gonf does not restart k3s (etcd quorum); a registries.yaml change applies at
the next boot or a manual one-node-at-a-time `systemctl restart k3s`.

```sh
just status | logs [lines] | port-forward [5000] | sync | argocd-status | restart
```
