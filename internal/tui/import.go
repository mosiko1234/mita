package tui

import (
	"fmt"
	"strings"

	"mita/internal/bundle"
	"mita/internal/config"
	"mita/internal/gitlab"
	"mita/internal/usb"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages
type bundlesDetectedMsg struct {
	bundles []string
	err     error
}

type importCompleteMsg struct {
	results []importResult
}

type importResult struct {
	bundleName string
	success    bool
	err        error
}

// ImportModel manages the import workflow screens.
type ImportModel struct {
	cfg        *config.Config
	usbDrives  []usb.USBDrive
	selectedUSB *usb.USBDrive
	bundles    []string
	results    []importResult

	usbList    list.Model
	bundleList list.Model
	spinner    spinner.Model

	// Mapping form
	mappingInputs   []textinput.Model
	mappingFocus    int
	currentBundle   string
	pendingManifest *bundle.Manifest

	// State
	loading     bool
	confirmIdx  int
	err         error
}

func NewImportModel(cfg *config.Config) *ImportModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	return &ImportModel{
		cfg:     cfg,
		spinner: s,
		loading: true,
	}
}

func (m *ImportModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		detectUSBDrivesCmd(),
	)
}

func (m *ImportModel) scanBundles() tea.Cmd {
	return func() tea.Msg {
		bundles, err := bundle.ScanForBundles(m.selectedUSB.Path)
		return bundlesDetectedMsg{bundles: bundles, err: err}
	}
}

func (m *ImportModel) runImport() tea.Cmd {
	return func() tea.Msg {
		var results []importResult

		for _, bundleName := range m.bundles {
			// Extract project name from bundle filename
			projName := extractProjectName(bundleName)
			mapping, ok := m.cfg.Mappings.Lookup(projName)
			if !ok {
				results = append(results, importResult{
					bundleName: bundleName,
					err:        fmt.Errorf("no mapping found for project %q", projName),
				})
				continue
			}

			client, err := gitlab.NewClientWithBasicAuth(
				m.cfg.TargetGitLabURL,
				m.cfg.TargetUsername,
				m.cfg.TargetPassword,
				m.cfg.InsecureTLS,
			)
			if err != nil {
				results = append(results, importResult{
					bundleName: bundleName,
					err:        err,
				})
				continue
			}

			err = bundle.Extract(bundleName, m.selectedUSB.Path, mapping, client, m.cfg)
			results = append(results, importResult{
				bundleName: bundleName,
				success:    err == nil,
				err:        err,
			})
		}

		return importCompleteMsg{results: results}
	}
}

func (m *ImportModel) refreshBundleList() {
	items := make([]list.Item, len(m.bundles))
	for i, b := range m.bundles {
		projName := extractProjectName(b)
		desc := "⚠ No mapping — press M to set target"
		if mapping, ok := m.cfg.Mappings.Lookup(projName); ok {
			desc = fmt.Sprintf("→ %s/%s", mapping.TargetGroup, mapping.TargetProject)
		}
		items[i] = listItem{title: b, desc: desc}
	}
	m.bundleList = list.New(items, list.NewDefaultDelegate(), 80, 15)
	m.bundleList.Title = "Detected Bundles (press M to set mapping, Enter to import)"
}

func extractProjectName(bundleName string) string {
	// Format: <project-name>-<timestamp>.mita.zip
	name := strings.TrimSuffix(bundleName, ".mita.zip")
	// Remove timestamp suffix (format: -YYYYMMDD-HHMMSS)
	if idx := strings.LastIndex(name, "-"); idx > 0 {
		prefix := name[:idx]
		if idx2 := strings.LastIndex(prefix, "-"); idx2 > 0 {
			// Check if the last two parts look like date-time
			return prefix[:idx2]
		}
	}
	return name
}

func (m *ImportModel) Update(app *App, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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

	case bundlesDetectedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return app, nil
		}
		m.bundles = msg.bundles
		items := make([]list.Item, len(msg.bundles))
		for i, b := range msg.bundles {
			projName := extractProjectName(b)
			_, hasMapping := m.cfg.Mappings.Lookup(projName)
			desc := "⚠ No mapping — press M to set target"
			if hasMapping {
				mapping, _ := m.cfg.Mappings.Lookup(projName)
				desc = fmt.Sprintf("→ %s/%s", mapping.TargetGroup, mapping.TargetProject)
			}
			items[i] = listItem{title: b, desc: desc}
		}
		m.bundleList = list.New(items, list.NewDefaultDelegate(), 80, 15)
		m.bundleList.Title = "Detected Bundles (press M to set mapping, Enter to import)"
		app.screen = ScreenImportBundleDetect
		return app, nil

	case importCompleteMsg:
		m.loading = false
		m.results = msg.results
		app.screen = ScreenImportSummary
		return app, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return app, cmd

	case tea.KeyMsg:
		return m.handleKey(app, msg)
	}

	// Update lists
	var cmd tea.Cmd
	switch app.screen {
	case ScreenImportUSBSelect:
		m.usbList, cmd = m.usbList.Update(msg)
	case ScreenImportBundleDetect:
		m.bundleList, cmd = m.bundleList.Update(msg)
	}

	return app, cmd
}

