package domain

import "time"

type Trend struct {
	Samples []float64 // normalized 0..1
	Window  time.Duration
}

type PodMetric struct {
	Namespace   string
	PodName     string
	Container   string
	NodeName    string
	CPUm        int    // millicores used
	MemBytes    int64  // bytes used
	CPUReqm     int    // request cpu
	MemReqBytes int64  // request mem
	Ready       string // "1/1", "2/3", ...
	Phase       string // Running, Pending...
	CPUTrend    Trend
	MemTrend    Trend
}

type NodeMetric struct {
	NodeName string
	CPUUsed  float64 // 0..1
	MEMUsed  float64 // 0..1
	Pods     int
	K8sVer   string
	CPUTrend Trend
	MEMTrend Trend
}

type LogLine struct {
	Time   time.Time
	Level  string // INFO/WARN/ERROR
	Text   string
	Source string // pod/container or owner
}
