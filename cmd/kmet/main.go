package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/HaPhanBaoMinh/kmet/help"
	"github.com/HaPhanBaoMinh/kmet/internal/app"
	"github.com/HaPhanBaoMinh/kmet/internal/domain"
	kk "github.com/HaPhanBaoMinh/kmet/internal/infrastructure/k8s"
	"github.com/HaPhanBaoMinh/kmet/internal/infrastructure/mock"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var useMock bool
	var kubeconfig, contextName string
	flag.BoolVar(&useMock, "mock", false, "use mock repo")
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(help.HomeDir(), ".kube", "config"), "path to kubeconfig")
	flag.StringVar(&contextName, "context", "", "kube context")
	flag.Parse()

	var repoM domain.MetricsRepo // actually domain.MetricsRepo, but shortcut in this file
	var repoL domain.LogsRepo

	if useMock {
		repo := mock.New()
		repoM, repoL = repo, repo
	} else {
		repo, err := kk.New(kubeconfig, contextName)
		if err != nil {
			log.Fatal(err)
		}
		repoM, repoL = repo, repo
	}

	m := app.New(repoM, repoL)
	if err := tea.NewProgram(m, tea.WithAltScreen()).Start(); err != nil {
		log.Fatal(err)
	}
}
