package tui

import (
	"fmt"
	"strings"

	"mita/internal/bundle"
	"mita/internal/config"
	"mita/internal/gitlab"
	"mita/internal/usb"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages for async operations
type projectsLoadedMsg struct {
	projects []gitlab.Project
	err      error
}

type branchesLoadedMsg struct {
	branches []gitlab.Branch
	err      error
}

type usbDrivesDetectedMsg struct {
	drives []usb.USBDrive
	err    error
}

type exportCompleteMsg struct {
	bundlePath string
	err        error
}

type exportProgressMsg struct {
	step    string
	percent float64
}

type usbEjectMsg struct {
	err error
}

// listItem implements list.Item for the Bubbles list component.
type listItem struct {
	title string
	desc  string
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

// ExportModel manages the export workflow screens.
type ExportModel struct {
	cfg           *config.Config
	client        *gitlab.Client
	projects      []gitlab.Project
	branches      []gitlab.Branch
	usbDrives     []usb.USBDrive
	selectedProj  *gitlab.Project
	selectedBranch string
	shallow       bool
	selectedUSB   *usb.USBDrive
	bundlePath    string

	projectList list.Model
	branchList  list.Model
	usbList     list.Model
	searchInput textinput.Model
	progress    progress.Model
	spinner     spinner.Model

	transferCursor int
	summaryCursor  int
	ejected        bool
	progressStep   string
	progressPct    float64
	err            error
	loading        bool
}

func NewExportModel(cfg *config.Config) *ExportModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	p := progress.New(progress.WithDefaultGradient())

	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.Focus()

	return &ExportModel{
		cfg:      cfg,
		spinner:  s,
		progress: p,
		searchInput: ti,
		shallow:  true,
		loading:  true,
	}
}

func (m *ExportModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadProjects(),
	)
}

func (m *ExportModel) loadProjects() tea.Cmd {
	return func() tea.Msg {
		var client *gitlab.Client
		var err error

		if m.cfg.SourceAuthMode == "basic" {
			client, err = gitlab.NewClientWithBasicAuth(
				m.cfg.SourceGitLabURL,
				m.cfg.SourceUsername,
				m.cfg.SourcePassword,
				m.cfg.InsecureTLS,
			)
		} else {
			client, err = gitlab.NewClient(m.cfg.SourceGitLabURL, m.cfg.SourceToken, m.cfg.InsecureTLS)
		}
		if err != nil {
			return projectsLoadedMsg{err: err}
		}
		m.client = client

		projects, err := client.ListProjects("")
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func (m *ExportModel) loadBranches() tea.Cmd {
	return func() tea.Msg {
		branches, err := m.client.ListBranches(m.selectedProj.ID)
		return branchesLoadedMsg{branches: branches, err: err}
	}
}

func detectUSBDrivesCmd() tea.Cmd {
	return func() tea.Msg {
		drives, err := usb.DetectUSBDrives()
		return usbDrivesDetectedMsg{drives: drives, err: err}
	}
}

func (m *ExportModel) runExport(p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		progress := func(step string) {
			if p != nil {
				p.Send(exportProgressMsg{step: step})
			}
		}

		bundlePath, err := bundle.Create(
			m.client,
			m.selectedProj.Name,
			m.selectedBranch,
			m.shallow,
			m.selectedUSB.Path,
			progress,
		)
		return exportCompleteMsg{bundlePath: bundlePath, err: err}
	}
}

func (m *ExportModel) Update(app *App, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return app, nil
		}
		m.projects = msg.projects
		items := make([]list.Item, len(msg.projects))
		for i, p := range msg.projects {
			items[i] = listItem{title: p.Name, desc: p.PathWithNS}
		}
		m.projectList = list.New(items, list.NewDefaultDelegate(), 60, 20)
		m.projectList.Title = "Select Project"
		m.projectList.SetShowStatusBar(true)
		m.projectList.SetFilteringEnabled(true)
		return app, nil

	case branchesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return app, nil
		}
		m.branches = msg.branches
		items := make([]list.Item, len(msg.branches))
		for i, b := range msg.branches {
			items[i] = listItem{title: b.Name, desc: b.Commit[:8]}
		}
		m.branchList = list.New(items, list.NewDefaultDelegate(), 60, 20)
		m.branchList.Title = "Select Branch"
		m.branchList.SetFilteringEnabled(true)
		return app, nil

	case usbDrivesDetectedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		}
		m.usbDrives = msg.drives
		items := make([]list.Item, len(msg.drives))
		for i, d := range msg.drives {
			items[i] = listItem{title: d.Label, desc: d.String()}
		}
		m.usbList = list.New(items, list.NewDefaultDelegate(), 60, 15)
		m.usbList.Title = "Select USB Drive"
		return app, nil

	case exportProgressMsg:
		m.progressStep = msg.step
		return app, nil

	case exportCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return app, nil
		}
		m.bundlePath = msg.bundlePath
		m.summaryCursor = 0
		m.ejected = false
		app.screen = ScreenExportSummary
		return app, nil

	case usbEjectMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.ejected = true
		}
		return app, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return app, cmd

	case tea.KeyMsg:
		return m.handleKey(app, msg)
	}

	// Update active list
	var cmd tea.Cmd
	switch app.screen {
	case ScreenExportProjectSelect:
		m.projectList, cmd = m.projectList.Update(msg)
	case ScreenExportBranchSelect:
		m.branchList, cmd = m.branchList.Update(msg)
	case ScreenExportUSBSelect:
		m.usbList, cmd = m.usbList.Update(msg)
	}

	return app, cmd
}

