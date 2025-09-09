package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/HaPhanBaoMinh/kmet/internal/domain"
)

type Repo struct {
	core    *kubernetes.Clientset
	metrics *metricsclient.Clientset

	podTrend  map[string][]float64
	nodeTrend map[string][]float64
}

func New(kubeconfigPath, contextName string) (*Repo, error) {
	cfg, err := loadRESTConfig(kubeconfigPath, contextName)
	cfg.QPS = 30
	cfg.Burst = 60
	if err != nil {
		return nil, err
	}
	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	m, err := metricsclient.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Repo{
		core: core, metrics: m,
		podTrend:  make(map[string][]float64),
		nodeTrend: make(map[string][]float64),
	}, nil
}

func loadRESTConfig(kubeconfigPath, contextName string) (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}

// -------- MetricsRepo --------

func (r *Repo) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := r.core.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		out = append(out, ns.Name)
	}
	// add "all" namespace
	out = append([]string{"all"}, out...)
	return out, nil
}

func (r *Repo) ListPods(ctx context.Context, ns string, selector string) ([]domain.PodMetric, error) {
	opts := metav1.ListOptions{}
	if selector != "" {
		opts.LabelSelector = selector // k8s-standard label selector string
	}

	// if ns "all" -> empty string
	if ns == "all" {
		ns = ""
	}

	// 1) Get pod objects (for Ready/Phase/Node)
	pods, err := r.core.CoreV1().Pods(ns).List(ctx, opts)
	if err != nil {
		return nil, err
	}

	// 2) Get pod usage (metrics.k8s.io). Gracefully degrade if unavailable.
	pms, err := r.metrics.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	})
	if err != nil {
		pms = &metricsv1beta1.PodMetricsList{} // empty => usage stays 0
	}

	// Sum container usage per pod -> map["ns/name"] = ResourceList
	podUsage := map[string]corev1.ResourceList{}
	for _, m := range pms.Items {
		total := corev1.ResourceList{}
		for _, c := range m.Containers {
			for res, q := range c.Usage {
				if cur, ok := total[res]; ok {
					cur.Add(q)
					total[res] = cur
				} else {
					total[res] = q.DeepCopy()
				}
			}
		}
		podUsage[m.Namespace+"/"+m.Name] = total
	}

	out := make([]domain.PodMetric, 0, len(pods.Items))
	for _, p := range pods.Items {
		key := p.Namespace + "/" + p.Name
		u := podUsage[key]

		var cpuMil int64
		var memB int64
		if u != nil {
			if q, ok := u[corev1.ResourceCPU]; ok {
				qq := q // copy to addressable var
				cpuMil = qq.MilliValue()
			}
			if q, ok := u[corev1.ResourceMemory]; ok {
				qq := q
				memB = qq.Value()
			}
		}

		// Ready string "x/y"
		ready := readyStr(p.Status.ContainerStatuses)

		// Pick first container name for UI (you can add a container switch later)
		ctr := ""
		if len(p.Spec.Containers) > 0 {
			ctr = p.Spec.Containers[0].Name
		}

		// Requests of the first container (simple; you can sum across containers later)
		var cpuReqm int
		var memReqBytes int64
		if len(p.Spec.Containers) > 0 {
			req := p.Spec.Containers[0].Resources.Requests
			if cpuQ := req.Cpu(); cpuQ != nil {
				cpuReqm = int(cpuQ.MilliValue())
			}
			if memQ := req.Memory(); memQ != nil {
				memReqBytes = memQ.Value()
			}
		}

		pm := domain.PodMetric{
			Namespace:   p.Namespace,
			PodName:     p.Name,
			Container:   ctr,
			NodeName:    p.Spec.NodeName,
			CPUm:        int(cpuMil),
			MemBytes:    memB,
			CPUReqm:     cpuReqm,
			MemReqBytes: memReqBytes,
			Ready:       ready,
			Phase:       string(p.Status.Phase),
			CPUTrend:    r.appendTrend(r.podTrend, key, normCPU(cpuMil)),
			MemTrend:    r.appendTrend(r.podTrend, key+"-mem", normMem(memB)),
		}
		out = append(out, pm)
	}

	return out, nil
}

func readyStr(sts []corev1.ContainerStatus) string {
	r, t := 0, len(sts)
	for _, s := range sts {
		if s.Ready {
			r++
		}
	}
	if t == 0 {
		t = 1
	}
	return fmt.Sprintf("%d/%d", r, t)
}

func normCPU(mil int64) float64 { // ~scale để vẽ bar đẹp
	// tuỳ cluster, giả sử 500m = 1.0 hiển thị; tránh >1.0
	v := float64(mil) / 500.0
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}
func normMem(b int64) float64 {
	v := float64(b) / (1.2 * 1024 * 1024 * 1024) // ~1.2Gi = 100%
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}

