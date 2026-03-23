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
	// subScreen: 0=menu, 1=gitlab urls, 2=source auth mode picker,
	//            3=source creds form, 4=target creds form, 5=security, 6=mappings
	subScreen  int
	mappingIdx int
	message    string

	// Auth mode picker
	authModeCursor int // 0=token, 1=basic
}

func NewConfigModel(cfg *config.Config) *ConfigModel {
	authCursor := 0
	if cfg.SourceAuthMode == "basic" {
		authCursor = 1
	}
	return &ConfigModel{
		cfg:            cfg,
		authModeCursor: authCursor,
	}
}

func (m *ConfigModel) Update(app *App, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(app, msg)
	}

	// Update text inputs if active
	if (m.subScreen == 1 || m.subScreen == 3 || m.subScreen == 4) && m.inputs != nil && m.inputFocus < len(m.inputs) {
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
		return m.handleAuthModePicker(app, msg)
	case 3:
		return m.handleSourceCreds(app, msg)
	case 4:
		return m.handleTargetCreds(app, msg)
	case 5:
		return m.handleSecurity(app, msg)
	case 6:
		return m.handleMappings(app, msg)
	}

	return app, nil
}

func (m *ConfigModel) handleConfigMenu(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	menuCount := 6
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
		case 1: // Source Auth Mode
			m.subScreen = 2
			app.screen = ScreenConfigCredentials
		case 2: // Source Credentials
			m.subScreen = 3
			app.screen = ScreenConfigCredentials
			m.initSourceCredInputs()
		case 3: // Target Credentials
			m.subScreen = 4
			app.screen = ScreenConfigCredentials
			m.initTargetCredInputs()
		case 4: // Security (TLS)
			m.subScreen = 5
		case 5: // Back
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

func (m *ConfigModel) initSourceCredInputs() {
	if m.cfg.SourceAuthMode == "basic" {
		m.inputs = make([]textinput.Model, 2)

		m.inputs[0] = textinput.New()
		m.inputs[0].Placeholder = "Source GitLab username"
		m.inputs[0].SetValue(m.cfg.SourceUsername)
		m.inputs[0].Focus()

		m.inputs[1] = textinput.New()
		m.inputs[1].Placeholder = "Source GitLab password"
		m.inputs[1].EchoMode = textinput.EchoPassword
		m.inputs[1].SetValue(m.cfg.SourcePassword)
	} else {
		m.inputs = make([]textinput.Model, 1)

		m.inputs[0] = textinput.New()
		m.inputs[0].Placeholder = "Private token / API key"
		m.inputs[0].SetValue(m.cfg.SourceToken)
		m.inputs[0].Focus()
	}
	m.inputFocus = 0
}

func (m *ConfigModel) initTargetCredInputs() {
	m.inputs = make([]textinput.Model, 2)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "Target GitLab username"
	m.inputs[0].SetValue(m.cfg.TargetUsername)
	m.inputs[0].Focus()

	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "Target GitLab password"
	m.inputs[1].EchoMode = textinput.EchoPassword
	m.inputs[1].SetValue(m.cfg.TargetPassword)

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

func (m *ConfigModel) handleAuthModePicker(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.authModeCursor = 0
	case "down", "j":
		m.authModeCursor = 1
	case "enter":
		if m.authModeCursor == 0 {
			m.cfg.SourceAuthMode = "token"
		} else {
			m.cfg.SourceAuthMode = "basic"
		}
		if err := m.cfg.Save(""); err != nil {
			m.message = "Error saving: " + err.Error()
		} else {
			m.message = "Saved! Now go to 'Source Credentials' to enter your details."
		}
	}
	return app, nil
}

func (m *ConfigModel) handleSourceCreds(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		m.cycleFocus(msg.String() == "shift+tab")
	case "enter":
		if m.inputFocus == len(m.inputs)-1 {
			m.saveSourceCreds()
			return app, nil
		}
		m.cycleFocus(false)
	}

	var cmd tea.Cmd
	m.inputs[m.inputFocus], cmd = m.inputs[m.inputFocus].Update(msg)
	return app, cmd
}

func (m *ConfigModel) saveSourceCreds() {
	if m.cfg.SourceAuthMode == "basic" {
		m.cfg.SourceUsername = m.inputs[0].Value()
		m.cfg.SourcePassword = m.inputs[1].Value()
		m.cfg.SourceToken = ""
	} else {
		m.cfg.SourceToken = m.inputs[0].Value()
		m.cfg.SourceUsername = ""
		m.cfg.SourcePassword = ""
	}
	if err := m.cfg.Save(""); err != nil {
		m.message = "Error saving: " + err.Error()
	} else {
		m.message = "Saved!"
	}
}