func (m *ExportModel) handleKey(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return app, tea.Quit
	case "esc":
		switch app.screen {
		case ScreenExportProjectSelect:
			app.screen = ScreenMainMenu
			return app, nil
		case ScreenExportBranchSelect:
			app.screen = ScreenExportProjectSelect
			return app, nil
		case ScreenExportTransferType:
			app.screen = ScreenExportBranchSelect
			return app, nil
		case ScreenExportUSBSelect:
			app.screen = ScreenExportTransferType
			return app, nil
		case ScreenExportSummary:
			if !m.loading {
				app.screen = ScreenMainMenu
				return app, nil
			}
		}
	}

	// Handle summary screen menu
	if app.screen == ScreenExportSummary && !m.loading {
		return m.handleSummaryKey(app, msg)
	}

	switch app.screen {
	case ScreenExportProjectSelect:
		if msg.String() == "enter" {
			if item, ok := m.projectList.SelectedItem().(listItem); ok {
				for i, p := range m.projects {
					if p.Name == item.title {
						m.selectedProj = &m.projects[i]
						break
					}
				}
				app.screen = ScreenExportBranchSelect
				m.loading = true
				return app, tea.Batch(m.spinner.Tick, m.loadBranches())
			}
		}
		var cmd tea.Cmd
		m.projectList, cmd = m.projectList.Update(msg)
		return app, cmd

	case ScreenExportBranchSelect:
		if msg.String() == "enter" {
			if item, ok := m.branchList.SelectedItem().(listItem); ok {
				m.selectedBranch = item.title
				app.screen = ScreenExportTransferType
				return app, nil
			}
		}
		var cmd tea.Cmd
		m.branchList, cmd = m.branchList.Update(msg)
		return app, cmd

	case ScreenExportTransferType:
		switch msg.String() {
		case "up", "k":
			m.transferCursor = 0
			m.shallow = true
		case "down", "j":
			m.transferCursor = 1
			m.shallow = false
		case "enter":
			app.screen = ScreenExportUSBSelect
			m.loading = true
			return app, tea.Batch(m.spinner.Tick, detectUSBDrivesCmd())
		}

	case ScreenExportUSBSelect:
		if msg.String() == "enter" {
			if item, ok := m.usbList.SelectedItem().(listItem); ok {
				for i, d := range m.usbDrives {
					if d.Label == item.title {
						m.selectedUSB = &m.usbDrives[i]
						break
					}
				}
				app.screen = ScreenExportProgress
				m.loading = true
				m.progressStep = "Starting export..."
				return app, tea.Batch(m.spinner.Tick, m.runExport(app.program))
			}
		}
		var cmd tea.Cmd
		m.usbList, cmd = m.usbList.Update(msg)
		return app, cmd
	}

	return app, nil
}

