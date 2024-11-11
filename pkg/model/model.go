package model

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"text/tabwriter"

	"github.com/AClarkie/k8s-tui/pkg/controller"
	tea "github.com/charmbracelet/bubbletea"
	appsv1 "k8s.io/api/apps/v1"
)

type state int

const (
	initializing state = iota
	ready
)

type model struct {
	choices     []string // items on the to-do list
	choiceMutex *sync.Mutex
	cursor      int              // which to-do list item our cursor is pointing at
	selected    map[int]struct{} // which to-do items are selected
	controller  *controller.Controller
	state       state
}

func InitialModel(controller *controller.Controller) (model, error) {
	return model{
		// Our to-do list is a grocery list
		choices: []string{},

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected:    make(map[int]struct{}),
		choiceMutex: &sync.Mutex{},

		controller: controller,
	}, nil
}

func (m model) Init() tea.Cmd {
	for !m.controller.Informer.HasSynced() {
		time.Sleep(100 * time.Millisecond)
	}
	return m.checkDeployments()
}

type deploymentMsg map[string]*appsv1.Deployment

func (m model) checkDeployments() tea.Cmd {
	d := time.Second * 1
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return deploymentMsg(m.controller.CurrentDeployments)
	})
}

func convertToSliceAndSort(deploymentMap map[string]*appsv1.Deployment) []string {
	keys := make([]string, len(deploymentMap))
	// fmt.Println("Length of deployment map: ", len(deploymentMap))

	i := 0
	for k := range deploymentMap {
		keys[i] = k
		i++
	}

	// Sort the keys
	sort.Strings(keys)

	return keys
}

func splitTheStringAndAddTabs(s string) string {
	return strings.ReplaceAll(s, "/", "\t")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.choiceMutex.Lock()
	defer m.choiceMutex.Unlock()
	switch msg := msg.(type) {

	case deploymentMsg:

		m.state = ready
		newChoices := convertToSliceAndSort(map[string]*appsv1.Deployment(msg))
		if len(m.choices) < len(newChoices) {
			m.cursor = 0
		}
		m.choices = newChoices

		return m, m.checkDeployments()

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	m.choiceMutex.Lock()
	defer m.choiceMutex.Unlock()
	if m.state == initializing {
		return "Initializing..."
	}

	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 8, 1, '\t', tabwriter.AlignRight)

	// The header
	footer := "\t Namespace\tDeployment\t\tReady\n"
	footer += "\t ---------\t----------\t\t-----"
	fmt.Fprintln(writer, footer)

	// Iterate over our choices
	for i, choice := range m.choices {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if _, ok := m.selected[i]; ok {
			checked = "x" // selected!
		}

		// Split the string and add tabs
		choice = splitTheStringAndAddTabs(choice)

		// Render the row
		fmt.Fprintln(writer, fmt.Sprintf("%s [%s] \t %s", cursor, checked, choice))
	}

	// The footer
	fmt.Fprintln(writer, "Press q to quit.")

	// Flush the writer and build the string
	writer.Flush()
	s := builder.String()

	// Send the UI for rendering
	return s
}
