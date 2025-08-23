package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/HaPhanBaoMinh/kmet/internal/domain"
	"github.com/HaPhanBaoMinh/kmet/internal/ui/styles"
	"github.com/HaPhanBaoMinh/kmet/internal/ui/widgets"
)

type View int
type logLineMsg domain.LogLine
type streamDone struct{}

const (
	ViewPods View = iota
	ViewNodes
)

type Model struct {
	ctx    context.Context
	cancel context.CancelFunc

	repoM domain.MetricsRepo
	repoL domain.LogsRepo

	// Namespace picker
	nsPickerOpen bool
	nsTable      table.Model
	autoCursor   bool

	view     View
	ns       string
	nsList   []string
	selector string
	sortBy   string // "cpu"|"mem"

	table table.Model

	// panes
	infoOpen   bool
	logsOpen   bool
	logsVP     viewport.Model
	logsCancel context.CancelFunc

	// cache
	pods  []domain.PodMetric
	nodes []domain.NodeMetric

	width, height int
	ticker        *time.Ticker
	err           error

	logCh  <-chan domain.LogLine
	logBuf strings.Builder
}

func New(repoM domain.MetricsRepo, repoL domain.LogsRepo) Model {
	ctx, cancel := context.WithCancel(context.Background())
	t := table.New()
	t.SetHeight(12)
	t.SetWidth(100)
	m := Model{
		ctx:        ctx,
		cancel:     cancel,
		repoM:      repoM,
		repoL:      repoL,
		view:       ViewPods,
		ns:         "default",
		autoCursor: false,
		sortBy:     "cpu",
		table:      t,
		logsVP:     viewport.New(10, 100),
	}
	m.ticker = time.NewTicker(2 * time.Second)

	// Get list namespace
	if repoM != nil {
		if nss, err := repoM.ListNamespaces(ctx); err == nil && len(nss) > 0 {
			m.nsList = nss
		} else {
			m.nsList = []string{"default"} // fallback
		}
	} else {
		m.nsList = []string{"default"}
	}

	// init ns picker table
	m.nsTable = table.New()
	m.nsTable.SetColumns([]table.Column{{Title: "Namespaces", Width: 32}})
	var nsRows []table.Row
	for _, ns := range m.nsList {
		m.ns = m.nsList[0]
		nsRows = append(nsRows, table.Row{ns})
	}
	m.nsTable.SetRows(nsRows)
	m.nsTable.SetHeight(10)
	m.nsTable.SetWidth(36)

	m.ticker = time.NewTicker(2 * time.Second)
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetch(),
		tea.Tick(time.Millisecond*500, func(time.Time) tea.Msg { return tickMsg{} }),
	)
}

type dataMsg struct{}
type tickMsg struct{}
type podsMsg []domain.PodMetric
type nodesMsg []domain.NodeMetric
type errMsg struct{ error }

func readNextLog(ch <-chan domain.LogLine) tea.Cmd {
	return func() tea.Msg {
		ln, ok := <-ch
		if !ok {
			return streamDone{}
		}
		return logLineMsg(ln)
	}
}