func (m *ExportModel) View(app *App) string {
	switch app.screen {
	case ScreenExportProjectSelect:
		if m.loading {
			return appStyle.Render(
				titleStyle.Render("Export") + "\n\n" +
					m.spinner.View() + " Loading projects...")
		}
		if m.err != nil {
			return appStyle.Render(
				titleStyle.Render("Export") + "\n\n" +
					errorStyle.Render("Error: "+m.err.Error()) + "\n\n" +
					helpStyle.Render("Press Esc to go back"))
		}
		return appStyle.Render(m.projectList.View())

	case ScreenExportBranchSelect:
		if m.loading {
			return appStyle.Render(
				titleStyle.Render("Export - Branch Selection") + "\n\n" +
					m.spinner.View() + " Loading branches...")
		}
		return appStyle.Render(m.branchList.View())

	case ScreenExportTransferType:
		return m.viewTransferType()

	case ScreenExportUSBSelect:
		if m.loading {
			return appStyle.Render(
				titleStyle.Render("Export - USB Selection") + "\n\n" +
					m.spinner.View() + " Detecting USB drives...")
		}
		if len(m.usbDrives) == 0 {
			return appStyle.Render(
				titleStyle.Render("Export - USB Selection") + "\n\n" +
					warningStyle.Render("No USB drives detected") + "\n\n" +
					helpStyle.Render("Insert a USB drive and press Esc to retry"))
		}
		return appStyle.Render(m.usbList.View())

	case ScreenExportProgress:
		return appStyle.Render(
			titleStyle.Render("Export - In Progress") + "\n\n" +
				m.spinner.View() + " " + m.progressStep + "\n\n" +
				helpStyle.Render("Please wait..."))

	case ScreenExportSummary:
		return m.viewSummary()
	}

	return ""
}

func (m *ExportModel) viewTransferType() string {
	title := titleStyle.Render("Export - Transfer Type")

	options := []struct {
		name string
		desc string
	}{
		{"Shallow (latest commit only)", "Faster, smaller bundle. Good for routine updates."},
		{"Full (complete history)", "Larger bundle. Includes all commits, branches, and tags."},
	}

	menu := ""
	for i, opt := range options {
		cursor := "  "
		style := menuItemStyle
		if i == m.transferCursor {
			cursor = "> "
			style = selectedMenuItemStyle
		}
		indicator := "( )"
		if (i == 0 && m.shallow) || (i == 1 && !m.shallow) {
			indicator = "(*)"
		}
		menu += style.Render(fmt.Sprintf("%s%s %s", cursor, indicator, opt.name)) + "\n"
		menu += lipgloss.NewStyle().PaddingLeft(8).Foreground(mutedColor).Render(opt.desc) + "\n"
	}

	help := helpStyle.Render("Up/Down: Select  |  Enter: Confirm  |  Esc: Back")

	return appStyle.Render(title + "\n\n" + menu + "\n" + help)
}

