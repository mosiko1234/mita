package tui

import (
	"fmt"

	"mita/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen identifies which screen is currently displayed.
type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenExportProjectSelect
	ScreenExportBranchSelect
	ScreenExportTransferType
	ScreenExportUSBSelect
	ScreenExportProgress
	ScreenExportSummary
	ScreenImportUSBSelect
	ScreenImportBundleDetect
	ScreenImportMappingCheck
	ScreenImportConfirm
	ScreenImportProgress
	ScreenImportSummary
	ScreenConfig
	ScreenConfigGitLabURL
	ScreenConfigCredentials
	ScreenConfigMappings
)

// App is the main TUI application model.
type App struct {
	version     string
	screen      Screen
	menuCursor  int
	cfg         *config.Config
	width       int
	height      int
	err         error
	exportModel *ExportModel
	importModel *ImportModel
	configModel *ConfigModel
	program     *tea.Program
}

// NewApp creates a new TUI application.
func NewApp(version string) *App {
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	return &App{
		version: version,
		screen:  ScreenMainMenu,
		cfg:     cfg,
		program: nil, // set after tea.NewProgram
	}
}

// SetProgram stores the tea.Program reference for sending async messages.
func (a *App) SetProgram(p *tea.Program) {
	a.program = p
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	}

	// Delegate to sub-models
	switch a.screen {
	case ScreenExportProjectSelect, ScreenExportBranchSelect,
		ScreenExportTransferType, ScreenExportUSBSelect,
		ScreenExportProgress, ScreenExportSummary:
		if a.exportModel != nil {
			return a.exportModel.Update(a, msg)
		}
	case ScreenImportUSBSelect, ScreenImportBundleDetect,
		ScreenImportMappingCheck, ScreenImportConfirm,
		ScreenImportProgress, ScreenImportSummary:
		if a.importModel != nil {
			return a.importModel.Update(a, msg)
		}
	case ScreenConfig, ScreenConfigGitLabURL, ScreenConfigCredentials, ScreenConfigMappings:
		if a.configModel != nil {
			return a.configModel.Update(a, msg)
		}
	}

	return a, nil
}

func (a *App) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.screen {
	case ScreenMainMenu:
		return a.handleMainMenu(msg)
	default:
		// Delegate to sub-models
		switch a.screen {
		case ScreenExportProjectSelect, ScreenExportBranchSelect,
			ScreenExportTransferType, ScreenExportUSBSelect,
			ScreenExportProgress, ScreenExportSummary:
			if a.exportModel != nil {
				return a.exportModel.Update(a, msg)
			}
		case ScreenImportUSBSelect, ScreenImportBundleDetect,
			ScreenImportMappingCheck, ScreenImportConfirm,
			ScreenImportProgress, ScreenImportSummary:
			if a.importModel != nil {
				return a.importModel.Update(a, msg)
			}
		case ScreenConfig, ScreenConfigGitLabURL, ScreenConfigCredentials, ScreenConfigMappings:
			if a.configModel != nil {
				return a.configModel.Update(a, msg)
			}
		}
	}

	return a, nil
}

func (a *App) handleMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	menuItems := 4

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit
	case "up", "k":
		if a.menuCursor > 0 {
			a.menuCursor--
		}
	case "down", "j":
		if a.menuCursor < menuItems-1 {
			a.menuCursor++
		}
	case "enter":
		switch a.menuCursor {
		case 0: // Export
			a.exportModel = NewExportModel(a.cfg)
			a.screen = ScreenExportProjectSelect
			return a, a.exportModel.Init()
		case 1: // Import
			a.importModel = NewImportModel(a.cfg)
			a.screen = ScreenImportUSBSelect
			return a, a.importModel.Init()
		case 2: // Configuration
			a.configModel = NewConfigModel(a.cfg)
			a.screen = ScreenConfig
			return a, nil
		case 3: // Exit
			return a, tea.Quit
		}
	}

	return a, nil
}

func (a *App) View() string {
	switch a.screen {
	case ScreenMainMenu:
		return a.viewMainMenu()
	case ScreenExportProjectSelect, ScreenExportBranchSelect,
		ScreenExportTransferType, ScreenExportUSBSelect,
		ScreenExportProgress, ScreenExportSummary:
		if a.exportModel != nil {
			return a.exportModel.View(a)
		}
	case ScreenImportUSBSelect, ScreenImportBundleDetect,
		ScreenImportMappingCheck, ScreenImportConfirm,
		ScreenImportProgress, ScreenImportSummary:
		if a.importModel != nil {
			return a.importModel.View(a)
		}
	case ScreenConfig, ScreenConfigGitLabURL, ScreenConfigCredentials, ScreenConfigMappings:
		if a.configModel != nil {
			return a.configModel.View(a)
		}
	}

	return ""
}

func (a *App) viewMainMenu() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Render(fmt.Sprintf("MITA v%s", a.version))

	subtitle := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Render("Moses In The Ark")

	menuItems := []string{
		"Export (Internet -> USB)",
		"Import (USB -> Classified)",
		"Configuration",
		"Exit",
	}

	icons := []string{">>", "<<", "**", "XX"}

	menu := ""
	for i, item := range menuItems {
		cursor := "  "
		style := menuItemStyle
		if i == a.menuCursor {
			cursor = "> "
			style = selectedMenuItemStyle
		}
		menu += style.Render(fmt.Sprintf("%s[%d] %s %s", cursor, i+1, icons[i], item)) + "\n"
	}

	help := helpStyle.Render("Up/Down: Navigate  |  Enter: Select  |  q: Quit")

	content := fmt.Sprintf("\n   %s\n   %s\n\n%s\n%s", title, subtitle, menu, help)

	return appStyle.Render(boxStyle.Render(content))
}
