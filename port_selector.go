package ecsexecpf

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type portItem struct {
	mappings []PortMapping
	label    string
}

func (p portItem) Title() string       { return p.label }
func (p portItem) Description() string { return "" }
func (p portItem) FilterValue() string { return p.label }

type portSelector struct {
	list      list.Model
	textInput textinput.Model
	inputMode bool
	selected  []PortMapping
	err       error
	errMsg    string
	done      bool
	width     int
	height    int
}

func newPortSelector(service string, history [][]PortMapping, width, height int) portSelector {
	ti := textinput.New()
	ti.Placeholder = "local:remote, local:remote (e.g. 8080:80, 8443:443)"
	ti.CharLimit = 256
	ti.Width = 50

	if len(history) == 0 {
		ti.Focus()
		return portSelector{
			textInput: ti,
			inputMode: true,
			width:     width,
			height:    height,
		}
	}

	items := make([]list.Item, 0, len(history)+1)
	for _, mappings := range history {
		items = append(items, portItem{
			mappings: mappings,
			label:    FormatMappings(mappings),
		})
	}
	items = append(items, portItem{label: "Enter new port mapping..."})

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).
		BorderLeftForeground(lipgloss.Color("170"))

	l := list.New(items, delegate, width, height)
	l.Title = fmt.Sprintf("Select port mapping (%s)", service)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)

	return portSelector{
		list:      l,
		textInput: ti,
		width:     width,
		height:    height,
	}
}

func (ps portSelector) Update(msg tea.Msg) (portSelector, tea.Cmd) {
	if ps.inputMode {
		return ps.updateInput(msg)
	}
	return ps.updateList(msg)
}

func (ps portSelector) updateList(msg tea.Msg) (portSelector, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ps.width = msg.Width
		ps.height = msg.Height - 2
		ps.list.SetWidth(msg.Width)
		ps.list.SetHeight(msg.Height - 2)
		return ps, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected, ok := ps.list.SelectedItem().(portItem)
			if !ok {
				return ps, nil
			}
			if selected.mappings == nil {
				ps.inputMode = true
				ps.textInput.Focus()
				return ps, nil
			}
			ps.selected = selected.mappings
			ps.done = true
			return ps, nil
		}
	}

	var cmd tea.Cmd
	ps.list, cmd = ps.list.Update(msg)
	return ps, cmd
}

func (ps portSelector) updateInput(msg tea.Msg) (portSelector, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			mappings, err := ParsePortMappings(ps.textInput.Value())
			if err != nil {
				ps.errMsg = err.Error()
				return ps, nil
			}
			ps.selected = mappings
			ps.done = true
			return ps, nil
		case "esc":
			if ps.list.Items() != nil {
				ps.inputMode = false
				ps.errMsg = ""
				return ps, nil
			}
		}
	}

	ps.errMsg = ""
	var cmd tea.Cmd
	ps.textInput, cmd = ps.textInput.Update(msg)
	return ps, cmd
}

func (ps portSelector) View() string {
	if ps.inputMode {
		s := "Enter port mappings (local:remote, ...)\n\n"
		s += ps.textInput.View() + "\n"
		if ps.errMsg != "" {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
			s += "\n" + errStyle.Render(ps.errMsg) + "\n"
		}
		s += "\nPress enter to confirm"
		if ps.list.Items() != nil {
			s += ", esc to go back"
		}
		return s
	}
	return ps.list.View()
}

func ParsePortMappings(input string) ([]PortMapping, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("port mappings cannot be empty")
	}

	parts := strings.Split(input, ",")
	mappings := make([]PortMapping, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		pair := strings.SplitN(part, ":", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid format %q, expected local:remote", part)
		}

		local, err := strconv.Atoi(strings.TrimSpace(pair[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid local port %q: %w", pair[0], err)
		}

		remote, err := strconv.Atoi(strings.TrimSpace(pair[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid remote port %q: %w", pair[1], err)
		}

		if local < 0 || local >= 65535 {
			return nil, fmt.Errorf("local port %d must be between 0 and 65534", local)
		}
		if remote < 0 || remote >= 65535 {
			return nil, fmt.Errorf("remote port %d must be between 0 and 65534", remote)
		}

		mappings = append(mappings, PortMapping{LocalPort: local, RemotePort: remote})
	}

	if len(mappings) == 0 {
		return nil, fmt.Errorf("no valid port mappings provided")
	}

	return mappings, nil
}
