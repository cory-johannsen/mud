# Kubernetes Deployment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Deploy the MUD game stack to a Kind-hosted Kubernetes cluster using a Helm chart, with rolling deploys and images pushed to registry.johannsen.cloud:5000.

**Architecture:** A Helm chart under `deployments/k8s/mud/` defines all resources in a `mud` namespace: postgres (Deployment+PVC), migrate (pre-upgrade Job), gameserver (Deployment, ClusterIP), frontend (Deployment, NodePort 4000). Config is passed via `MUD_*` environment variables that Viper picks up as overrides. Secrets are passed at deploy time via `helm --set`, never committed. Kind is configured with `extraPortMappings` to forward host port 4000 to the NodePort.

**Tech Stack:** Kubernetes (Kind), Helm 3, Docker, kubectl, kind CLI, GNU make.

---

### Task 1: Kind cluster config

**Files:**
- Create: `kind-config.yaml` (repo root)

**Step 1: Create kind-config.yaml**

```yaml
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30400
        hostPort: 4000
        protocol: TCP
```

**Step 2: Verify kind is installed**

```bash
kind version
```

Expected: version string like `kind v0.x.x`

**Step 3: Create the cluster**

```bash
kind create cluster --name mud --config kind-config.yaml
```

Expected output ends with: `Have a nice day! 👋`

**Step 4: Verify cluster is running**

```bash
kubectl cluster-info --context kind-mud
```

Expected: two lines showing control plane and CoreDNS URLs.

**Step 5: Delete the cluster (cleanup — Makefile targets come in Task 8)**

```bash
kind delete cluster --name mud
```

**Step 6: Commit**

```bash
git add kind-config.yaml
git commit -m "feat: add Kind cluster config with port mapping for telnet"
```

---

### Task 2: Helm chart skeleton

**Files:**
- Create: `deployments/k8s/mud/Chart.yaml`
- Create: `deployments/k8s/mud/values.yaml`
- Create: `deployments/k8s/mud/values-prod.yaml`

**Step 1: Create Chart.yaml**

```yaml
# deployments/k8s/mud/Chart.yaml
apiVersion: v2
name: mud
description: MUD game server Helm chart
type: application
version: 0.1.0
appVersion: "0.1.0"
```

**Step 2: Create values.yaml**

```yaml
# deployments/k8s/mud/values.yaml
image:
  registry: registry.johannsen.cloud:5000
  tag: latest
  pullPolicy: Always

db:
  name: mud
  user: mud
  password: ""   # required; pass via --set db.password=...

postgres:
  storage: 5Gi

logging:
  level: info
  format: json
```

**Step 3: Create values-prod.yaml**

```yaml
# deployments/k8s/mud/values-prod.yaml
# Production overrides. No secrets here — pass db.password via --set at deploy time.
logging:
  level: info
  format: json
```

**Step 4: Verify helm is installed**

```bash
helm version
```

Expected: version string like `version.BuildInfo{Version:"v3.x.x"...}`

**Step 5: Verify helm lint passes (no templates yet, just skeleton)**

```bash
helm lint deployments/k8s/mud/
```

Expected: `1 chart(s) linted, 0 chart(s) failed`

**Step 6: Commit**

```bash
git add deployments/k8s/mud/
git commit -m "feat: add Helm chart skeleton for k8s deployment"
```

---

### Task 3: Namespace and Secret templates

**Files:**
- Create: `deployments/k8s/mud/templates/namespace.yaml`
- Create: `deployments/k8s/mud/templates/secret.yaml`

**Step 1: Create namespace.yaml**

```yaml
# deployments/k8s/mud/templates/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mud
```

**Step 2: Create secret.yaml**

```yaml
# deployments/k8s/mud/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: mud-credentials
  namespace: mud
type: Opaque
stringData:
  db-user: {{ .Values.db.user | quote }}
  db-password: {{ .Values.db.password | quote }}
  db-name: {{ .Values.db.name | quote }}
```

**Step 3: Verify helm template renders correctly**

```bash
helm template mud deployments/k8s/mud/ --set db.password=testpass
```

Expected: YAML output showing Namespace and Secret with `db-user: "mud"` and `db-password: "testpass"`.

**Step 4: Run helm lint**

```bash
helm lint deployments/k8s/mud/ --set db.password=testpass
```

Expected: `1 chart(s) linted, 0 chart(s) failed`

**Step 5: Commit**

```bash
git add deployments/k8s/mud/templates/
git commit -m "feat: add namespace and secret Helm templates"
```

---

### Task 4: Postgres PVC and resources

