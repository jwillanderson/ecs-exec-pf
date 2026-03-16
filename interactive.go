package ecsexecpf

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type InteractiveResult struct {
	Service      string
	Task         string
	AutoTask     bool
	PortMappings []PortMapping
}

type selectState int

const (
	stateSelectService selectState = iota
	stateLoadingTasks
	stateSelectTask
	stateSelectPortMapping
	stateDone
)

const autoTaskLabel = "Any (auto-reconnect)"

type item struct {
	title string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.title }

type tasksLoadedMsg struct {
	tasks []string
	err   error
}

type model struct {
	state        selectState
	cfg          aws.Config
	cluster      string
	list         list.Model
	service      string
	task         string
	autoTask     bool
	needPorts    bool
	portSelector portSelector
	portMappings []PortMapping
	err          error
	quitting     bool
}

func newModel(cfg aws.Config, cluster string, services []string, needPorts bool) model {
	items := make([]list.Item, len(services))
	for i, s := range services {
		items[i] = item{title: s}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).
		BorderLeftForeground(lipgloss.Color("170"))

	l := list.New(items, delegate, 60, 20)
	l.Title = fmt.Sprintf("Select a service (%s)", cluster)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return model{
		state:     stateSelectService,
		cfg:       cfg,
		cluster:   cluster,
		list:      l,
		needPorts: needPorts,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state == stateSelectPortMapping {
		return m.updatePortSelector(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			return m.handleSelection()
		}

	case tasksLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		return m.showTaskList(msg.tasks), nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) updatePortSelector(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if !m.portSelector.inputMode {
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.portSelector, cmd = m.portSelector.Update(msg)
	if m.portSelector.done {
		m.portMappings = m.portSelector.selected
		m.state = stateDone
		return m, tea.Quit
	}
	return m, cmd
}

func (m model) handleSelection() (tea.Model, tea.Cmd) {
	selected, ok := m.list.SelectedItem().(item)
	if !ok {
		return m, nil
	}

	switch m.state {
	case stateSelectService:
		m.service = selected.title
		m.state = stateLoadingTasks
		m.list.Title = fmt.Sprintf("Loading tasks for %s...", m.service)
		return m, m.loadTasks()

	case stateSelectTask:
		if selected.title == autoTaskLabel {
			m.autoTask = true
		} else {
			m.task = selected.title
		}
		if m.needPorts {
			return m.transitionToPortSelection()
		}
		m.state = stateDone
		return m, tea.Quit
	}

	return m, nil
}

func (m model) transitionToPortSelection() (tea.Model, tea.Cmd) {
	history, _ := LoadPortHistory()
	savedMappings := history.Services[m.service]
	m.portSelector = newPortSelector(m.service, savedMappings, m.list.Width(), m.list.Height())
	m.state = stateSelectPortMapping
	return m, nil
}

func (m model) loadTasks() tea.Cmd {
	return func() tea.Msg {
		tasks, err := ListTasksForService(m.cfg, m.cluster, m.service)
		return tasksLoadedMsg{tasks: tasks, err: err}
	}
}

func (m model) showTaskList(tasks []string) model {
	items := make([]list.Item, 0, len(tasks)+1)
	items = append(items, item{title: autoTaskLabel})
	for _, t := range tasks {
		items = append(items, item{title: t})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).
		BorderLeftForeground(lipgloss.Color("170"))

	m.list = list.New(items, delegate, m.list.Width(), m.list.Height())
	m.list.Title = fmt.Sprintf("Select a task (%s/%s)", m.cluster, m.service)
	m.list.SetShowStatusBar(true)
	m.list.SetFilteringEnabled(true)
	m.state = stateSelectTask
	return m
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.state == stateSelectPortMapping {
		return m.portSelector.View()
	}
	return m.list.View()
}

func SelectServiceAndTask(cfg aws.Config, cluster string, needPorts bool) (*InteractiveResult, error) {
	services, err := ListServices(cfg, cluster)
	if err != nil {
		return nil, err
	}

	m := newModel(cfg, cluster, services, needPorts)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("interactive selection failed: %w", err)
	}

	result := finalModel.(model)
	if result.quitting && result.state != stateDone {
		return nil, fmt.Errorf("selection cancelled")
	}
	if result.err != nil {
		return nil, result.err
	}

	ir := &InteractiveResult{
		Service:      result.service,
		Task:         result.task,
		AutoTask:     result.autoTask,
		PortMappings: result.portMappings,
	}

	if len(result.portMappings) > 0 {
		history, _ := LoadPortHistory()
		updated := AddMapping(history, result.service, result.portMappings)
		_ = SavePortHistory(updated)
	}

	return ir, nil
}
