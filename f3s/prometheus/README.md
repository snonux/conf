# Prometheus installation

This directory contains the configuration to deploy the kube-prometheus-stack, with persistent storage for Prometheus and Grafana, and a Grafana ingress.

## Prerequisites

1.  Create the monitoring namespace:

    ```sh
    kubectl create ns monitoring
    ```

2.  Add the Prometheus Helm chart repository:

    ```sh
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    ```

3.  Create the directories on your NFS server:

    ```sh
    mkdir -p /data/nfs/k3svolumes/prometheus/data
    mkdir -p /data/nfs/k3svolumes/grafana/data
    ```

## Automation with Justfile

A `Justfile` is provided to automate the installation and uninstallation process.

-   To install everything, run:

    ```sh
    just install
    ```

-   To uninstall everything, run:

    ```sh
    just uninstall
    ```
