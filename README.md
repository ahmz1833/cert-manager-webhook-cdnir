# cert-manager ACME DNS-01 Webhook for CDN.ir (Asiatech Cloud)

[![CI](https://github.com/ahmz1833/cert-manager-webhook-cdnir/actions/workflows/ci.yml/badge.svg)](https://github.com/ahmz1833/cert-manager-webhook-cdnir/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This webhook solver allows [cert-manager](https://cert-manager.io/) to solve ACME DNS-01 challenges using the **CDN.ir / Asiatech Cloud DNS API** (`bss.asiatech.cloud`). Designed to mirror the Cloudflare cert-manager webhook interface.

## Features
- Fully compatible with `cert-manager` v1.x+.
- Automatically handles ACME challenge TXT records (`_acme-challenge.<domain>`).
- Supports wildcard certificates (e.g. `*.example.ir`) and apex/root certificates.
- Resolves API tokens securely from Kubernetes Secrets or Pod Environment variables.
- Auto clean up of challenge records upon issuance or failure.

---

## 1. Prerequisites
- A Kubernetes cluster with `cert-manager` installed (v1.12+ recommended).
- A valid CDN.ir account and API Bearer Token.

---

## 2. Quick Start / Installation

### Method A: Install via Helm (Recommended)
```bash
helm install cert-manager-webhook-cdnir ./deploy/helm \
  --namespace cert-manager \
  --set groupName="acme.cdn.ir"
```

### Method B: Deploy Manifests Manually
```bash
# 1. Create Secret with CDN.ir Token
kubectl create secret generic cdnir-api-token \
  --namespace=cert-manager \
  --from-literal=token="YOUR_CDNIR_JWT_TOKEN"

# 2. Deploy Webhook
kubectl apply -f deploy/manifests/rbac.yaml
kubectl apply -f deploy/manifests/deployment.yaml
kubectl apply -f deploy/manifests/service.yaml
```

---

## 3. Configure ClusterIssuer / Issuer

Create a `ClusterIssuer` (e.g., Let's Encrypt Production):

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-cdnir
spec:
  acme:
    email: admin@example.ir
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-cdnir-account-key
    solvers:
      - dns01:
          webhook:
            groupName: acme.cdn.ir
            solverName: cdnir-solver
            config:
              apiEndpoint: "https://bss.asiatech.cloud/api/v3"
              apiTokenSecretRef:
                name: cdnir-api-token
                key: token
```

Apply the ClusterIssuer:
```bash
kubectl apply -f deploy/manifests/cluster-issuer.yaml
```

---

## 4. Request a Wildcard Certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-ir-wildcard
  namespace: default
spec:
  secretName: example-ir-tls
  issuerRef:
    name: letsencrypt-cdnir
    kind: ClusterIssuer
  dnsNames:
    - "example.ir"
    - "*.example.ir"
```

Apply and verify:
```bash
kubectl apply -f certificate.yaml
kubectl get certificate example-ir-wildcard
```

---

## 5. Building the Docker Image

To build and tag the container image locally:
```bash
docker build -t ghcr.io/ahmz1833/cert-manager-webhook-cdnir:latest .
```

---

## License

Apache License 2.0. Copyright (c) 2026 AH MZ <ahmz.ut@gmail.com>.
