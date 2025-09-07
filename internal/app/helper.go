// internal/ui/app/helpers.go
package app

// clamp clamps v into [min, max].
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// compute dynamic widths for Pods table based on available total width
func (m *Model) podColWidths(total int) (wPod, wCPU, wCPUBar, wMem, wMemBar, wReady, wNode, wTrend int) {
	// fixed minimums (numbers and labels)
	minPod, minCPU, minMem, minReady, minNode, minTrend := 24, 6, 8, 6, 12, 8

	base := minPod + minCPU + minMem + minReady + minNode + minTrend
	remain := total - base
	if remain < 10 {
		remain = 10
	}

	// allocate flexible space to bars, favor pod name with any remainder
	wCPUBar = remain / 3
	wMemBar = remain / 3
	extra := remain - (wCPUBar + wMemBar)

	wPod = minPod + extra
	wCPU = minCPU
	wMem = minMem
	wReady = minReady
	wNode = minNode
	wTrend = minTrend

	// sanity clamps
	wPod = clamp(wPod, 16, 60)
	wNode = clamp(wNode, 10, 30)
	wCPUBar = clamp(wCPUBar, 6, 40)
	wMemBar = clamp(wMemBar, 6, 40)
	return
}

// compute dynamic widths for Nodes table based on available total width
func (m *Model) nodeColWidths(total int) (wNode, wCPUP, wCPUBar, wMEMP, wMEMBar, wPods, wK8s, wTrend int) {
	minNode, minPct, minPods, minK8s, minTrend := 16, 6, 5, 6, 8
	base := minNode + minPct + minPct + minPods + minK8s + minTrend
	remain := total - base
	if remain < 8 {
		remain = 8
	}

	wCPUBar = remain / 2
	wMEMBar = remain - wCPUBar

	wNode = minNode
	wCPUP = minPct
	wMEMP = minPct
	wPods = minPods
	wK8s = minK8s
	wTrend = minTrend

	// clamps
	wCPUBar = clamp(wCPUBar, 6, 40)
	wMEMBar = clamp(wMEMBar, 6, 40)
	wNode = clamp(wNode, 12, 40)
	return
}
