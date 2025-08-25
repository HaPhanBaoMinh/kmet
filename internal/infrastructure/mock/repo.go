package mock

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/HaPhanBaoMinh/kmet/internal/domain"
)

type Repo struct {
	start time.Time
	rnd   *rand.Rand
}

func New() *Repo {
	src := rand.NewSource(time.Now().UnixNano())
	return &Repo{start: time.Now(), rnd: rand.New(src)}
}

func (r *Repo) ListNodes(ctx context.Context) ([]domain.NodeMetric, error) {
	base := []string{"ip-10-0-1-5", "ip-10-0-1-12", "ip-10-0-2-3", "ip-10-0-2-7", "ip-10-0-3-2"}
	out := make([]domain.NodeMetric, 0, len(base))
	for i, n := range base {
		c := clamp01(0.45 + 0.25*(r.noise(i)))
		m := clamp01(0.42 + 0.28*(r.noise(i+10)))
		out = append(out, domain.NodeMetric{
			NodeName: n,
			CPUUsed:  c,
			MEMUsed:  m,
			Pods:     70 + i*5 + int(10*r.rnd.Float64()),
			K8sVer:   "1.29",
			CPUTrend: trendFrom(c, 60, r.rnd),
			MEMTrend: trendFrom(m, 60, r.rnd),
		})
	}
	return out, nil
}

func matchesSelector(podName, container, selector string) bool {
	sel := strings.TrimSpace(selector)
	if sel == "" {
		return true
	}
	parts := strings.Split(sel, ",")
	for _, part := range parts {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		if strings.Contains(t, "=") {
			kv := strings.SplitN(t, "=", 2)
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			switch key {
			case "app", "component", "name":
				if !(strings.EqualFold(container, val) || strings.Contains(podName, val)) {
					return false
				}
			default:
				if !(strings.Contains(podName, val) || strings.Contains(container, val)) {
					return false
				}
			}
		} else {
			if !(strings.Contains(podName, t) || strings.Contains(container, t)) {
				return false
			}
		}
	}
	return true
}

func (r *Repo) ListPods(ctx context.Context, ns string, selector string) ([]domain.PodMetric, error) {
	pods := []struct {
		name, ctn, node string
	}{{"api-7cfb9d9c9c-9tghd", "api", "ip-10-0-1-5"},
		{"api-7cfb9d9c9c-sj2lq", "api", "ip-10-0-1-12"},
		{"worker-5f7dcbffd6-2jqkz", "worker", "ip-10-0-2-3"},
		{"cart-6d79f8b5f7-m2x8l", "cart", "ip-10-0-2-7"},
	}
	var out []domain.PodMetric
	for i, p := range pods {
		// selector mock
		if !matchesSelector(p.name, p.ctn, selector) {
			continue
		}
		cpu := int(80 + 60*r.rnd.Float64()) // m
		mem := int64(500*1024*1024 + int64(300*1024*1024*r.rnd.Float64()))
		out = append(out, domain.PodMetric{
			Namespace:   coalesce(ns, "default"),
			PodName:     p.name,
			Container:   p.ctn,
			NodeName:    p.node,
			CPUm:        cpu,
			MemBytes:    mem,
			CPUReqm:     100,
			MemReqBytes: 256 * 1024 * 1024,
			Ready:       "1/1",
			Phase:       "Running",
			CPUTrend:    trendFrom(float64(cpu)/500.0, 60, r.rnd),                // normalize ~0..1
			MemTrend:    trendFrom(float64(mem)/(1.2*1024*1024*1024), 60, r.rnd), // ~0..1
		})
		if i == 0 {
			out[i].CPUm = 120
			out[i].MemBytes = 612 * 1024 * 1024
		}
	}
	return out, nil
}

func (r *Repo) StreamLogs(ctx context.Context, t domain.LogsTarget) (<-chan domain.LogLine, error) {
	ch := make(chan domain.LogLine, 100)
	go func() {
		defer close(ch)
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case ts := <-tick.C:
				i++
				level := "INFO"
				msg := "request ok"
				if i%13 == 0 {
					level = "WARN"
					msg = "queue lag=233ms"
				}
				if i%37 == 0 {
					level = "ERROR"
					msg = "db timeout op=save_order retry=1"
				}
				ch <- domain.LogLine{
					Time:   ts,
					Level:  level,
					Text:   msg,
					Source: fmt.Sprintf("%s/%s", t.Name, coalesce(t.Container, "api")),
				}
			}
		}
	}()
	return ch, nil
}

// helpers
func trendFrom(base float64, n int, r *rand.Rand) domain.Trend {
	v := clamp01(base)
	s := make([]float64, n)
	for i := range s {
		v += (r.Float64() - 0.5) * 0.05
		v = clamp01(v)
		s[i] = v
	}
	return domain.Trend{Samples: s, Window: time.Minute}
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func (r *Repo) noise(seed int) float64 {
	return (mathSin(float64(time.Since(r.start)/time.Second)) + float64(seed%3)*0.1 + r.rnd.Float64()*0.2)
}

func coalesce(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// tiny sin approx so we don't import math just for a feel
func mathSin(x float64) float64 {
	// Taylor up to x^5 near 0, crude but ok for mock wobble
	xx := x - float64(int(x/6.283185))*6.283185
	return xx - (xx*xx*xx)/6 + (xx*xx*xx*xx*xx)/120
}

func (r *Repo) ListNamespaces(ctx context.Context) ([]string, error) {
	return []string{"default", "staging", "kube-system"}, nil
}
