package domain

import "context"

type MetricsRepo interface {
	ListPods(ctx context.Context, ns string, selector string) ([]PodMetric, error)
	ListNodes(ctx context.Context) ([]NodeMetric, error)
	ListNamespaces(ctx context.Context) ([]string, error)
}

type LogsTarget struct {
	Namespace string
	Kind      string // "Pod","Deployment","Node"...
	Name      string
	Container string
}

type LogsRepo interface {
	StreamLogs(ctx context.Context, t LogsTarget) (<-chan LogLine, error)
}