func (m *ImportModel) handleKey(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return app, tea.Quit
	case "esc":
		switch app.screen {
		case ScreenImportUSBSelect:
			app.screen = ScreenMainMenu
			return app, nil
		case ScreenImportBundleDetect:
			app.screen = ScreenImportUSBSelect
			return app, nil
		case ScreenImportConfirm:
			app.screen = ScreenImportBundleDetect
			return app, nil
		case ScreenImportSummary:
			app.screen = ScreenMainMenu
			return app, nil
		}
	}

	switch app.screen {
	case ScreenImportUSBSelect:
		if msg.String() == "enter" {
			if item, ok := m.usbList.SelectedItem().(listItem); ok {
				for i, d := range m.usbDrives {
					if d.Label == item.title {
						m.selectedUSB = &m.usbDrives[i]
						break
					}
				}
				m.loading = true
				return app, tea.Batch(m.spinner.Tick, m.scanBundles())
			}
		}
		var cmd tea.Cmd
		m.usbList, cmd = m.usbList.Update(msg)
		return app, cmd

	case ScreenImportBundleDetect:
		switch msg.String() {
		case "enter":
			// Check if all bundles have mappings
			allMapped := true
			for _, b := range m.bundles {
				projName := extractProjectName(b)
				if _, ok := m.cfg.Mappings.Lookup(projName); !ok {
					allMapped = false
					break
				}
			}
			if allMapped {
				app.screen = ScreenImportConfirm
			} else {
				m.err = fmt.Errorf("some bundles have no mapping — press M on each to set target")
			}
			return app, nil
		case "m", "M":
			// Create mapping for selected bundle
			if item, ok := m.bundleList.SelectedItem().(listItem); ok {
				projName := extractProjectName(item.title)
				m.currentBundle = item.title
				m.initMappingForm(projName)
				app.screen = ScreenImportMappingCheck
			}
			return app, nil
		}
		var cmd tea.Cmd
		m.bundleList, cmd = m.bundleList.Update(msg)
		return app, cmd

	case ScreenImportConfirm:
		switch msg.String() {
		case "y", "Y", "enter":
			app.screen = ScreenImportProgress
			m.loading = true
			return app, tea.Batch(m.spinner.Tick, m.runImport())
		case "n", "N":
			app.screen = ScreenImportBundleDetect
			return app, nil
		}

	case ScreenImportMappingCheck:
		return m.handleMappingInput(app, msg)

	case ScreenImportSummary:
		if msg.String() == "enter" || msg.String() == "esc" {
			app.screen = ScreenMainMenu
			return app, nil
		}
	}

	return app, nil
}

func (m *ImportModel) handleMappingInput(app *App, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		if msg.String() == "tab" {
			m.mappingFocus++
		} else {
			m.mappingFocus--
		}
		if m.mappingFocus < 0 {
			m.mappingFocus = len(m.mappingInputs) - 1
		}
		if m.mappingFocus >= len(m.mappingInputs) {
			m.mappingFocus = 0
		}
		for i := range m.mappingInputs {
			if i == m.mappingFocus {
				m.mappingInputs[i].Focus()
			} else {
				m.mappingInputs[i].Blur()
			}
		}
	case "enter":
		if m.mappingFocus == len(m.mappingInputs)-1 {
			// Save mapping
			projName := extractProjectName(m.currentBundle)
			if m.mappingInputs[1].Value() == "" {
				return app, nil // group is required
			}
			targetProj := m.mappingInputs[0].Value()
			if targetProj == "" {
				targetProj = projName
			}
			branch := m.mappingInputs[2].Value()
			if branch == "" {
				branch = "main"
			}
			m.cfg.Mappings.Set(projName, config.MappingEntry{
				TargetProject: targetProj,
				TargetGroup:   m.mappingInputs[1].Value(),
				TargetBranch:  branch,
			})
			m.cfg.Save("")
			// Refresh bundle list with updated mapping status
			m.refreshBundleList()
			app.screen = ScreenImportBundleDetect
			return app, nil
		}
		m.mappingFocus++
		for i := range m.mappingInputs {
			if i == m.mappingFocus {
				m.mappingInputs[i].Focus()
			} else {
				m.mappingInputs[i].Blur()
			}
		}
	}

	var cmd tea.Cmd
	m.mappingInputs[m.mappingFocus], cmd = m.mappingInputs[m.mappingFocus].Update(msg)
	return app, cmd
}

