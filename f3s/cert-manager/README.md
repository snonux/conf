# cert-manager (LAN TLS)

Self-signed wildcard certificate `*.f3s.lan.buetow.org` for the LAN
ingresses. Traefik terminates TLS with the `f3s-lan-tls` secret; relayd on the
CARP VIP (192.168.1.138) on f0/f1 is plain TCP passthrough and holds no cert.
Full LAN path: [`../docs/lan-access-setup-guide.md`](../docs/lan-access-setup-guide.md).

| File | Content |
|---|---|
| `cert-manager.yaml` | upstream cert-manager v1.20.4, unmodified (see its header for the upgrade path) |
| `kustomization.yaml` | how ArgoCD renders the directory: lists every manifest and patches memory requests/limits into the three Deployments (`just upgrade` applies it with `kubectl apply -k .`; `just install` bootstraps with the plain files and ArgoCD adds the patches); it also marks the CRDs `Prune=false,Delete=false` for ArgoCD |
| `self-signed-issuer.yaml` | self-signed ClusterIssuer |
| `ca-certificate.yaml` | CA `selfsigned-ca` (secret `selfsigned-ca-secret`), issuer `selfsigned-ca-issuer` |
| `wildcard-certificate.yaml` | one `Certificate` `f3s-lan-wildcard` -> secret `f3s-lan-tls` per namespace |

Deployed by ArgoCD (`argocd-apps/infra/cert-manager.yaml`). Manual:

```sh
just install      # apply all, wait for CA and wildcard Ready
just upgrade
just status
just export-ca    # -> /tmp/f3s-lan-ca.crt for client trust
just renew        # delete + re-create f3s-lan-wildcard in cert-manager ns
just uninstall
```

`just export-certs` still exists but nothing consumes its output any more.

## Upgrading cert-manager

Follow <https://cert-manager.io/docs/releases/upgrading/>: one minor version
at a time, latest patch of each, and check the Kubernetes range in the
supported-releases table (1.21 needs k8s >= 1.33; the cluster runs 1.32).
With static manifests the CRDs ship inside `cert-manager.yaml`, so there is
no Helm `installCRDs`/`crds.enabled` handling to worry about.

1. Back up: `kubectl get -A certificates,issuers,clusterissuers -o yaml > backup.yaml`.
2. Pause ArgoCD, or selfHeal reverts every intermediate step:
   `kubectl -n cicd patch application cert-manager --type merge -p '{"spec":{"syncPolicy":{"automated":null}}}'`.
3. Per minor: `kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/vX.Y.Z/cert-manager.yaml`,
   wait for the three rollouts, check `kubectl get certificate -A` is all
   Ready and issue a throwaway Certificate from `selfsigned-ca-issuer` in a
   scratch namespace. On failure, re-apply the previous version's manifest.
4. Vendor the final manifest here (keep the header), commit, push to
   Forgejo, then `kubectl apply -f ../argocd-apps/infra/cert-manager.yaml`
   to restore auto-sync; ArgoCD prunes objects the new release dropped.

Things the 1.14 -> 1.20 upgrade changed for this setup:

- 1.17: the RSA-4096 CA now signs leaves with SHA-512 instead of SHA-256.
- 1.18: `rotationPolicy` defaults to `Always`. The CA pins `Never` so its key
  (trusted by clients) survives renewal; the leaves set `Always` explicitly.
- 1.18: `revisionHistoryLimit` defaults to 1, so only the latest
  CertificateRequest per Certificate is kept.

## One certificate per namespace

An Ingress can only use a TLS secret from its own namespace, so each
namespace with `.lan` hosts has its own `Certificate`: `cert-manager`
(source, CA export), `services` (all `*-ingress-lan`), `cicd`. All are signed
by the same CA, so clients trust one root.

Never `kubectl apply` a copy of the secret into another namespace.
cert-manager doesn't see copies, so they expire at the next renewal (this
caused the August 2026 outage).

Validity 90 days, `renewBefore: 360h`. Renewal is automatic and Traefik
reloads the secret itself.

Check what is actually served:

```sh
kubectl get certificate -A
for ns in cert-manager services cicd; do
  kubectl -n $ns get secret f3s-lan-tls -o jsonpath='{.data.tls\.crt}' \
    | base64 -d | openssl x509 -noout -enddate
done
echo | openssl s_client -connect 192.168.1.138:443 \
  -servername code.f3s.lan.buetow.org 2>/dev/null | openssl x509 -noout -enddate
```

## relayd pitfall

After editing relayd's TLS config on f0/f1, `doas service relayd restart` on
both (not `reload`: SIGHUP doesn't reload keypairs or switch modes). The old
`/usr/local/etc/ssl/relayd/f3s.lan.buetow.org*` files there are unused.

## Trust the CA on clients

```sh
just export-ca
sudo cp /tmp/f3s-lan-ca.crt /etc/pki/ca-trust/source/anchors/ && sudo update-ca-trust     # Fedora
sudo cp /tmp/f3s-lan-ca.crt /usr/local/share/ca-certificates/ && sudo update-ca-certificates  # Debian/Ubuntu
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /tmp/f3s-lan-ca.crt  # macOS
```

- Windows: open the `.crt`, install to "Trusted Root Certification Authorities" (Local Machine).
- Android: Settings, Security, Encryption & credentials, Install a certificate, CA certificate.
- iOS: install the profile (VPN & Device Management), then enable full trust
  under About, Certificate Trust Settings.
