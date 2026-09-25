# Update certs

To update certs run from the repository root:

```sh
./gonf.sh cluster frontends frontends_nsd frontends_httpd frontends_acme frontends_acme_invoke frontends_relayd
```

`frontends_acme_invoke` is the explicit certificate request; it is not part
of the `frontends` aggregate. Then verify that relayd doesn't throw any error:

```sh
ssh -p 2 rex@blowfish.buetow.org 'doas rcctl check relayd'
ssh -p 2 rex@fishfinger.buetow.org 'doas rcctl check relayd'
```
