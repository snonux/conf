# LAN access to f3s services

`https://<service>.f3s.lan.buetow.org` reaches k3s directly from the LAN,
bypassing the OpenBSD relayd and WireGuard path used for
`<service>.f3s.buetow.org`.

```
external: Internet -> OpenBSD relayd -> WireGuard -> Traefik (k3s)
LAN:      client -> CARP VIP 192.168.1.138 (f0/f1 relayd, TCP) -> Traefik :443 on r0/r1/r2
```

| Piece | Where |
|---|---|
| DNS | Pi-hole on pi2/pi3 answers `*.f3s.lan.buetow.org` with 192.168.1.138 ([`../pihole/README.md`](../pihole/README.md)) |
| CARP VIP | 192.168.1.138, vhid 1; f0 (192.168.1.130) master advskew 0, f1 (.131) backup advskew 100; also carries stunnel NFS on 2323 |
| relayd | f0 and f1, `/usr/local/etc/relayd.conf` (not in this repo), `relayd_enable=YES` |
| backend | `table <k3s_nodes> { 192.168.1.120 192.168.1.121 192.168.1.122 }`, `forward to <k3s_nodes> port 443 check tcp`, no `tls` keyword |
| TLS | Traefik, secret `f3s-lan-tls` from cert-manager ([`../cert-manager/README.md`](../cert-manager/README.md)) |
| client trust | `just export-ca` in `f3s/cert-manager`, install the CA |

## Add a service

Add a second ingress to the chart and push; nothing changes on relayd,
DNS or cert-manager.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: <app>-ingress-lan
  namespace: services
  annotations:
    spec.ingressClassName: traefik
    traefik.ingress.kubernetes.io/router.entrypoints: web,websecure
spec:
  tls:
    - hosts: [<app>.f3s.lan.buetow.org]
      secretName: f3s-lan-tls
  rules:
    - host: <app>.f3s.lan.buetow.org
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: <app>-service
                port:
                  number: <port>
```

This is `navidrome/helm-chart/templates/ingress.yaml`'s LAN ingress.

## Check and debug

```sh
doas sockstat -4 -l | grep 192.168.1.138      # on f0/f1: relayd :80/:443, stunnel :2323
ifconfig re0 | grep carp                      # MASTER/BACKUP
doas relayctl show hosts                      # backend health
doas relayd -n                                # config check
doas grep relayd /var/log/daemon.log | tail -50
curl -k -H "Host: navidrome.f3s.lan.buetow.org" https://192.168.1.120   # Traefik direct
kubectl get pods -n kube-system -l app.kubernetes.io/name=traefik
kubectl get ingress -n services | grep lan
```

| Symptom | Look at |
|---|---|
| name doesn't resolve | Pi-hole wildcard on pi2/pi3 |
| connection refused | relayd running on the CARP master, `ifconfig re0 | grep carp` |
| 502 / timeout | k3s nodes reachable from f0/f1, Traefik pods, `relayctl show hosts` |
| 404 | the `*-ingress-lan` object exists and points at the right service |
| cert warning | CA not trusted, or stale `f3s-lan-tls` in that namespace |

After changing relayd TLS settings: `doas service relayd restart` on f0
and f1, never `reload`.

Failover test: `doas ifconfig re0 down` on f0, curl a LAN name (f1 takes
over in a few seconds), `doas ifconfig re0 up`.
