# Frontends

Rexify my internet facing frontend servers!

## goprecords upload (fishfinger, blowfish)

Uptimed stats are pushed once per day from **`/etc/daily.local`** via **`/usr/local/bin/goprecords-upload.sh`** (POSIX **`sh`**, deploy **`rex goprecords_upload`** or **`rex commons`**).

Bearer tokens live in **geheim** as plain text (one line, no newline):

- **`secrets/etc/goprecords/fishfinger.token`**
- **`secrets/etc/goprecords/blowfish.token`**

Issue or rotate keys on the goprecords daemon (Kubernetes example):

```bash
kubectl exec -n services deployment/goprecords -- \
  goprecords --create-client-key fishfinger -stats-dir=/data/stats
kubectl exec -n services deployment/goprecords -- \
  goprecords --create-client-key blowfish -stats-dir=/data/stats
```

Then update the matching **`secrets/etc/goprecords/<host>.token`** file and re-run **`rex goprecords_upload`** (or **`commons`**) so the script on each host is regenerated.
