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
	subScreen  int // 0=menu, 1=gitlab, 2=creds, 3=mappings, 4=security
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
	case 0:
		return m.handleConfigMenu(app, msg)
	case 1:
		return m.handleGitLabURL(app, msg)
	case 2:
		return m.handleCredentials(app, msg)
	case 3:
		return m.handleMappings(app, msg)
	case 4:
		return m.handleSecurity(app, msg)
	}

	return app, nil
}

func (m *ConfigModel) handleConfigMenu(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	menuCount := 5
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < menuCount-1 {
			m.cursor++
		}
	case "enter":
		switch m.cursor {
		case 0: // GitLab URLs
			m.subScreen = 1
			app.screen = ScreenConfigGitLabURL
			m.initGitLabURLInputs()
		case 1: // Source Credentials
			m.subScreen = 2
			app.screen = ScreenConfigCredentials
			m.initCredentialInputs()
		case 2: // Security (TLS)
			m.subScreen = 4
		case 3: // Mappings
			m.subScreen = 3
			app.screen = ScreenConfigMappings
		case 4: // Back
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
	if m.cfg.SourceAuthMode == "" {
		m.cfg.SourceAuthMode = "token"
	}

	if m.cfg.SourceAuthMode == "basic" {
		m.inputs = make([]textinput.Model, 5)

		m.inputs[0] = textinput.New()
		m.inputs[0].Placeholder = "Source username"
		m.inputs[0].SetValue(m.cfg.SourceUsername)
		m.inputs[0].Focus()

		m.inputs[1] = textinput.New()
		m.inputs[1].Placeholder = "Source password"
		m.inputs[1].EchoMode = textinput.EchoPassword
		m.inputs[1].SetValue(m.cfg.SourcePassword)

		m.inputs[2] = textinput.New()
		m.inputs[2].Placeholder = "Target username"
		m.inputs[2].SetValue(m.cfg.TargetUsername)

		m.inputs[3] = textinput.New()
		m.inputs[3].Placeholder = "Target password"
		m.inputs[3].EchoMode = textinput.EchoPassword
		m.inputs[3].SetValue(m.cfg.TargetPassword)

		// Hidden dummy to detect "last field enter"
		m.inputs[4] = textinput.New()
		m.inputs[4].Placeholder = ""
	} else {
		m.inputs = make([]textinput.Model, 4)

		m.inputs[0] = textinput.New()
		m.inputs[0].Placeholder = "Private token (for source GitLab)"
		m.inputs[0].SetValue(m.cfg.SourceToken)
		m.inputs[0].Focus()

		m.inputs[1] = textinput.New()
		m.inputs[1].Placeholder = "Target username"
		m.inputs[1].SetValue(m.cfg.TargetUsername)

		m.inputs[2] = textinput.New()
		m.inputs[2].Placeholder = "Target password"
		m.inputs[2].EchoMode = textinput.EchoPassword
		m.inputs[2].SetValue(m.cfg.TargetPassword)

		// Hidden dummy
		m.inputs[3] = textinput.New()
		m.inputs[3].Placeholder = ""
	}

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
	case "ctrl+a":
		// Toggle auth mode
		if m.cfg.SourceAuthMode == "basic" {
			m.cfg.SourceAuthMode = "token"
		} else {
			m.cfg.SourceAuthMode = "basic"
		}
		m.initCredentialInputs()
		return app, nil
	case "enter":
		lastReal := len(m.inputs) - 2 // last real field before dummy
		if m.inputFocus >= lastReal {
			m.saveCredentials()
			return app, nil
		}
		m.cycleFocus(false)
	}

	if m.inputFocus < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.inputFocus], cmd = m.inputs[m.inputFocus].Update(msg)
		return app, cmd
	}
	return app, nil
}

func (m *ConfigModel) saveCredentials() {
	if m.cfg.SourceAuthMode == "basic" {
		m.cfg.SourceUsername = m.inputs[0].Value()
		m.cfg.SourcePassword = m.inputs[1].Value()
		m.cfg.TargetUsername = m.inputs[2].Value()
		m.cfg.TargetPassword = m.inputs[3].Value()
		m.cfg.SourceToken = "" // clear token
	} else {
		m.cfg.SourceToken = m.inputs[0].Value()
		m.cfg.TargetUsername = m.inputs[1].Value()
		m.cfg.TargetPassword = m.inputs[2].Value()
		m.cfg.SourceUsername = "" // clear basic
		m.cfg.SourcePassword = ""
	}
	if err := m.cfg.Save(""); err != nil {
		m.message = "Error saving: " + err.Error()
	} else {
		m.message = "Saved!"
	}
}

