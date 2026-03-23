package tui

import (
	"fmt"
	"strings"

	"mita/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ConfigModel manages the configuration screens.
type ConfigModel struct {
	cfg        *config.Config
	cursor     int
	inputs     []textinput.Model
	inputFocus int
	subScreen  int // 0=menu, 1=gitlab, 2=creds, 3=mappings
	mappingIdx int
	message    string
}

func NewConfigModel(cfg *config.Config) *ConfigModel {
	return &ConfigModel{
		cfg: cfg,
	}
}

func (m *ConfigModel) Update(app *App, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(app, msg)
	}

	// Update text inputs if active
	if m.subScreen > 0 && m.inputs != nil && m.inputFocus < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.inputFocus], cmd = m.inputs[m.inputFocus].Update(msg)
		return app, cmd
	}

	return app, nil
}

func (m *ConfigModel) handleKey(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return app, tea.Quit
	case "esc":
		if m.subScreen > 0 {
			m.subScreen = 0
			app.screen = ScreenConfig
			m.message = ""
			return app, nil
		}
		app.screen = ScreenMainMenu
		return app, nil
	}

	switch m.subScreen {
	case 0: // Config menu
		return m.handleConfigMenu(app, msg)
	case 1: // GitLab URL
		return m.handleGitLabURL(app, msg)
	case 2: // Credentials
		return m.handleCredentials(app, msg)
	case 3: // Mappings
		return m.handleMappings(app, msg)
	}

	return app, nil
}

func (m *ConfigModel) handleConfigMenu(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 3 {
			m.cursor++
		}
	case "enter":
		switch m.cursor {
		case 0: // GitLab URL
			m.subScreen = 1
			app.screen = ScreenConfigGitLabURL
			m.initGitLabURLInputs()
		case 1: // Credentials
			m.subScreen = 2
			app.screen = ScreenConfigCredentials
			m.initCredentialInputs()
		case 2: // Mappings
			m.subScreen = 3
			app.screen = ScreenConfigMappings
		case 3: // Back
			app.screen = ScreenMainMenu
		}
	}
	return app, nil
}

func (m *ConfigModel) initGitLabURLInputs() {
	m.inputs = make([]textinput.Model, 2)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "https://gitlab.example.com"
	m.inputs[0].SetValue(m.cfg.SourceGitLabURL)
	m.inputs[0].Focus()

	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "https://gitlab.classified.local"
	m.inputs[1].SetValue(m.cfg.TargetGitLabURL)

	m.inputFocus = 0
}

func (m *ConfigModel) initCredentialInputs() {
	m.inputs = make([]textinput.Model, 3)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "Private token (for source GitLab)"
	m.inputs[0].SetValue(m.cfg.SourceToken)
	m.inputs[0].Focus()

	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "Username (for target GitLab)"
	m.inputs[1].SetValue(m.cfg.TargetUsername)

	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "Password (for target GitLab)"
	m.inputs[2].EchoMode = textinput.EchoPassword
	m.inputs[2].SetValue(m.cfg.TargetPassword)

	m.inputFocus = 0
}

func (m *ConfigModel) handleGitLabURL(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		m.cycleFocus(msg.String() == "shift+tab")
	case "enter":
		if m.inputFocus == len(m.inputs)-1 {
			m.cfg.SourceGitLabURL = m.inputs[0].Value()
			m.cfg.TargetGitLabURL = m.inputs[1].Value()
			if err := m.cfg.Save(""); err != nil {
				m.message = "Error saving: " + err.Error()
			} else {
				m.message = "Saved!"
			}
			return app, nil
		}
		m.cycleFocus(false)
	}

	var cmd tea.Cmd
	m.inputs[m.inputFocus], cmd = m.inputs[m.inputFocus].Update(msg)
	return app, cmd
}

func (m *ConfigModel) handleCredentials(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		m.cycleFocus(msg.String() == "shift+tab")
	case "enter":
		if m.inputFocus == len(m.inputs)-1 {
			m.cfg.SourceToken = m.inputs[0].Value()
			m.cfg.TargetUsername = m.inputs[1].Value()
			m.cfg.TargetPassword = m.inputs[2].Value()
			if err := m.cfg.Save(""); err != nil {
				m.message = "Error saving: " + err.Error()
			} else {
				m.message = "Saved!"
			}
			return app, nil
		}
		m.cycleFocus(false)
	}

	var cmd tea.Cmd
	m.inputs[m.inputFocus], cmd = m.inputs[m.inputFocus].Update(msg)
	return app, cmd
}

func (m *ConfigModel) handleMappings(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	mappings := m.cfg.Mappings.List()
	keys := make([]string, 0, len(mappings))
	for k := range mappings {
		keys = append(keys, k)
	}

	switch msg.String() {
	case "up", "k":
		if m.mappingIdx > 0 {
			m.mappingIdx--
		}
	case "down", "j":
		if m.mappingIdx < len(keys)-1 {
			m.mappingIdx++
		}
	case "d":
		if len(keys) > 0 && m.mappingIdx < len(keys) {
			m.cfg.Mappings.Delete(keys[m.mappingIdx])
			m.cfg.Save("")
			if m.mappingIdx > 0 {
				m.mappingIdx--
			}
		}
	}

	return app, nil
}