func (m *ExportModel) handleSummaryKey(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	menuCount := 3
	if m.ejected {
		menuCount = 2 // no eject option after ejection
	}

	switch msg.String() {
	case "up", "k":
		if m.summaryCursor > 0 {
			m.summaryCursor--
		}
	case "down", "j":
		if m.summaryCursor < menuCount-1 {
			m.summaryCursor++
		}
	case "enter":
		if m.ejected {
			switch m.summaryCursor {
			case 0: // Export another project (new USB needed)
				app.screen = ScreenExportProjectSelect
				m.loading = true
				return app, tea.Batch(m.spinner.Tick, m.loadProjects())
			case 1: // Main menu
				app.screen = ScreenMainMenu
			}
		} else {
			switch m.summaryCursor {
			case 0: // Export another to same USB
				app.screen = ScreenExportProjectSelect
				m.loading = true
				return app, tea.Batch(m.spinner.Tick, m.loadProjects())
			case 1: // Clean & Eject USB
				m.loading = true
				return app, tea.Batch(m.spinner.Tick, m.cleanAndEject())
			case 2: // Main menu
				app.screen = ScreenMainMenu
			}
		}
	}
	return app, nil
}

func (m *ExportModel) cleanAndEject() tea.Cmd {
	return func() tea.Msg {
		err := bundle.CleanAndEject(m.selectedUSB.Path)
		return usbEjectMsg{err: err}
	}
}

func (m *ExportModel) viewSummary() string {
	title := titleStyle.Render("Export - Complete")
	status := successStyle.Render("Bundle created successfully!")

	var details strings.Builder
	details.WriteString(fmt.Sprintf("\n%s %s\n", labelStyle.Render("Project:"), valueStyle.Render(m.selectedProj.Name)))
	details.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Branch:"), valueStyle.Render(m.selectedBranch)))
	transferType := "Shallow"
	if !m.shallow {
		transferType = "Full"
	}
	details.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Type:"), valueStyle.Render(transferType)))
	details.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("USB Drive:"), valueStyle.Render(m.selectedUSB.Label)))
	details.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Bundle:"), valueStyle.Render(m.bundlePath)))

	if m.err != nil {
		status = errorStyle.Render("Export failed: " + m.err.Error())
	}

	// Loading state (ejecting)
	if m.loading {
		return appStyle.Render(boxStyle.Render(
			title + "\n\n" + status + "\n" + details.String() + "\n\n" +
				m.spinner.View() + " Cleaning and ejecting USB..."))
	}

	// After eject attempt
	if m.ejected {
		ejectStatus := "\n" + successStyle.Render("✓ USB cleaned and ejected safely. You can remove it now.") + "\n"

		var menu string
		items := []string{"Export another project", "Back to main menu"}
		for i, item := range items {
			cursor := "  "
			style := menuItemStyle
			if i == m.summaryCursor {
				cursor = "> "
				style = selectedMenuItemStyle
			}
			menu += style.Render(fmt.Sprintf("%s%s", cursor, item)) + "\n"
		}

		help := helpStyle.Render("Up/Down: Navigate  |  Enter: Select  |  Esc: Main menu")
		return appStyle.Render(boxStyle.Render(title + "\n\n" + status + "\n" + details.String() + ejectStatus + "\n" + menu + "\n" + help))
	}

	// Eject failed — show error
	if m.err != nil && !m.ejected {
		ejectErr := "\n" + errorStyle.Render("Eject failed: "+m.err.Error()) + "\n"
		help := helpStyle.Render("Press Esc to return to main menu")
		return appStyle.Render(boxStyle.Render(title + "\n\n" + status + "\n" + details.String() + ejectErr + "\n" + help))
	}

	// Normal summary with action menu
	var menu string
	items := []string{
		"Export another project to same USB",
		"Clean & Eject USB (remove macOS hidden files)",
		"Back to main menu",
	}
	for i, item := range items {
		cursor := "  "
		style := menuItemStyle
		if i == m.summaryCursor {
			cursor = "> "
			style = selectedMenuItemStyle
		}
		menu += style.Render(fmt.Sprintf("%s%s", cursor, item)) + "\n"
	}

	help := helpStyle.Render("Up/Down: Navigate  |  Enter: Select  |  Esc: Main menu")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + status + "\n" + details.String() + "\n" + menu + "\n" + help))
}