**Files:**
- Create: `deployments/k8s/mud/templates/postgres/pvc.yaml`
- Create: `deployments/k8s/mud/templates/postgres/deployment.yaml`
- Create: `deployments/k8s/mud/templates/postgres/service.yaml`

**Step 1: Create postgres/pvc.yaml**

```yaml
# deployments/k8s/mud/templates/postgres/pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  namespace: mud
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .Values.postgres.storage }}
```

**Step 2: Create postgres/deployment.yaml**

```yaml
# deployments/k8s/mud/templates/postgres/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: mud
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          env:
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-user
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-password
            - name: POSTGRES_DB
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-name
          ports:
            - containerPort: 5432
          readinessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - $(POSTGRES_USER)
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 10
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: postgres-data
```

**Step 3: Create postgres/service.yaml**

```yaml
# deployments/k8s/mud/templates/postgres/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: mud
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
      targetPort: 5432
```

**Step 4: Verify**

```bash
helm template mud deployments/k8s/mud/ --set db.password=testpass | grep "kind:"
```

Expected output includes: `kind: Namespace`, `kind: Secret`, `kind: PersistentVolumeClaim`, `kind: Deployment`, `kind: Service`

**Step 5: Commit**

```bash
git add deployments/k8s/mud/templates/postgres/
git commit -m "feat: add postgres PVC, Deployment, and Service Helm templates"
```

---

### Task 5: Migrate Job

**Files:**
- Create: `deployments/k8s/mud/templates/migrate/job.yaml`

**Step 1: Create migrate/job.yaml**

The migrate job uses the frontend image (which contains `/bin/migrate`). It runs as a Helm pre-upgrade hook so migrations always run before new app pods start.

```yaml
# deployments/k8s/mud/templates/migrate/job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: mud-migrate
  namespace: mud
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "0"
    "helm.sh/hook-delete-policy": before-hook-creation
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: OnFailure
      initContainers:
        - name: wait-for-postgres
          image: postgres:16-alpine
          command:
            - sh
            - -c
            - |
              until pg_isready -h postgres.mud.svc.cluster.local -U $(DB_USER); do
                echo "waiting for postgres..."; sleep 2;
              done
          env:
            - name: DB_USER
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-user
      containers:
        - name: migrate
          image: {{ .Values.image.registry }}/mud-frontend:{{ .Values.image.tag }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          entrypoint: ["/bin/migrate"]
          command:
            - /bin/migrate
            - -config
            - /configs/dev.yaml
          env:
            - name: MUD_DATABASE_HOST
              value: postgres.mud.svc.cluster.local
            - name: MUD_DATABASE_PORT
              value: "5432"
            - name: MUD_DATABASE_USER
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-user
            - name: MUD_DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-password
            - name: MUD_DATABASE_NAME
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-name
```

Note: The `entrypoint` field is not valid in Kubernetes — the container command override uses `command` (which maps to Docker `ENTRYPOINT`) and `args` (which maps to Docker `CMD`). Since the image's entrypoint is `/bin/frontend`, we override it with `command: ["/bin/migrate", ...]` to run the migration binary instead.

Fix the job to use correct k8s syntax:

```yaml
          command:
            - /bin/migrate
            - -config
            - /configs/dev.yaml
```

Remove the `entrypoint` line. The `command` field in k8s overrides the Docker `ENTRYPOINT`, so this will run `/bin/migrate` instead of `/bin/frontend`.

**Step 2: Verify helm template**

```bash
helm template mud deployments/k8s/mud/ --set db.password=testpass | grep -A2 "kind: Job"
```

Expected: shows `kind: Job` with hook annotations.

**Step 3: Commit**

```bash
git add deployments/k8s/mud/templates/migrate/
git commit -m "feat: add migrate pre-install/pre-upgrade Job Helm template"
```

---

### Task 6: Gameserver resources

**Files:**
- Create: `deployments/k8s/mud/templates/gameserver/deployment.yaml`
- Create: `deployments/k8s/mud/templates/gameserver/service.yaml`

**Step 1: Create gameserver/deployment.yaml**

```yaml
# deployments/k8s/mud/templates/gameserver/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gameserver
  namespace: mud
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app: gameserver
  template:
    metadata:
      labels:
        app: gameserver
    spec:
      containers:
        - name: gameserver
          image: {{ .Values.image.registry }}/mud-gameserver:{{ .Values.image.tag }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - containerPort: 50051
          env:
            - name: MUD_GAMESERVER_GRPC_HOST
              value: "0.0.0.0"
            - name: MUD_GAMESERVER_GRPC_PORT
              value: "50051"
            - name: MUD_DATABASE_HOST
              value: postgres.mud.svc.cluster.local
            - name: MUD_DATABASE_PORT
              value: "5432"
            - name: MUD_DATABASE_USER
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-user
            - name: MUD_DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-password
            - name: MUD_DATABASE_NAME
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-name
            - name: MUD_LOGGING_LEVEL
              value: {{ .Values.logging.level | quote }}
            - name: MUD_LOGGING_FORMAT
              value: {{ .Values.logging.format | quote }}
          readinessProbe:
            tcpSocket:
              port: 50051
            initialDelaySeconds: 5
            periodSeconds: 5
```

