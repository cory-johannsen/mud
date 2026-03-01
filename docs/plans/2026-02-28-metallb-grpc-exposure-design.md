# MetalLB + External gRPC Exposure Design

**Date:** 2026-02-28

## Goal

Expose the gRPC game server and Telnet frontend to native clients (Go, mobile, desktop, future GUI) via stable LoadBalancer IPs using MetalLB in the Kind cluster, replacing the current ClusterIP/NodePort setup.

## Constraints

- Plaintext only (no TLS) — Kind cluster is local.
- Native gRPC clients only — no browser/gRPC-web support needed.
- Both services migrate to MetalLB to give clients stable IPs that survive pod restarts.

## Architecture

Install MetalLB in the Kind cluster. Change both services to `type: LoadBalancer`. MetalLB assigns stable cluster-local IPs from an IPAddressPool backed by the Docker bridge subnet. Clients connect directly to those IPs.

**Services after migration:**
- `gameserver`: `LoadBalancer` on port `50051` → stable IP (e.g. `172.18.0.100:50051`)
- `frontend`: `LoadBalancer` on port `4000` → stable IP (e.g. `172.18.0.101:4000`)

## Components

1. **MetalLB installation** — applied to the Kind cluster via `kubectl apply`. One-time cluster setup; not part of the app Helm chart. Stored as `deployments/k8s/metallb/` manifests.
2. **IPAddressPool** — MetalLB CRD declaring IP range from the Kind Docker bridge subnet (e.g. `172.18.0.100-172.18.0.110`).
3. **L2Advertisement** — MetalLB CRD enabling Layer 2 / ARP mode (correct for Kind; no BGP).
4. **Service manifest changes** — `type: LoadBalancer` in both service Helm templates; remove `nodePort` from frontend.
5. **Helm values** — IP pool range configurable in `values-prod.yaml`.
6. **Makefile** — Add `k8s-metallb` target to install MetalLB CRDs + config; document in setup instructions.

## Data Flow

```
Native gRPC client (Go/mobile/desktop)
  → 172.18.0.100:50051 (MetalLB LoadBalancer IP)
  → gameserver pod :50051
  → bidirectional streaming Session RPC

Telnet client
  → 172.18.0.101:4000 (MetalLB LoadBalancer IP)
  → frontend pod :4000
```

## Error Handling

- MetalLB not installed → services stay `Pending` — obvious failure mode caught by `kubectl get svc`.
- IP pool exhaustion → not a concern (10 IPs available, 2 used).
- Kind cluster restart → Docker bridge subnet is stable; LoadBalancer IPs reassigned from same pool.

## Testing

- `kubectl get svc -n mud` shows `EXTERNAL-IP` assigned (not `<pending>`)
- `grpcurl -plaintext <IP>:50051 list` lists the game service
- Telnet to `<IP>:4000` connects and game flow works end-to-end