func (m *ConfigModel) handleTargetCreds(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		m.cycleFocus(msg.String() == "shift+tab")
	case "enter":
		if m.inputFocus == len(m.inputs)-1 {
			m.cfg.TargetUsername = m.inputs[0].Value()
			m.cfg.TargetPassword = m.inputs[1].Value()
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
	count := len(m.inputs)
	if count < 1 {
		return
	}

	if backward {
		m.inputFocus--
	} else {
		m.inputFocus++
	}
	if m.inputFocus < 0 {
		m.inputFocus = count - 1
	}
	if m.inputFocus >= count {
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

// ======================== Views ========================

func (m *ConfigModel) View(app *App) string {
	switch m.subScreen {
	case 0:
		return m.viewConfigMenu()
	case 1:
		return m.viewGitLabURL()
	case 2:
		return m.viewAuthModePicker()
	case 3:
		return m.viewSourceCreds()
	case 4:
		return m.viewTargetCreds()
	case 5:
		return m.viewSecurity()
	case 6:
		return m.viewMappings()
	}
	return ""
}

func (m *ConfigModel) viewConfigMenu() string {
	title := titleStyle.Render("Configuration")

	authMode := m.cfg.SourceAuthMode
	if authMode == "" {
		authMode = "token"
	}
	authLabel := "Token"
	if authMode == "basic" {
		authLabel = "User+Password"
	}

	items := []string{
		"GitLab URLs",
		fmt.Sprintf("Source Auth Mode  [%s]", authLabel),
		"Source Credentials",
		"Target Credentials",
		"Security (TLS)",
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
	tlsStatus := "Secure"
	if m.cfg.InsecureTLS {
		tlsStatus = "Insecure (self-signed OK)"
	}

	summary := "\n" + mutedStyle.Render("Current Configuration:") + "\n"
	summary += fmt.Sprintf("  Source:   %s\n", m.cfg.SourceGitLabURL)
	summary += fmt.Sprintf("  Target:   %s\n", m.cfg.TargetGitLabURL)
	summary += fmt.Sprintf("  Auth:     %s\n", authLabel)
	summary += fmt.Sprintf("  TLS:      %s\n", tlsStatus)
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

	msg := m.renderMessage()
	help := helpStyle.Render("Tab: Next field  |  Enter: Save  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + form.String() + msg + "\n" + help))
}

func (m *ConfigModel) viewAuthModePicker() string {
	title := titleStyle.Render("Configuration - Source Auth Mode")
	subtitle := subtitleStyle.Render("How do you authenticate with the source GitLab?")

	options := []struct {
		name string
		desc string
	}{
		{"Token (Private Token / API Key)", "Use a personal access token or deploy token"},
		{"Username + Password", "Use your GitLab username and password"},
	}

	menu := ""
	for i, opt := range options {
		cursor := "  "
		style := menuItemStyle
		if i == m.authModeCursor {
			cursor = "> "
			style = selectedMenuItemStyle
		}
		selected := "( )"
		if (i == 0 && m.cfg.SourceAuthMode != "basic") || (i == 1 && m.cfg.SourceAuthMode == "basic") {
			selected = "(*)"
		}
		menu += style.Render(fmt.Sprintf("%s%s %s", cursor, selected, opt.name)) + "\n"
		menu += mutedStyle.Render("       "+opt.desc) + "\n\n"
	}

	msg := m.renderMessage()
	help := helpStyle.Render("Up/Down: Select  |  Enter: Confirm  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n" + subtitle + "\n\n" + menu + msg + "\n" + help))
}

func (m *ConfigModel) viewSourceCreds() string {
	authMode := m.cfg.SourceAuthMode
	if authMode == "" {
		authMode = "token"
	}

	var title string
	var form strings.Builder

	if authMode == "basic" {
		title = titleStyle.Render("Configuration - Source Credentials (User+Password)")
		labels := []string{"Username:", "Password:"}
		for i, input := range m.inputs {
			if i < len(labels) {
				form.WriteString(labelStyle.Render(labels[i]) + "\n" + input.View() + "\n\n")
			}
		}
	} else {
		title = titleStyle.Render("Configuration - Source Credentials (Token)")
		form.WriteString(labelStyle.Render("Private Token:") + "\n" + m.inputs[0].View() + "\n\n")
	}

	msg := m.renderMessage()
	help := helpStyle.Render("Tab: Next field  |  Enter: Save  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + form.String() + msg + "\n" + help))
}

func (m *ConfigModel) viewTargetCreds() string {
	title := titleStyle.Render("Configuration - Target Credentials")

	var form strings.Builder
	labels := []string{"Username:", "Password:"}
	for i, input := range m.inputs {
		if i < len(labels) {
			form.WriteString(labelStyle.Render(labels[i]) + "\n" + input.View() + "\n\n")
		}
	}

	msg := m.renderMessage()
	help := helpStyle.Render("Tab: Next field  |  Enter: Save  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + form.String() + msg + "\n" + help))
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

	msg := m.renderMessage()
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

func (m *ConfigModel) renderMessage() string {
	if m.message == "" {
		return ""
	}
	if strings.HasPrefix(m.message, "Error") {
		return "\n" + errorStyle.Render(m.message)
	}
	return "\n" + successStyle.Render(m.message)
}