**Step 2: Create gameserver/service.yaml**

```yaml
# deployments/k8s/mud/templates/gameserver/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: gameserver
  namespace: mud
spec:
  selector:
    app: gameserver
  ports:
    - port: 50051
      targetPort: 50051
```

**Step 3: Verify helm template**

```bash
helm template mud deployments/k8s/mud/ --set db.password=testpass | grep "name: gameserver" -A2
```

**Step 4: Commit**

```bash
git add deployments/k8s/mud/templates/gameserver/
git commit -m "feat: add gameserver Deployment and Service Helm templates"
```

---

### Task 7: Frontend resources

**Files:**
- Create: `deployments/k8s/mud/templates/frontend/deployment.yaml`
- Create: `deployments/k8s/mud/templates/frontend/service.yaml`

**Step 1: Create frontend/deployment.yaml**

```yaml
# deployments/k8s/mud/templates/frontend/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: mud
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      containers:
        - name: frontend
          image: {{ .Values.image.registry }}/mud-frontend:{{ .Values.image.tag }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - containerPort: 4000
          env:
            - name: MUD_GAMESERVER_GRPC_HOST
              value: gameserver.mud.svc.cluster.local
            - name: MUD_GAMESERVER_GRPC_PORT
              value: "50051"
            - name: MUD_DATABASE_HOST
              value: postgres.mud.svc.cluster.local
            - name: MUD_DATABASE_PORT
              value: "5432"
            - name: MUD_DATABASE_USER
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-user
            - name: MUD_DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-password
            - name: MUD_DATABASE_NAME
              valueFrom:
                secretKeyRef:
                  name: mud-credentials
                  key: db-name
            - name: MUD_LOGGING_LEVEL
              value: {{ .Values.logging.level | quote }}
            - name: MUD_LOGGING_FORMAT
              value: {{ .Values.logging.format | quote }}
          readinessProbe:
            tcpSocket:
              port: 4000
            initialDelaySeconds: 5
            periodSeconds: 5
```

**Step 2: Create frontend/service.yaml**

```yaml
# deployments/k8s/mud/templates/frontend/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: frontend
  namespace: mud
spec:
  type: NodePort
  selector:
    app: frontend
  ports:
    - port: 4000
      targetPort: 4000
      nodePort: 30400
```

**Step 3: Run final helm lint**

```bash
helm lint deployments/k8s/mud/ --set db.password=testpass
```

Expected: `1 chart(s) linted, 0 chart(s) failed`

**Step 4: Verify all resources render**

```bash
helm template mud deployments/k8s/mud/ --set db.password=testpass | grep "^kind:"
```

Expected output (order may vary):
```
kind: Namespace
kind: Secret
kind: PersistentVolumeClaim
kind: Deployment
kind: Service
kind: Job
kind: Deployment
kind: Service
kind: Deployment
kind: Service
```

**Step 5: Commit**

```bash
git add deployments/k8s/mud/templates/frontend/
git commit -m "feat: add frontend Deployment and NodePort Service Helm templates"
```

---

### Task 8: Makefile targets and helper scripts

**Files:**
- Modify: `Makefile`
- Create: `deployments/k8s/mud/scripts/cluster-up.sh`
- Create: `deployments/k8s/mud/scripts/cluster-down.sh`

**Step 1: Create cluster-up.sh**

```bash
#!/usr/bin/env bash
# deployments/k8s/mud/scripts/cluster-up.sh
# Creates the Kind cluster. Run once before first deploy.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

echo "Creating Kind cluster 'mud'..."
kind create cluster --name mud --config "${REPO_ROOT}/kind-config.yaml"
echo "Cluster ready."
kubectl cluster-info --context kind-mud
```

**Step 2: Create cluster-down.sh**

```bash
#!/usr/bin/env bash
# deployments/k8s/mud/scripts/cluster-down.sh
# Destroys the Kind cluster and all its data.
set -euo pipefail

echo "Deleting Kind cluster 'mud'..."
kind delete cluster --name mud
echo "Cluster deleted."
```

**Step 3: Make scripts executable**