func (m Model) fetch() tea.Cmd {
	return func() tea.Msg {
		switch m.view {
		case ViewPods:
			p, err := m.repoM.ListPods(m.ctx, m.ns, m.selector)
			if err != nil {
				return errMsg{err}
			}
			mx := p
			sortPods(mx, m.sortBy)
			return podsMsg(p)
		case ViewNodes:
			n, err := m.repoM.ListNodes(m.ctx)
			if err != nil {
				return errMsg{err}
			}
			mx := n
			sortNodes(mx, m.sortBy)
			return nodesMsg(n)
		}
		return dataMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetWidth(m.width - 4)
		m.logsVP.Width = m.width - 4

		base := m.height - 4
		tableH := base
		if m.infoOpen && m.logsOpen {
			tableH = int(float64(base) * 0.5)
			m.logsVP.Height = int(float64(base) * 0.3)
		} else if m.logsOpen {
			tableH = int(float64(base) * 0.55)
			m.logsVP.Height = base - tableH
		} else if m.infoOpen {
			tableH = int(float64(base) * 0.65)
			m.logsVP.Height = 0
		} else {
			m.logsVP.Height = 0
		}
		if tableH < 8 {
			tableH = 8
		}
		m.table.SetHeight(tableH)
		return m, nil

	case podsMsg:
		m.pods = msg
		m.rebuildTable()

		rows := len(m.pods)
		cur := m.table.Cursor()
		if rows == 0 {
			// nothing to select
		} else if m.autoCursor || cur < 0 || cur >= rows {
			m.table.SetCursor(0) // auto-select first ONLY when needed
		} // else keep user's current selection
		m.autoCursor = false
		return m, nil

	case nodesMsg:
		m.nodes = msg
		m.rebuildTable()

		rows := len(m.nodes)
		cur := m.table.Cursor()
		if rows == 0 {
			// nothing to select
		} else if m.autoCursor || cur < 0 || cur >= rows {
			m.table.SetCursor(0)
		}
		m.autoCursor = false
		return m, nil
	case logLineMsg:
		ln := domain.LogLine(msg)
		m.logBuf.WriteString(fmt.Sprintf("%s %-5s %s [%s]\n",
			ln.Time.Format("15:04:05.000"), ln.Level, ln.Text, ln.Source))
		m.logsVP.SetContent(m.logBuf.String())
		m.logsVP.GotoBottom()
		return m, readNextLog(m.logCh)

	case streamDone:
		return m, nil

	case dataMsg:
		m.rebuildTable()
		// if we got data from old fetch() path
		if (m.view == ViewPods && len(m.pods) > 0) ||
			(m.view == ViewNodes && len(m.nodes) > 0) {
			m.table.SetCursor(0)
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(
			m.fetch(),
			tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} }),
		)

	case tea.KeyMsg:
		if m.nsPickerOpen {
			switch msg.String() {
			case "esc":
				m.nsPickerOpen = false
				return m, nil
			case "enter":
				if len(m.nsList) > 0 {
					idx := m.nsTable.Cursor()
					if idx < 0 {
						idx = 0
					}
					if idx >= len(m.nsList) {
						idx = len(m.nsList) - 1
					}
					newNS := m.nsList[idx]
					if newNS != "" && newNS != m.ns {
						m.ns = newNS
						m.table.SetCursor(0)
						m.infoOpen, m.logsOpen = false, false
						if m.logsCancel != nil {
							m.logsCancel()
						}
						m.nsPickerOpen = false
						return m, m.fetch()
					}
				}
				m.nsPickerOpen = false
				return m, nil
			case "up", "k", "down", "j", "pgup", "pgdn", "home", "end":
				var cmd tea.Cmd
				m.nsTable, cmd = m.nsTable.Update(msg)
				return m, cmd
			}

		}
		if m.logsOpen {
			switch msg.String() {
			case "pgup":
				m.logsVP.LineUp(10)
				return m, nil
			case "pgdn":
				m.logsVP.LineDown(10)
				return m, nil
			case "g":
				m.logsVP.GotoTop()
				return m, nil
			case "G":
				m.logsVP.GotoBottom()
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "q", "esc":
			if m.logsCancel != nil {
				m.logsCancel()
			}
			m.nsTable.Blur()
			m.cancel()
			return m, tea.Quit

		case "n":
			m.nsPickerOpen = true
			m.nsTable.Focus()
			cur := 0
			for i, v := range m.nsList {
				if v == m.ns {
					cur = i
					break
				}
			}
			m.nsTable.SetCursor(cur)
			return m, nil

		case "tab":
			if m.view == ViewPods {
				m.view = ViewNodes
			} else {
				m.view = ViewPods
			}
			m.infoOpen, m.logsOpen = false, false
			m.autoCursor = true // <— reset cursor on next load
			return m, m.fetch()
		case "i":
			m.infoOpen = !m.infoOpen
			return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }

		case "l":
			if m.logsOpen {
				m.logsOpen = false
				if m.logsCancel != nil {
					m.logsCancel()
				}
				return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }
			}
			m.logsOpen = true
			target := m.currentLogsTarget()
			ctx, cancel := context.WithCancel(m.ctx)
			m.logsCancel = cancel
			m.logsVP.SetContent("")
			m.logBuf.Reset()
			return m, tea.Batch(
				m.consumeLogs(ctx, target),
				func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} },
			)

		case "s":
			if m.sortBy == "cpu" {
				m.sortBy = "mem"
			} else {
				m.sortBy = "cpu"
			}
			return m, m.fetch()

		case "up", "k", "down", "j", "enter":
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}

	case errMsg:
		m.err = msg.error
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) rebuildTable() {
	switch m.view {
	case ViewPods:
		cols := []table.Column{
			{Title: "POD (ctr)", Width: 36},
			{Title: "CPU", Width: 12},
			{Title: "MEM", Width: 14},
			{Title: "READY", Width: 6},
			{Title: "NODE", Width: 16},
			{Title: "Trend", Width: 12},
		}
		var rows []table.Row
		for _, p := range m.pods {
			cpuBar := widgets.Bar(float64(p.CPUm)/500.0, 8) // normalize ~500m
			memBar := widgets.Bar(float64(p.MemBytes)/(1.2*1024*1024*1024), 10)
			rows = append(rows, table.Row{
				fmt.Sprintf("%s (%s)", p.PodName, p.Container),
				fmt.Sprintf("%3dm %s", p.CPUm, cpuBar),
				fmt.Sprintf("%4.1fMi %s", float64(p.MemBytes)/(1024*1024), memBar),
				p.Ready,
				p.NodeName,
				widgets.Spark8(p.CPUTrend.Samples, 8),
			})
		}
		m.table.SetColumns(cols)
		m.table.SetRows(rows)
		m.table.Focus()

	case ViewNodes:
		cols := []table.Column{
			{Title: "NODE", Width: 20},
			{Title: "CPU%", Width: 16},
			{Title: "MEM%", Width: 16},
			{Title: "PODS", Width: 5},
			{Title: "K8S", Width: 5},
			{Title: "Trend", Width: 12},
		}
		var rows []table.Row
		for _, n := range m.nodes {
			rows = append(rows, table.Row{
				n.NodeName,
				fmt.Sprintf("%3.0f%% %s", n.CPUUsed*100, widgets.Bar(n.CPUUsed, 8)),
				fmt.Sprintf("%3.0f%% %s", n.MEMUsed*100, widgets.Bar(n.MEMUsed, 8)),
				fmt.Sprintf("%d", n.Pods),
				n.K8sVer,
				widgets.Spark8(n.CPUTrend.Samples, 8),
			})
		}
		m.table.SetColumns(cols)
		m.table.SetRows(rows)
		m.table.Focus()
	}
}

