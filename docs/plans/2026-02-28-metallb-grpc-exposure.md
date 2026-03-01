# MetalLB + External gRPC Exposure Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the ClusterIP/NodePort service setup with MetalLB LoadBalancer services so both the gRPC game server (port 50051) and Telnet frontend (port 4000) are reachable at stable cluster-local IPs.

**Architecture:** Install MetalLB in the Kind cluster and configure an IPAddressPool from the Kind Docker bridge subnet (`172.18.0.0/16`). Change both Kubernetes services to `type: LoadBalancer`. Remove the `extraPortMappings` from `kind-config.yaml` since the frontend is no longer a NodePort.

**Tech Stack:** MetalLB v0.14, Kind, Helm, kubectl, Go (no application code changes)

---

## Context for Implementer

### Repo layout (relevant paths)
- `kind-config.yaml` — Kind cluster config; currently has `extraPortMappings: containerPort 30400 → hostPort 4000`
- `deployments/k8s/mud/templates/gameserver/service.yaml` — ClusterIP on 50051 (no external exposure today)
- `deployments/k8s/mud/templates/frontend/service.yaml` — NodePort 30400 → targetPort 4000
- `deployments/k8s/mud/values-prod.yaml` — Helm production values
- `Makefile` — has `kind-up`, `k8s-up`, `k8s-redeploy` targets

### Docker bridge subnet
Kind uses `172.18.0.0/16`. Kind node IPs are assigned from the low end (typically `172.18.0.2` and up). We use `172.18.0.200-172.18.0.210` to avoid conflicts.

### MetalLB version
Use MetalLB v0.14.9 (latest stable as of 2026-02).

---

## Task 1: Create MetalLB install manifests

**Files:**
- Create: `deployments/k8s/metallb/metallb-native.yaml` (downloaded manifest)
- Create: `deployments/k8s/metallb/ipaddresspool.yaml`
- Create: `deployments/k8s/metallb/l2advertisement.yaml`

**Step 1: Download MetalLB native manifest**

```bash
curl -Lo deployments/k8s/metallb/metallb-native.yaml \
  https://raw.githubusercontent.com/metallb/metallb/v0.14.9/config/manifests/metallb-native.yaml
```

**Step 2: Create IPAddressPool**

Create `deployments/k8s/metallb/ipaddresspool.yaml`:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: mud-pool
  namespace: metallb-system
spec:
  addresses:
    - 172.18.0.200-172.18.0.210
```

**Step 3: Create L2Advertisement**

Create `deployments/k8s/metallb/l2advertisement.yaml`:

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: mud-l2
  namespace: metallb-system
spec:
  ipAddressPools:
    - mud-pool
```

**Step 4: Commit**

```bash
git add deployments/k8s/metallb/
git commit -m "feat: add MetalLB install manifests and IP pool config"
```

---

## Task 2: Add Makefile target for MetalLB setup

**Files:**
- Modify: `Makefile`

**Step 1: Read current Makefile k8s section**

Open `Makefile` and find the `k8s-up` and `kind-up` targets.

**Step 2: Add `k8s-metallb` target**

After the `kind-up` target, add:

```makefile
k8s-metallb:
	kubectl apply -f deployments/k8s/metallb/metallb-native.yaml
	kubectl rollout status deployment/controller -n metallb-system --timeout=120s
	kubectl apply -f deployments/k8s/metallb/ipaddresspool.yaml
	kubectl apply -f deployments/k8s/metallb/l2advertisement.yaml
```

**Step 3: Update `k8s-up` to call `k8s-metallb` after `kind-up`**

Change:
```makefile
k8s-up: kind-up docker-push helm-install
```
To:
```makefile
k8s-up: kind-up k8s-metallb docker-push helm-install
```

**Step 4: Update `.PHONY` line** to add `k8s-metallb`.

**Step 5: Verify Makefile parses**

```bash
make -n k8s-metallb
```

Expected: prints the kubectl commands without running them.

**Step 6: Commit**

```bash
git add Makefile
git commit -m "feat: add k8s-metallb Makefile target, wire into k8s-up"
```

---

## Task 3: Change gameserver service to LoadBalancer

**Files:**
- Modify: `deployments/k8s/mud/templates/gameserver/service.yaml`

**Step 1: Read current file**

Current content:
```yaml
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

**Step 2: Replace with LoadBalancer**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: gameserver
  namespace: mud
spec:
  type: LoadBalancer
  selector:
    app: gameserver
  ports:
    - port: 50051
      targetPort: 50051
      protocol: TCP
```

**Step 3: Commit**

```bash
git add deployments/k8s/mud/templates/gameserver/service.yaml
git commit -m "feat: change gameserver service to LoadBalancer"
```

---

## Task 4: Change frontend service to LoadBalancer, remove NodePort

**Files:**
- Modify: `deployments/k8s/mud/templates/frontend/service.yaml`
- Modify: `kind-config.yaml`

**Step 1: Update frontend service**

Replace current NodePort content with:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: frontend
  namespace: mud
spec:
  type: LoadBalancer
  selector:
    app: frontend
  ports:
    - port: 4000
      targetPort: 4000
      protocol: TCP
```

**Step 2: Remove extraPortMappings from kind-config.yaml**

Current `kind-config.yaml` has this block under the control-plane node — remove it entirely:

```yaml
    extraPortMappings:
      - containerPort: 30400
        hostPort: 4000
        protocol: TCP
```

The result should be:

```yaml
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
```

**Step 3: Commit**

```bash
git add deployments/k8s/mud/templates/frontend/service.yaml kind-config.yaml
git commit -m "feat: change frontend service to LoadBalancer, remove NodePort extraPortMappings"
```

---

## Task 5: Apply MetalLB to running cluster and redeploy

> **Note:** This task modifies the live cluster. The cluster must be running (`kubectl cluster-info` succeeds).

**Step 1: Install MetalLB**

```bash
make k8s-metallb
```

Expected output:
```
namespace/metallb-system created (or configured)
...
deployment.apps/controller condition met
ipaddresspool.metallb.io/mud-pool created (or configured)
l2advertisement.metallb.io/mud-l2 created (or configured)
```

**Step 2: Redeploy the Helm chart**

```bash
make k8s-redeploy
```

**Step 3: Verify LoadBalancer IPs are assigned**

```bash
kubectl get svc -n mud
```

Expected — both services show an IP in `EXTERNAL-IP` (not `<pending>`):

```
NAME         TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)           AGE
frontend     LoadBalancer   10.96.x.x      172.18.0.200     4000:xxxxx/TCP    ...
gameserver   LoadBalancer   10.96.x.x      172.18.0.201     50051:xxxxx/TCP   ...
```

**Step 4: Test Telnet connection**

```bash
telnet 172.18.0.200 4000
```

Expected: MUD welcome screen.

**Step 5: Test gRPC connection**

```bash
grpcurl -plaintext 172.18.0.201:50051 list
```

Expected: lists the service (e.g. `game.v1.GameService`).

**Step 6: Commit verification note**

No code changes in this task — just cluster verification. No commit needed.

---

## Task 6: Update FEATURES.md

**Files:**
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Mark the feature complete**

Change:
```
- [ ] Expose GRPC API and GRPC REST proxy to external clients.
```
To:
```
- [x] Expose GRPC API and GRPC REST proxy to external clients.
```

**Step 2: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "feat: mark gRPC external exposure complete in FEATURES.md"
```