```bash
chmod +x deployments/k8s/mud/scripts/cluster-up.sh
chmod +x deployments/k8s/mud/scripts/cluster-down.sh
```

**Step 4: Add Makefile targets**

Add the following block to `Makefile` after the `# Docker` section. The variables `DB_USER`, `DB_PASSWORD`, and `IMAGE_TAG` are passed in from the command line (e.g. `make helm-install DB_PASSWORD=secret`).

```makefile
# Kubernetes / Kind
REGISTRY := registry.johannsen.cloud:5000
DB_USER  := mud
DB_PASSWORD :=
IMAGE_TAG := $(shell git rev-parse --short HEAD 2>/dev/null || echo latest)
HELM_CHART := deployments/k8s/mud
HELM_RELEASE := mud
HELM_VALUES := $(HELM_CHART)/values-prod.yaml

kind-up:
	deployments/k8s/mud/scripts/cluster-up.sh

kind-down:
	deployments/k8s/mud/scripts/cluster-down.sh

docker-push:
	docker build -t $(REGISTRY)/mud-gameserver:$(IMAGE_TAG) -f deployments/docker/Dockerfile.gameserver .
	docker push $(REGISTRY)/mud-gameserver:$(IMAGE_TAG)
	docker build -t $(REGISTRY)/mud-frontend:$(IMAGE_TAG) -f deployments/docker/Dockerfile.frontend .
	docker push $(REGISTRY)/mud-frontend:$(IMAGE_TAG)

helm-install:
	helm install $(HELM_RELEASE) $(HELM_CHART) \
		--values $(HELM_VALUES) \
		--set db.user=$(DB_USER) \
		--set db.password=$(DB_PASSWORD) \
		--set image.tag=$(IMAGE_TAG)

helm-upgrade:
	helm upgrade $(HELM_RELEASE) $(HELM_CHART) \
		--values $(HELM_VALUES) \
		--set db.user=$(DB_USER) \
		--set db.password=$(DB_PASSWORD) \
		--set image.tag=$(IMAGE_TAG)

helm-uninstall:
	helm uninstall $(HELM_RELEASE)

k8s-up: kind-up docker-push helm-install

k8s-down: helm-uninstall kind-down

k8s-redeploy: docker-push helm-upgrade
```

Also update the `.PHONY` line at the top of the Makefile to include the new targets:

```makefile
.PHONY: build test test-fast test-postgres test-cover migrate run-dev docker-up docker-down clean lint proto build-import-content kind-up kind-down docker-push helm-install helm-upgrade helm-uninstall k8s-up k8s-down k8s-redeploy
```

**Step 5: Verify make targets parse**

```bash
make --dry-run kind-up 2>&1 | head -5
make --dry-run docker-push IMAGE_TAG=test123 2>&1 | head -5
make --dry-run helm-install DB_PASSWORD=secret 2>&1 | head -5
```

Expected: each prints the commands that would run, no errors.

**Step 6: Commit**

```bash
git add Makefile deployments/k8s/mud/scripts/
git commit -m "feat: add k8s Makefile targets and cluster lifecycle scripts"
```

---

### Task 9: End-to-end verification

This task verifies the full deployment works. It requires:
- `registry.johannsen.cloud:5000` to be reachable and accepting pushes
- `kind`, `kubectl`, `helm` installed
- A `DB_PASSWORD` value

**Step 1: Build and push images**

```bash
make docker-push IMAGE_TAG=smoke-test
```

Expected: both images built and pushed successfully.

**Step 2: Create Kind cluster**

```bash
make kind-up
```

Expected: cluster created, `kubectl cluster-info --context kind-mud` succeeds.

**Step 3: Install the Helm chart**

```bash
make helm-install DB_PASSWORD=mud IMAGE_TAG=smoke-test
```

Expected: `NAME: mud`, `STATUS: deployed`

**Step 4: Watch pods come up**

```bash
kubectl get pods -n mud -w
```

Expected: all pods reach `Running` status. The `mud-migrate` Job completes (`Completed`).

**Step 5: Verify game is reachable**

```bash
telnet localhost 4000
```

Expected: MUD game welcome banner appears.

**Step 6: Verify rolling deploy works**

```bash
make docker-push IMAGE_TAG=smoke-test-2
make helm-upgrade DB_PASSWORD=mud IMAGE_TAG=smoke-test-2
kubectl rollout status deployment/gameserver -n mud
kubectl rollout status deployment/frontend -n mud
```

Expected: both rollouts complete successfully. Old pods replaced with no downtime.

**Step 7: Tear down**

```bash
make k8s-down
```

Expected: Helm release uninstalled, Kind cluster deleted.

**Step 8: Commit (push everything)**

```bash
git push origin main
```