func (m Model) currentSelection() int {
	i := m.table.Cursor()
	if i < 0 {
		i = 0
	}
	return i
}

func (m Model) currentLogsTarget() domain.LogsTarget {
	switch m.view {
	case ViewPods:
		i := m.currentSelection()
		if len(m.pods) == 0 {
			return domain.LogsTarget{Namespace: m.ns, Kind: "Pod", Name: ""}
		}
		p := m.pods[i%len(m.pods)]
		return domain.LogsTarget{Namespace: m.ns, Kind: "Pod", Name: p.PodName, Container: p.Container}
	case ViewNodes:
		i := m.currentSelection()
		if len(m.nodes) == 0 {
			return domain.LogsTarget{Kind: "Node"}
		}
		n := m.nodes[i%len(m.nodes)]
		return domain.LogsTarget{Kind: "Node", Name: n.NodeName}
	default:
		return domain.LogsTarget{}
	}
}

func (m Model) consumeLogs(ctx context.Context, t domain.LogsTarget) tea.Cmd {
	return func() tea.Msg {
		ch, err := m.repoL.StreamLogs(ctx, t)
		if err != nil {
			return errMsg{err}
		}
		m.logCh = ch
		m.logsVP.SetContent("")
		m.logBuf.Reset()
		return readNextLog(ch)()
	}
}

