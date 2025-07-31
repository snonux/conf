# Private Docker Registry

This document describes how to push Docker images to the private registry deployed in your Kubernetes cluster.

## Prerequisites

*   The `infra` namespace must exist in your cluster. If it doesn't, create it with `kubectl create namespace infra`.

*   A running Kubernetes cluster.
*   `kubectl` configured to connect to your cluster.
*   Docker installed and running on your local machine.

## Steps

0.  **Create the registry directory in the NFS share**

1.  **Tag your Docker image:**

    Replace `<your-image>` with the name of your local Docker image and `<node-ip>` with the IP address of any node in your Kubernetes cluster. The registry is available on NodePort `30001`.

    ```bash
    docker tag <your-image> <node-ip>:30001/<your-image>
    ```

2.  **Push the image to the registry:**

    ```bash
    docker push <node-ip>:30001/<your-image>
    ```

3.  **Pull the image from the registry (from a Kubernetes pod):**

    You can now use the image in your Kubernetes deployments by referencing it as `docker-registry-service:5000/<your-image>`.

## Communication

The Docker registry is exposed via a static NodePort (`30001`) and uses plain HTTP. It is not configured for TLS.


  First, run this command to create or update the configuration file. This command will overwrite the file if it exists.

   1 sudo bash -c 'echo "{ \\"insecure-registries\\": [\\"r0.lan.buetow.org:30001\\",\\"r1.lan.buetow.org:30001\\",\\"r2.lan.buetow.org:30001\\"] }" > /etc/docker/daemon.json'

  After running that command, you need to restart your Docker daemon for the changes to take effect.

   1 sudo systemctl restart docker


And afterwards I could push the anky-sync-server image.

## K3s Configuration

To use the private registry from within the k3s cluster, you need to configure each k3s node.

### 1. Update /etc/hosts
On each k3s node, you must ensure that `registry.lan.buetow.org` resolves to the node's loopback address. You can do this by adding an entry to the `/etc/hosts` file.

Run the following command, which will add the entry to `r0`, `r1`, and `r2`:
```bash
for node in r0 r1 r2; do ssh root@$node "echo '127.0.0.1 registry.lan.buetow.org' >> /etc/hosts"; done
```

### 2. Configure K3s to trust the insecure registry
You need to configure each k3s node to trust the insecure registry. This is done by creating a `registries.yaml` file in `/etc/rancher/k3s/` on each node.

The following command will create the file and restart the k3s service. You will need to run this for each node (`r0`, `r1`, `r2`):

```bash
ssh root@<node> "echo -e 'mirrors:\n  "registry.lan.buetow.org:30001":\n    endpoint:\n      - "http://localhost:30001"' > /etc/rancher/k3s/registries.yaml && systemctl restart k3s"
```

