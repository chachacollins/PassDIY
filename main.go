package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	cmd "github.com/jalpp/passdiy/cmds"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	style "github.com/jalpp/passdiy/style"
)

type model struct {
	list       list.Model
	spinner    spinner.Model
	textInput  textinput.Model
	loading    bool
	output     string
	prevOutput string
	copied     bool
	showInput  bool
	inputMode  string
}

func NewModel() model {
	const defaultWidth = 70

	items := cmd.CreateCommandItems()
	listModel := list.New(items, list.NewDefaultDelegate(), defaultWidth, len(items)+2)

	spin := spinner.New(
		spinner.WithSpinner(spinner.Points),
		spinner.WithStyle(style.GreenStyle),
	)

	textInput := textinput.New()
	textInput.Placeholder = "Enter input"
	textInput.Focus()
	textInput.CharLimit = 5000
	textInput.Width = 30

	return model{
		list:       listModel,
		spinner:    spin,
		textInput:  textInput,
		loading:    false,
		output:     "",
		prevOutput: "",
		copied:     false,
		showInput:  false,
		inputMode:  "",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "c":
			if m.output != "" {
				clipboard.WriteAll(m.output)
				m.copied = true
			}
			return m, nil
		case "enter":
			if m.loading {
				return m, nil
			}
			if m.showInput {
				inputValue := m.textInput.Value()
				m.textInput.SetValue("")
				m.showInput = false

				if m.inputMode == "hash" && inputValue != "" {
					return m, tea.Batch(cmd.ExecuteCommand("hash", inputValue), m.spinner.Tick)
				}
				if m.inputMode == "hcpvaultstore" {
					if inputValue != "" && strings.Contains(inputValue, "=") {
						return m, tea.Batch(cmd.ExecuteCommand("hcpvaultstore", inputValue), m.spinner.Tick)
					}
					m.output = "Please provide input in 'name=value' format."
					return m, nil
				}
				m.output = "Please provide valid input."
				return m, nil
			}

			selectedItem := m.list.SelectedItem().(cmd.CommandItem).Title()
			if selectedItem == "hash" || selectedItem == "hcpvaultstore" {
				m.inputMode = selectedItem
				m.textInput.SetValue("")
				m.prevOutput = ""
				m.showInput = true
				m.textInput.Focus()
				return m, nil
			}

			m.loading = true
			m.copied = false
			return m, tea.Batch(cmd.ExecuteCommand(selectedItem, m.prevOutput), m.spinner.Tick)
		}

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case cmd.CommandFinishedMsg:
		m.loading = false
		m.output = msg.Result()
		m.prevOutput = msg.Result()
	}

	if m.showInput {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.loading {
		return style.GreenStyle.Render(fmt.Sprintf("%s Processing command...\n\n%s", m.spinner.View(), ""))
	}

	if m.showInput {
		maskedInput := cmd.CoverUp(m.textInput.Value())

		if m.inputMode == "tfvaultstore" {
			return style.GreenStyle.Render(fmt.Sprintf(
				"Enter the token in 'name=value' format and press Enter:\n\n%s\n\n",
				maskedInput,
			))
		}
		return style.GreenStyle.Render(fmt.Sprintf(
			"Enter the input for hashing and press Enter:\n\n%s\n\n",
			maskedInput,
		))
	}

	copyMessage := ""
	if m.copied {
		copyMessage = style.GreenStyle.Render("\n📋 Buffer value copied to clipboard!")
	}

	if strings.Contains(strings.ToLower(m.output), "please") {
		return style.ErrorStyle.Render(fmt.Sprintf("%s\n\n ❌ Error: %s", m.list.View(), m.output))
	}

	if strings.Contains(strings.ToLower(m.output), "authentication is required") {
		return style.ErrorStyle.Render(fmt.Sprintf("%s\n\n ❌ Error: %s", m.list.View(), "HCP_API_TOKEN expired, please run hcpvaultconnect to re connect to HCP Vault"))
	}

	if strings.Contains(strings.ToLower(m.output), "hashicorp") {
		return style.VaultStyle.Render(fmt.Sprintf("%s\n\n ⛛ Vault: %s", m.list.View(), m.output))
	}

	return style.GreenStyle.Render(fmt.Sprintf("%s\n\n 🔑 (Press c to copy) Buffer: %s%s", m.list.View(), cmd.CoverUp(m.output), copyMessage))
}

func main() {
	if _, err := tea.NewProgram(NewModel()).Run(); err != nil {
		fmt.Printf("Uh oh, there was an error: %v\n", err)
		os.Exit(1)
	}
}
