# kmet

## Overview
`kmet` is a fast terminal UI for monitoring Kubernetes. It shows live Pod and Node metrics with tiny trend charts and lets you switch namespaces, sort by CPU or memory, and inspect details — all from your terminal.

## Features
- Live views: Pods and Nodes (switch with Tab)
- CPU and Memory numbers with bars and sparkline trends
- Namespace picker overlay
- Sort by CPU or Memory
- Info panel with utilization vs requests and max
- Mock mode for quick demo without a cluster

## How to use
1) Install (Go 1.22+):
```bash
go install github.com/HaPhanBaoMinh/kmet/cmd/kmet@latest
```

2) Run against a cluster:
```bash
# macOS/Linux
kmet -kubeconfig ~/.kube/config -context <your-context>

# Windows
kmet -kubeconfig %USERPROFILE%\.kube\config -context <your-context>
```

3) Try the demo (no cluster needed):
```bash
kmet -mock
```

### Keyboard shortcuts
- Up/Down: move selection
- Tab: switch Pods/Nodes view
- n: open namespace picker
- i: toggle info panel
- s: toggle sort (CPU/MEM)
- q or Ctrl+C: quit (Esc also closes panels)

### Flags
- `-mock`: use built‑in demo data
- `-kubeconfig <path>`: kubeconfig path (defaults to your home directory)
- `-context <name>`: kube context to use

### Notes
- For real metrics, your cluster should expose metrics via metrics.k8s.io (e.g. metrics‑server). Without it, usage bars may show zeros.
- Your kubeconfig/user needs permission to list pods/nodes and read pod logs.
