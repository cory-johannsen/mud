# Kubernetes Deployment Design

Date: 2026-02-27

## Goal

Deploy the MUD game stack to a Kind-hosted Kubernetes cluster with rolling deploy support, using a Helm chart for manifest management and a private registry for images.

## Stack

- **Cluster:** Kind (local)
- **Image registry:** `registry.johannsen.cloud:5000`
- **Manifest management:** Helm chart
- **Secrets:** Passed at deploy time via `helm --set`, never committed

## Repository Structure

```
deployments/
├── docker/                          # existing, unchanged
└── k8s/
    └── mud/                         # Helm chart root
        ├── Chart.yaml
        ├── values.yaml              # defaults
        ├── values-prod.yaml         # production overrides (no secrets, committed)
        ├── templates/
        │   ├── namespace.yaml
        │   ├── secret.yaml          # mud-credentials, populated via --set at deploy time
        │   ├── postgres/
        │   │   ├── deployment.yaml  # Recreate strategy
        │   │   ├── service.yaml     # ClusterIP :5432
        │   │   └── pvc.yaml         # ReadWriteOnce, local-path provisioner
        │   ├── migrate/
        │   │   └── job.yaml         # pre-upgrade hook Job, backoffLimit: 3
        │   ├── gameserver/
        │   │   ├── deployment.yaml  # RollingUpdate, maxUnavailable:0 maxSurge:1
        │   │   └── service.yaml     # ClusterIP :50051 (internal gRPC only)
        │   └── frontend/
        │       ├── deployment.yaml  # RollingUpdate, maxUnavailable:0 maxSurge:1
        │       └── service.yaml     # NodePort :4000 → nodePort:30400
        └── scripts/
            ├── cluster-up.sh        # create Kind cluster + install chart
            └── cluster-down.sh      # destroy Kind cluster

kind-config.yaml                     # repo root — extraPortMappings host:4000 → 30400
```

## Kubernetes Resources

### postgres
- `Deployment` (1 replica, `Recreate` update strategy)
- `ClusterIP Service` (port 5432)
- `PersistentVolumeClaim` (`ReadWriteOnce`, local-path provisioner)

### migrate
- `Job` (Helm pre-upgrade hook, `backoffLimit: 3`)
- Init container polls `pg_isready` before running migrations
- Stays in cluster after completion for log inspection

### gameserver
- `Deployment` (1 replica, `RollingUpdate` maxUnavailable:0 maxSurge:1)
- `ClusterIP Service` (port 50051, internal only)

### frontend
- `Deployment` (1 replica, `RollingUpdate` maxUnavailable:0 maxSurge:1)
- `NodePort Service` (containerPort 4000 → nodePort 30400)

### Secrets
- Single `Secret` named `mud-credentials` in namespace `mud`
- Contains: `db-user`, `db-password`
- All services that need DB access mount it as env vars
- Populated at deploy time: `helm install --set db.user=mud --set db.password=xxx`

### Namespace
- All resources deployed to `mud` namespace

## Image Tagging & Rolling Deploys

Images tagged with git SHA:
```
registry.johannsen.cloud:5000/mud-gameserver:<short-sha>
registry.johannsen.cloud:5000/mud-frontend:<short-sha>
```

`values.yaml` defaults `image.tag: latest`. `values-prod.yaml` does not pin the tag — the SHA is passed at deploy time:
```bash
make helm-upgrade DB_PASSWORD=xxx IMAGE_TAG=$(git rev-parse --short HEAD)
```

The migrate Job runs as a `helm.sh/hook: pre-upgrade` hook so DB schema is always up to date before new app pods start.

## Makefile Targets

```makefile
# Kind cluster lifecycle
kind-up          # create Kind cluster from kind-config.yaml
kind-down        # delete Kind cluster

# Helm chart lifecycle
helm-install     # helm install mud deployments/k8s/mud
helm-upgrade     # helm upgrade mud deployments/k8s/mud
helm-uninstall   # helm uninstall mud

# Image build and push
docker-push      # build and push all images to registry.johannsen.cloud:5000

# Convenience
k8s-up           # kind-up + docker-push + helm-install
k8s-down         # helm-uninstall + kind-down
k8s-redeploy     # docker-push + helm-upgrade (cluster already running)
```

Secrets passed as Makefile variables:
```bash
make helm-install DB_USER=mud DB_PASSWORD=secret IMAGE_TAG=abc1234
make k8s-up DB_USER=mud DB_PASSWORD=secret IMAGE_TAG=abc1234
```

## Kind Cluster Configuration

`kind-config.yaml` at repo root:
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30400
        hostPort: 4000
        protocol: TCP
```

## Testing / Verification

- `kubectl get pods -n mud` — all pods Running
- `kubectl logs job/mud-migrate -n mud` — migrations applied
- `telnet localhost 4000` — game accessible
- `make k8s-redeploy` with incremented image tag — rolling deploy with zero downtime on frontend/gameserver
