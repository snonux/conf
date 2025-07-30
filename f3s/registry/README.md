# Private Docker Registry

This document describes how to push Docker images to the private registry deployed in your Kubernetes cluster.

## Prerequisites

*   The `infra` namespace must exist in your cluster. If it doesn't, create it with `kubectl create namespace infra`.

*   A running Kubernetes cluster.
*   `kubectl` configured to connect to your cluster.
*   Docker installed and running on your local machine.

## Steps

0.  **Create the registry directory in the NFS share**

1.  **Get the NodePort of the registry service:**

    ```bash
    kubectl get svc docker-registry-service -o jsonpath='{.spec.ports[0].nodePort}'
    ```

2.  **Tag your Docker image:**

    Replace `<your-image>` with the name of your local Docker image and `<node-ip>` with the IP address of any node in your Kubernetes cluster and `<node-port>` with the port obtained in the previous step.

    ```bash
    docker tag <your-image> <node-ip>:<node-port>/<your-image>
    ```

3.  **Push the image to the registry:**

    ```bash
    docker push <node-ip>:<node-port>/<your-image>
    ```

4.  **Pull the image from the registry (from a Kubernetes pod):**

    You can now use the image in your Kubernetes deployments by referencing it as `docker-registry-service:5000/<your-image>`.
