# Audit-HTTP

A custom Kubernetes operator built using Kubebuilder and Go. This operator manages ClusterScan custom resources to perform HTTP checks on a Kubernetes cluster, handling both one-off and scheduled (cron) jobs. The results of these HTTP checks are updated in the ClusterScan status, providing insights into the health and availability of specified endpoints.

## Features

- **Custom Kubernetes Operator**: Leverages Kubebuilder and Go to create a custom operator.
- **ClusterScan Custom Resource**: Manages ClusterScan resources to specify HTTP checks.
- **One-off Jobs**: Creates Kubernetes Jobs to perform single HTTP checks.
- **Cron Jobs**: Creates Kubernetes CronJobs to perform recurring HTTP checks.
- **Status Updates**: Updates the status of ClusterScan resources with the results of the HTTP checks, including the last run time and result message.

## Prerequisites

- Kubernetes cluster
- Kubebuilder
- Go (version 1.16+)
- Docker
- Minikube (for local development and testing)

## Getting Started

1. Clone the Repository

```bash
git clone https://github.com/sttvk/audit-http.git
cd audit-http
```

2. Install the custom resource definitions (CRDs):

```bash
make install
```

3. Build and push the operator image:

```bash
make docker-build docker-push IMG=<your-registry>/audit-http:tag
```

4. Deploy the operator to your cluster:

```bash
make deploy
```

To create a scheduled audit job, use a custom resource with a cron schedule.