func (r *Repo) appendTrend(store map[string][]float64, key string, v float64) domain.Trend {
	s := store[key]
	s = append(s, v)
	if len(s) > 90 {
		s = s[len(s)-90:]
	} // 90 samples ~ 90 ticks
	store[key] = s
	return domain.Trend{Samples: s, Window: time.Minute}
}
func (r *Repo) ListNodes(ctx context.Context) ([]domain.NodeMetric, error) {
	// 1) Pull node usage from metrics.k8s.io; gracefully degrade if unavailable.
	nms, err := r.metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		nms = &metricsv1beta1.NodeMetricsList{} // empty → usage=0
	}

	usage := map[string]corev1.ResourceList{}
	for _, m := range nms.Items {
		usage[m.Name] = m.Usage
	}

	// 2) List nodes
	nodes, err := r.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := make([]domain.NodeMetric, 0, len(nodes.Items))

	for _, n := range nodes.Items {
		allocCPU := n.Status.Allocatable.Cpu().MilliValue()
		allocMem := n.Status.Allocatable.Memory().Value()

		u := usage[n.Name]

		var uCPU, uMem float64
		if u != nil {
			if q, ok := u[corev1.ResourceCPU]; ok {
				qq := q // make addressable before calling pointer receiver
				den := float64(max64(1, allocCPU))
				uCPU = float64(qq.MilliValue()) / den
			}
			if q, ok := u[corev1.ResourceMemory]; ok {
				qq := q
				den := float64(max64(1, allocMem))
				uMem = float64(qq.Value()) / den
			}
		}

		// (Optional) Count actual pods on this node (one API call per node).
		// For large clusters, consider precomputing once outside the loop.
		podCount := 0
		if podsOnNode, err := r.core.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + n.Name,
		}); err == nil {
			podCount = len(podsOnNode.Items)
		}

		nm := domain.NodeMetric{
			NodeName: n.Name,
			CPUUsed:  clamp01(uCPU),
			MEMUsed:  clamp01(uMem),
			Pods:     podCount, // was: capacity-allocatable; now actual running/pending pods count
			K8sVer:   n.Status.NodeInfo.KubeletVersion,
			CPUTrend: r.appendTrend(r.nodeTrend, "cpu-"+n.Name, clamp01(uCPU)),
			MEMTrend: r.appendTrend(r.nodeTrend, "mem-"+n.Name, clamp01(uMem)),
		}
		out = append(out, nm)
	}

	return out, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// -------- LogsRepo --------

func (r *Repo) StreamLogs(ctx context.Context, t domain.LogsTarget) (<-chan domain.LogLine, error) {
	ch := make(chan domain.LogLine, 200)

	switch t.Kind {
	case "Pod":
		req := r.core.CoreV1().Pods(t.Namespace).GetLogs(t.Name, &corev1.PodLogOptions{
			Container:  t.Container,
			Follow:     true,
			Timestamps: false,
			// SinceSeconds: pointer.Int64(600),
		})
		stream, err := req.Stream(ctx)
		if err != nil {
			close(ch)
			return nil, err
		}
		go func() {
			defer close(ch)
			defer stream.Close()
			rd := bufio.NewReader(stream)
			for {
				line, err := rd.ReadString('\n')
				if len(line) > 0 {
					ch <- domain.LogLine{
						Time: time.Now(), Level: levelFrom(line),
						Text:   strings.TrimRight(line, "\r\n"),
						Source: fmt.Sprintf("%s/%s", t.Name, t.Container),
					}
				}
				if err != nil {
					if err == io.EOF || ctx.Err() != nil {
						return
					}
					// non-fatal: small backoff or just return
					return
				}
			}
		}()
		return ch, nil

	case "Deployment", "StatefulSet", "DaemonSet":
		ns := t.Namespace
		sel, _ := r.selectorOfOwner(ctx, ns, t)
		pods, _ := r.core.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: sel})
		go r.streamMultiPods(ctx, ch, pods.Items, "")
		return ch, nil

	case "Node":
		go r.streamNodeEvents(ctx, ch, t.Name)
		return ch, nil
	}

	// fallback
	close(ch)
	return ch, nil
}

func (r *Repo) streamMultiPods(ctx context.Context, out chan<- domain.LogLine, pods []corev1.Pod, container string) {
	for _, p := range pods {
		req := r.core.CoreV1().Pods(p.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{
			Container: container,
			Follow:    true,
		})
		stream, err := req.Stream(ctx)
		if err != nil {
			continue
		}
		go func(pod corev1.Pod, rc io.ReadCloser) {
			defer rc.Close()
			rd := bufio.NewReader(rc)
			for {
				line, err := rd.ReadString('\n')
				if len(line) > 0 {
					out <- domain.LogLine{Time: time.Now(), Level: levelFrom(line),
						Text:   strings.TrimRight(line, "\r\n"),
						Source: fmt.Sprintf("%s/%s", pod.Name, container)}
				}
				if err != nil {
					return
				}
			}
		}(p, stream)
	}
}

func (r *Repo) streamNodeEvents(ctx context.Context, out chan<- domain.LogLine, node string) {
	w, err := r.core.CoreV1().Events("").Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.kind=Node,involvedObject.name=%s", node),
	})
	if err != nil {
		close(out)
		return
	}
	for ev := range w.ResultChan() {
		if e, ok := ev.Object.(*corev1.Event); ok {
			out <- domain.LogLine{Time: e.LastTimestamp.Time, Level: strings.ToUpper(string(e.Type)), Text: e.Message, Source: "event/" + node}
		}
	}
}

func (r *Repo) selectorOfOwner(ctx context.Context, ns string, t domain.LogsTarget) (string, error) {
	if t.Kind == "Deployment" {
		d, err := r.core.AppsV1().Deployments(ns).Get(ctx, t.Name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return metav1.FormatLabelSelector(d.Spec.Selector), nil
	}
	return "", fmt.Errorf("not implemented")
}

func levelFrom(s string) string {
	ss := strings.ToUpper(s)
	switch {
	case strings.Contains(ss, "ERROR"):
		return "ERROR"
	case strings.Contains(ss, "WARN"):
		return "WARN"
	default:
		return "INFO"
	}
}