func (m *ImportModel) initMappingForm(projName string) {
	m.mappingFocus = 0

	inputs := make([]textinput.Model, 3)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Target project name"
	inputs[0].SetValue(projName) // pre-fill with source project name
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Target group (e.g., group/subgroup)"

	// Pre-fill from existing mapping if available
	if mapping, ok := m.cfg.Mappings.Lookup(projName); ok {
		inputs[0].SetValue(mapping.TargetProject)
		inputs[1].SetValue(mapping.TargetGroup)
	}

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Target branch (default: main)"
	inputs[2].SetValue("main")

	m.mappingInputs = inputs
}

func (m *ImportModel) View(app *App) string {
	switch app.screen {
	case ScreenImportUSBSelect:
		if m.loading {
			return appStyle.Render(
				titleStyle.Render("Import") + "\n\n" +
					m.spinner.View() + " Detecting USB drives...")
		}
		if len(m.usbDrives) == 0 {
			return appStyle.Render(
				titleStyle.Render("Import - USB Selection") + "\n\n" +
					warningStyle.Render("No USB drives detected") + "\n\n" +
					helpStyle.Render("Insert a USB drive and press Esc to retry"))
		}
		return appStyle.Render(m.usbList.View())

	case ScreenImportBundleDetect:
		if m.loading {
			return appStyle.Render(
				titleStyle.Render("Import") + "\n\n" +
					m.spinner.View() + " Scanning for bundles...")
		}
		if len(m.bundles) == 0 {
			return appStyle.Render(
				titleStyle.Render("Import - Bundle Detection") + "\n\n" +
					warningStyle.Render("No .mita.zip bundles found") + "\n\n" +
					helpStyle.Render("Press Esc to go back"))
		}
		return appStyle.Render(m.bundleList.View())

	case ScreenImportMappingCheck:
		return m.viewMappingForm()

	case ScreenImportConfirm:
		return m.viewConfirm()

	case ScreenImportProgress:
		return appStyle.Render(
			titleStyle.Render("Import - In Progress") + "\n\n" +
				m.spinner.View() + " Importing bundles..." + "\n\n" +
				helpStyle.Render("Please wait..."))

	case ScreenImportSummary:
		return m.viewSummary()
	}

	return ""
}

func (m *ImportModel) viewMappingForm() string {
	title := titleStyle.Render("Import - New Project Mapping")
	subtitle := subtitleStyle.Render(fmt.Sprintf("No mapping found for: %s", m.currentBundle))

	var form strings.Builder
	labels := []string{"Target Project:", "Target Group:", "Target Branch:"}
	for i, input := range m.mappingInputs {
		form.WriteString(labelStyle.Render(labels[i]) + " " + input.View() + "\n")
	}

	help := helpStyle.Render("Tab: Next field  |  Enter: Save  |  Esc: Cancel")

	return appStyle.Render(boxStyle.Render(title + "\n" + subtitle + "\n\n" + form.String() + "\n" + help))
}

func (m *ImportModel) viewConfirm() string {
	title := titleStyle.Render("Import - Confirm")

	var details strings.Builder
	details.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("USB Drive:"), valueStyle.Render(m.selectedUSB.Label)))
	details.WriteString(fmt.Sprintf("%s %d\n\n", labelStyle.Render("Bundles:"), len(m.bundles)))

	for _, b := range m.bundles {
		projName := extractProjectName(b)
		mapping, ok := m.cfg.Mappings.Lookup(projName)
		if ok {
			details.WriteString(fmt.Sprintf("  %s -> %s/%s\n", b, mapping.TargetGroup, mapping.TargetProject))
		} else {
			details.WriteString(fmt.Sprintf("  %s -> %s\n", b, warningStyle.Render("no mapping")))
		}
	}

	help := helpStyle.Render("Y: Confirm import  |  N: Cancel  |  Esc: Back")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + details.String() + "\n" + help))
}

func (m *ImportModel) viewSummary() string {
	title := titleStyle.Render("Import - Complete")

	var details strings.Builder
	successCount := 0
	failCount := 0

	for _, r := range m.results {
		if r.success {
			successCount++
			details.WriteString(successStyle.Render("OK") + " " + r.bundleName + "\n")
		} else {
			failCount++
			errMsg := "unknown error"
			if r.err != nil {
				errMsg = r.err.Error()
			}
			details.WriteString(errorStyle.Render("FAIL") + " " + r.bundleName + ": " + errMsg + "\n")
		}
	}

	summary := fmt.Sprintf("\n%s %d succeeded, %d failed",
		labelStyle.Render("Results:"),
		successCount,
		failCount,
	)

	help := helpStyle.Render("Press Enter or Esc to return to main menu")

	return appStyle.Render(boxStyle.Render(title + "\n\n" + details.String() + summary + "\n\n" + help))
}
