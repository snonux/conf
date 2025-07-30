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
