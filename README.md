# Ephemeral Environment Operator 🚀

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.28%2B-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io/)
[![CRD Version](https://img.shields.io/badge/CRD-v1alpha1-blueviolet)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **Production-grade Kubernetes Operator & Controller-Runtime** engineered for **Internal Developer Platforms (IDP)**. Dynamically spins up, secures, and automatically garbage-collects isolated full-stack **Ephemeral Preview Environments** on every Pull Request.

---

## 🏗️ Architecture & Lifecycle Workflow

```mermaid
sequenceDiagram
    autonumber
    actor Dev as Developer / GitHub PR
    participant API as K8s API Server
    participant Op as Ephemeral-Env Operator
    participant K8s as Cluster Resources
    participant DNS as Ingress / External DNS

    Dev->>API: 1. PR #104 opened (Create PreviewEnvironment CR)
    API->>Op: 2. Reconcile Request triggered
    Op->>K8s: 3. Provision dedicated namespace: preview-pr-104
    Op->>K8s: 4. Inject ResourceQuota & LimitRanges (Cost Control)
    Op->>K8s: 5. Apply Zero-Trust NetworkPolicy (Namespace Isolation)
    Op->>K8s: 6. Spin up Ephemeral Mock DB (Postgres/Redis emptyDir)
    Op->>K8s: 7. Deploy microservice workloads & ClusterIP Services
    Op->>DNS: 8. Configure Ingress (https://pr-104.preview.cloudnative.io)
    Op->>API: 9. Update CR Status (Phase: Ready, Endpoints active)
    Note over Op,K8s: Background TTL Worker monitors lifespan (e.g. 4h)
    Dev->>API: 10. PR merged/closed OR TTL Expired
    Op->>K8s: 11. Graceful Finalizer teardown (Purge namespace & routes)
```

---

## 🌟 Key Capabilities

* **⚡ Declarative CRD (`PreviewEnvironment`):** Developers declare isolated environments via GitOps or GitHub Actions PR triggers.
* **🛡️ Zero-Trust Network Isolation:** Automatically enforces default-deny ingress policies between preview environments to prevent noisy neighbors and data leakage.
* **💰 FinOps Guardrails & ResourceQuotas:** Enforces CPU, Memory, and Pod bounds per environment to eliminate runaway cloud costs.
* **⏱️ Automated TTL Garbage Collection:** Native reconciler requeue + background scanner automatically terminates environments when their TTL (e.g. `2h`, `24h`) expires.
* **🗄️ Ephemeral State Provisioner:** Provisions on-demand mock databases (`Postgres`, `Redis`, `MongoDB`, `MySQL`) using in-memory or emptyDir storage.
* **🌐 Dynamic Ingress Templating:** Computes subdomains based on templates (`pr-{{.PRNumber}}.preview.example.com`) with automated TLS secret generation.
* **🔒 Validating Admission Webhook:** Rejects invalid PR numbers, illegal branch ref naming, and TTL policy violations exceeding company limits (>72h).
* **📊 Prometheus Metrics Exporter:** Exposes operational metrics (`ephemeral_env_created_total`, `ephemeral_env_active_gauge`, `ephemeral_env_destroyed_total`).

---

## 📋 Custom Resource Definition (`PreviewEnvironment`)

```yaml
apiVersion: idp.cloudnative.io/v1alpha1
kind: PreviewEnvironment
metadata:
  name: preview-pr-104
spec:
  branch: feature/checkout-v2
  prNumber: 104
  repository: github.com/ekrmcakir/ecommerce-core
  author: ekrmcakir
  ttl: 4h
  autoCleanup: true
  isolatedNetwork: true
  ingress:
    enabled: true
    hostTemplate: "pr-{{.PRNumber}}.preview.cloudnative.io"
    ingressClassName: nginx
    path: /
    tls: true
  resources:
    cpuLimit: "3000m"
    memoryLimit: "6Gi"
    maxPods: 20
  mockDatabase:
    engine: postgres
    databaseName: checkout_preview
    ephemeral: true
  workloads:
  - name: checkout-api
    image: ghcr.io/ekrmcakir/checkout-api:v2.4.0
    port: 8080
    replicas: 2
    env:
    - name: DB_HOST
      value: database
    - name: DB_NAME
      value: checkout_preview
    - name: DB_USER
      value: preview
    - name: DB_PASSWORD
      value: previewpass
```

---

## 🚀 Quickstart & Local Installation

### Prerequisites
* Go 1.24+
* Docker / Podman
* `kubectl` and a test cluster (Kind / Minikube / EKS)

### 1. Clone & Build
```bash
git clone https://github.com/ekrmcakir/ephemeral-env-operator.git
cd ephemeral-env-operator

# Run tests
make test

# Build binary
make build
```

### 2. Install CRDs & Deploy Operator
```bash
# Register CRDs
make install

# Run controller locally against your current kubeconfig:
make run
```

### 3. Deploy a Sample Preview Environment
```bash
kubectl apply -f config/samples/full-preview-env.yaml

# Watch environment provisioning in real time
kubectl get previewenvironments.idp.cloudnative.io -w
```

Output:
```text
NAME             PR    BRANCH                PHASE   NAMESPACE        TTL   EXPIRESAT              AGE
preview-pr-104   104   feature/checkout-v2   Ready   preview-pr-104   4h    2026-10-16T22:15:00Z   25s
```

---

## 📈 Observability & Prometheus Metrics

The operator exposes Prometheus metrics on `:8080/metrics`:

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `ephemeral_env_created_total` | Counter | Total preview environments created |
| `ephemeral_env_destroyed_total` | Counter | Total preview environments purged via TTL or PR close |
| `ephemeral_env_active_gauge` | Gauge | Currently active ephemeral environments running in cluster |

---

## 🛡️ Security & Governance Guardrails

1. **Restricted Pod Security Standard:** Provisioned namespaces enforce Kubernetes `pod-security.kubernetes.io/enforce: restricted`.
2. **Admission Webhook Checks:**
   * Prevents mutation of immutable fields (`spec.prNumber`).
   * Caps maximum TTL duration to `72h`.
   * Enforces container port boundaries `[1, 65535]`.

---

## 📜 License
Released under the [MIT License](LICENSE). Built by **Ekrem Cakir** (@ekrmcakir).