func (m *ConfigModel) cycleFocus(backward bool) {
	if backward {
		m.inputFocus--
	} else {
		m.inputFocus++
	}
	if m.inputFocus < 0 {
		m.inputFocus = len(m.inputs) - 1
	}
	if m.inputFocus >= len(m.inputs) {
		m.inputFocus = 0
	}
	for i := range m.inputs {
		if i == m.inputFocus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *ConfigModel) View(app *App) string {
	switch m.subScreen {
	case 0:
		return m.viewConfigMenu()
	case 1:
		return m.viewGitLabURL()
	case 2:
		return m.viewCredentials()
	case 3:
		return m.viewMappings()
	}
	return ""
}

func (m *ConfigModel) viewConfigMenu() string {
	title := titleStyle.Render("Configuration")

	items := []string{
		"GitLab URLs",
		"Credentials",
		"Project Mappings",
		"Back to Main Menu",
	}

	menu := ""
	for i, item := range items {
		cursor := "  "
		style := menuItemStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedMenuItemStyle
		}
		menu += style.Render(fmt.Sprintf("%s[%d] %s", cursor, i+1, item)) + "\n"
	}

	// Show current config summary
	summary := "\n" + mutedStyle.Render("Current Configuration:") + "\n"
	summary += fmt.Sprintf("  Source: %s\n", m.cfg.SourceGitLabURL)
	summary += fmt.Sprintf("  Target: %s\n", m.cfg.TargetGitLabURL)
	summary += fmt.Sprintf("  Mappings: %d\n", len(m.cfg.Mappings.Entries))

	help := helpStyle.Render("Up/Down: Navigate  |  Enter: Select  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + menu + summary + "\n" + help))
}

func (m *ConfigModel) viewGitLabURL() string {
	title := titleStyle.Render("Configuration - GitLab URLs")

	var form strings.Builder
	labels := []string{"Source GitLab URL:", "Target GitLab URL:"}
	for i, input := range m.inputs {
		form.WriteString(labelStyle.Render(labels[i]) + "\n" + input.View() + "\n\n")
	}

	msg := ""
	if m.message != "" {
		if strings.HasPrefix(m.message, "Error") {
			msg = errorStyle.Render(m.message)
		} else {
			msg = successStyle.Render(m.message)
		}
	}

	help := helpStyle.Render("Tab: Next field  |  Enter: Save  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + form.String() + msg + "\n" + help))
}

func (m *ConfigModel) viewCredentials() string {
	title := titleStyle.Render("Configuration - Credentials")

	var form strings.Builder
	labels := []string{"Source Token:", "Target Username:", "Target Password:"}
	for i, input := range m.inputs {
		form.WriteString(labelStyle.Render(labels[i]) + "\n" + input.View() + "\n\n")
	}

	msg := ""
	if m.message != "" {
		if strings.HasPrefix(m.message, "Error") {
			msg = errorStyle.Render(m.message)
		} else {
			msg = successStyle.Render(m.message)
		}
	}

	help := helpStyle.Render("Tab: Next field  |  Enter: Save  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + form.String() + msg + "\n" + help))
}

func (m *ConfigModel) viewMappings() string {
	title := titleStyle.Render("Configuration - Project Mappings")

	mappings := m.cfg.Mappings.List()
	if len(mappings) == 0 {
		return appStyle.Render(boxStyle.Render(
			title + "\n\n" +
				mutedStyle.Render("No mappings configured.") + "\n" +
				mutedStyle.Render("Mappings are created during import when a new project is detected.") + "\n\n" +
				helpStyle.Render("Esc: Back")))
	}

	keys := make([]string, 0, len(mappings))
	for k := range mappings {
		keys = append(keys, k)
	}

	var table strings.Builder
	table.WriteString(fmt.Sprintf("  %-25s %-25s %-20s %s\n",
		mutedStyle.Render("Source"),
		mutedStyle.Render("Target Project"),
		mutedStyle.Render("Group"),
		mutedStyle.Render("Branch")))
	table.WriteString("  " + strings.Repeat("-", 80) + "\n")

	for i, key := range keys {
		entry := mappings[key]
		cursor := "  "
		if i == m.mappingIdx {
			cursor = "> "
		}
		table.WriteString(fmt.Sprintf("%s%-25s %-25s %-20s %s\n",
			cursor, key, entry.TargetProject, entry.TargetGroup, entry.TargetBranch))
	}

	help := helpStyle.Render("Up/Down: Navigate  |  D: Delete  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + table.String() + "\n" + help))
}
