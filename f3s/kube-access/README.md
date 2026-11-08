# Laptop Kubernetes API over wg2

Earth's default kubeconfig now uses `https://127.0.0.1:16443` with
`tls-server-name: r0.wg0.wan.buetow.org`, retaining the existing CA and client
credentials. `wg0` remains available; `lan` preserves the original direct
LAN endpoint. Context `wg2` is an explicit alias for the default connection.
No kubeconfig or credentials belong in Git.

The user service `f3s-kube-wg2.service` binds only loopback and forwards via
SSH from wg2 address10.99.0.2 to root@10.20.0.120:22, then r0's local6443.
Existing trusted host alias r0 and SSH identity/agent are used. This requires
wg-quick@wg2.service to be running and the existing SSH key to be available;
failed connections retry every10seconds. Enabled user-service startup is
not a tested reboot guarantee; an encrypted SSH identity may need unlocking
at login. No router API port or public listener is opened.

Install the non-secret unit when rebuilding the laptop:

```sh
install -m644 f3s/kube-access/f3s-kube-wg2.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now f3s-kube-wg2.service
kubectl get nodes
```

The kubeconfig edit is local and requires the existing credential-bearing
config. Preserve the original config privately before editing. Set only the
default cluster's server and TLS server name as above; keep CA/client auth,
namespace and alternative contexts. Never use insecure-skip-tls-verify.

Control:

```sh
systemctl --user status f3s-kube-wg2.service
systemctl --user restart f3s-kube-wg2.service
kubectl config use-context default
```

Verified on2026-11-08: real default kubectl TLS/authenticated node/pod queries,
all3nodes Ready, listener127.0.0.1 only, wg2 route/source99.2. First immediate
query raced SSH startup; listener then appeared and subsequent queries passed.
Private receipt/backup: ~/.local/state/conf/kube-wg2-20261108/.
