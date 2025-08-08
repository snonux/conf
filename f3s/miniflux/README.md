# Miniflux Helm Chart

This chart deploys Miniflux.

## Prerequisites

Before installing the chart, you must manually create the following:

1.  **Database Password Secret:**

    Create a secret that contains only the database password. The chart reads
    this value and constructs the Miniflux `DATABASE_URL` internally at runtime:

    ```bash
    kubectl create secret generic miniflux-db-password \
      --from-literal=fluxdb_password='YOUR_PASSWORD' \
      -n services
    ```

    Replace `YOUR_PASSWORD` with your desired database password. You do not
    need to provide a full DSN in the secret; the chart uses the password from
    `fluxdb_password` to build:

    `postgres://miniflux:${POSTGRES_PASSWORD}@miniflux-postgres:5432/miniflux?sslmode=disable`

2.  **Admin Password Secret:**

    Create a secret for the initial Miniflux admin user password. The chart
    reads this secret into the `ADMIN_PASSWORD` environment variable during
    the first startup to create the admin user. The admin username is set
    to `admin` in the deployment template.

    ```bash
    kubectl create secret generic miniflux-admin-password \
      --from-literal=admin_password='YOUR_ADMIN_PASSWORD' \
      -n services
    ```

    Replace `YOUR_ADMIN_PASSWORD` with your desired password. The secret key
    used by the chart is `admin_password`.

3.  **Persistent Volume Directory:**

    You must manually create the directory on your host system to be used by the persistent volume:

    ```bash
    mkdir -p /data/nfs/k3svolumes/miniflux/data
    ```

## Installing the Chart

To install the chart with the release name `miniflux`, run the following command:

```bash
helm install miniflux . --namespace services --create-namespace
```