func (m Model) View() string {
	head := styles.Header.Render(
		fmt.Sprintf("kmet v0.x  │ ctx: dev  ns: %s  view: %s  sort: %s  (Tab switch Pods/Nodes)  [i]info [l]logs [s]sort [q]quit",
			m.ns, map[View]string{ViewPods: "Pods", ViewNodes: "Nodes"}[m.view], m.sortBy),
	)
	body := lipgloss.NewStyle().Padding(0, 1).Render(m.table.View())

	info := ""
	if m.infoOpen {
		info = styles.Box.Width(m.width - 2).Render(m.renderInfo())
	}

	logs := ""
	if m.logsOpen {
		logs = styles.Box.Width(m.width - 2).Render("Logs:\n" + m.logsVP.View())
	}

	// Overlay picker
	overlay := ""
	if m.nsPickerOpen {
		box := styles.Box.
			BorderForeground(lipgloss.Color("#7DCE13")).
			Width(40).Height(14)
		title := styles.Title.Render(" Switch Namespace (↑/↓, Enter, Esc) ")
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			m.nsTable.View(),
		)
		// căn giữa màn hình
		overlay = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			box.Render(content),
		)
	}
	footer := styles.Footer.Render("↑/↓ move • [Tab] switch view • [n] namespace • [i] info • [l] logs • [s] sort • [q] quit")

	main := lipgloss.JoinVertical(lipgloss.Left, head, body, info, logs, footer)
	if m.nsPickerOpen {
		// ghép overlay lên trên (simple: nối cuối; advanced: layer)
		return main + "\n" + overlay
	}
	return main
}

func (m Model) renderInfo() string {
	switch m.view {
	case ViewPods:
		i := m.currentSelection()
		if len(m.pods) == 0 {
			return "No pods"
		}
		p := m.pods[i%len(m.pods)]
		utilCPU := float64(p.CPUm) / float64(max(1, p.CPUReqm))
		utilMem := float64(p.MemBytes) / float64(max64(1, p.MemReqBytes))
		return fmt.Sprintf(
			"Pod: %s  ns: %s  node: %s  phase: %s\nImage: ghcr.io/acme/%s:mock\nRequests: cpu=%dm mem=%dMi  Ready: %s\nUtil vs Req: CPU %.0f%% %s  MEM %.0f%% %s\nTrend CPU: %s\nTrend MEM: %s",
			p.PodName, p.Namespace, p.NodeName, p.Phase, p.Container, p.CPUReqm, p.MemReqBytes/(1024*1024), p.Ready,
			utilCPU*100, widgets.Bar(utilCPU/2.5, 12), // scale a bit
			utilMem*100, widgets.Bar(utilMem/3.0, 12),
			widgets.Spark8(p.CPUTrend.Samples, 30),
			widgets.Spark8(p.MemTrend.Samples, 30),
		)
	case ViewNodes:
		i := m.currentSelection()
		if len(m.nodes) == 0 {
			return "No nodes"
		}
		n := m.nodes[i%len(m.nodes)]
		return fmt.Sprintf(
			"Node: %s  k8s: %s  pods: %d\nCPU(5m): %s\nMEM(5m): %s",
			n.NodeName, n.K8sVer, n.Pods,
			widgets.Spark8(n.CPUTrend.Samples, 40),
			widgets.Spark8(n.MEMTrend.Samples, 40),
		)
	default:
		return ""
	}
}

func sortPods(p []domain.PodMetric, by string) {
	// simple bubble (mock), stable enough for demo
	for i := 0; i < len(p); i++ {
		for j := 0; j < len(p)-1; j++ {
			less := p[j].CPUm < p[j+1].CPUm
			if by == "mem" {
				less = p[j].MemBytes < p[j+1].MemBytes
			}
			if less {
				p[j], p[j+1] = p[j+1], p[j]
			}
		}
	}
}

func sortNodes(n []domain.NodeMetric, by string) {
	for i := 0; i < len(n); i++ {
		for j := 0; j < len(n)-1; j++ {
			less := n[j].CPUUsed < n[j+1].CPUUsed
			if by == "mem" {
				less = n[j].MEMUsed < n[j+1].MEMUsed
			}
			if less {
				n[j], n[j+1] = n[j+1], n[j]
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