func (m *ConfigModel) handleSecurity(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		m.cfg.InsecureTLS = !m.cfg.InsecureTLS
		if err := m.cfg.Save(""); err != nil {
			m.message = "Error saving: " + err.Error()
		} else {
			m.message = "Saved!"
		}
	}
	return app, nil
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
	realCount := len(m.inputs) - 1 // exclude dummy last input
	if realCount < 1 {
		realCount = len(m.inputs)
	}

	if backward {
		m.inputFocus--
	} else {
		m.inputFocus++
	}
	if m.inputFocus < 0 {
		m.inputFocus = realCount - 1
	}
	if m.inputFocus >= realCount {
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
	case 4:
		return m.viewSecurity()
	}
	return ""
}

func (m *ConfigModel) viewConfigMenu() string {
	title := titleStyle.Render("Configuration")

	items := []string{
		"GitLab URLs",
		"Credentials",
		"Security (TLS)",
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
	authMode := m.cfg.SourceAuthMode
	if authMode == "" {
		authMode = "token"
	}
	tlsStatus := "Secure (verify certificates)"
	if m.cfg.InsecureTLS {
		tlsStatus = "Insecure (skip verification)"
	}

	summary := "\n" + mutedStyle.Render("Current Configuration:") + "\n"
	summary += fmt.Sprintf("  Source: %s\n", m.cfg.SourceGitLabURL)
	summary += fmt.Sprintf("  Target: %s\n", m.cfg.TargetGitLabURL)
	summary += fmt.Sprintf("  Auth:   %s\n", authMode)
	summary += fmt.Sprintf("  TLS:    %s\n", tlsStatus)
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

	authMode := m.cfg.SourceAuthMode
	if authMode == "" {
		authMode = "token"
	}
	modeLabel := "Token (Private Token / API Key)"
	if authMode == "basic" {
		modeLabel = "Basic Auth (Username + Password)"
	}
	authInfo := statusStyle.Render("Source Auth Mode: "+modeLabel) + "\n\n"

	var form strings.Builder
	if authMode == "basic" {
		labels := []string{"Source Username:", "Source Password:", "Target Username:", "Target Password:"}
		for i := 0; i < 4 && i < len(m.inputs); i++ {
			form.WriteString(labelStyle.Render(labels[i]) + "\n" + m.inputs[i].View() + "\n\n")
		}
	} else {
		labels := []string{"Source Token:", "Target Username:", "Target Password:"}
		for i := 0; i < 3 && i < len(m.inputs); i++ {
			form.WriteString(labelStyle.Render(labels[i]) + "\n" + m.inputs[i].View() + "\n\n")
		}
	}

	msg := ""
	if m.message != "" {
		if strings.HasPrefix(m.message, "Error") {
			msg = errorStyle.Render(m.message)
		} else {
			msg = successStyle.Render(m.message)
		}
	}

	help := helpStyle.Render("Tab: Next  |  Ctrl+A: Toggle auth mode  |  Enter: Save  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + authInfo + form.String() + msg + "\n" + help))
}

func (m *ConfigModel) viewSecurity() string {
	title := titleStyle.Render("Configuration - Security")

	checkbox := "[ ]"
	if m.cfg.InsecureTLS {
		checkbox = "[x]"
	}

	content := fmt.Sprintf(
		"%s  Allow insecure TLS (skip certificate verification)\n\n"+
			"%s\n"+
			"%s",
		checkbox,
		mutedStyle.Render("Enable this for self-signed certificates on air-gapped GitLab instances."),
		mutedStyle.Render("Required when GitLab uses untrusted/internal CA."),
	)

	msg := ""
	if m.message != "" {
		if strings.HasPrefix(m.message, "Error") {
			msg = "\n" + errorStyle.Render(m.message)
		} else {
			msg = "\n" + successStyle.Render(m.message)
		}
	}

	help := helpStyle.Render("Enter/Space: Toggle  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + content + msg + "\n\n" + help))
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
