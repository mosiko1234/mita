package main

import (
	"flag"
	"fmt"
	"os"

	"mita/internal/bundle"
	"mita/internal/config"
	"mita/internal/gitlab"
	"mita/internal/tui"
	"mita/internal/usb"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "export":
		runExportCLI(os.Args[2:])
	case "import":
		runImportCLI(os.Args[2:])
	case "version":
		fmt.Printf("MITA %s\n", version)
	case "--version", "-v":
		fmt.Printf("MITA %s\n", version)
	default:
		runTUI()
	}
}

func runTUI() {
	app := tui.NewApp(version)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runExportCLI(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	project := fs.String("project", "", "Project name to export")
	branch := fs.String("branch", "main", "Branch to export")
	depth := fs.String("depth", "shallow", "Transfer type: shallow or full")
	usbPath := fs.String("usb", "", "USB drive path")
	gitlabURL := fs.String("gitlab-url", "", "GitLab URL (overrides config)")
	token := fs.String("token", "", "GitLab private token")

	fs.Parse(args)

	if *project == "" || *usbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: mita export --project <name> --branch <branch> --depth <shallow|full> --usb <path>")
		fs.PrintDefaults()
		os.Exit(1)
	}

	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if *gitlabURL != "" {
		cfg.SourceGitLabURL = *gitlabURL
	}
	if *token != "" {
		cfg.SourceToken = *token
	}

	client, err := gitlab.NewClient(cfg.SourceGitLabURL, cfg.SourceToken, cfg.InsecureTLS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating GitLab client: %v\n", err)
		os.Exit(1)
	}

	drives, err := usb.DetectUSBDrives()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: USB detection failed: %v\n", err)
	}

	found := false
	for _, d := range drives {
		if d.Path == *usbPath {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Warning: %s not detected as a USB drive, proceeding anyway\n", *usbPath)
	}

	shallow := *depth == "shallow"

	fmt.Printf("Exporting project %s (branch: %s, type: %s) to %s\n", *project, *branch, *depth, *usbPath)

	bundlePath, err := bundle.Create(client, *project, *branch, shallow, *usbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Export failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Export complete: %s\n", bundlePath)
}

func runImportCLI(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	usbPath := fs.String("usb", "", "USB drive path")
	auto := fs.Bool("auto", false, "Auto-import using existing mappings")

	fs.Parse(args)

	if *usbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: mita import --usb <path> [--auto]")
		fs.PrintDefaults()
		os.Exit(1)
	}

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	bundles, err := bundle.ScanForBundles(*usbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning USB: %v\n", err)
		os.Exit(1)
	}

	if len(bundles) == 0 {
		fmt.Println("No .mita.zip bundles found on USB")
		return
	}

	fmt.Printf("Found %d bundle(s)\n", len(bundles))

	for _, b := range bundles {
		fmt.Printf("\nImporting: %s\n", b)

		mapping, ok := cfg.Mappings.Lookup(b)
		if !ok && !*auto {
			fmt.Fprintf(os.Stderr, "No mapping found for %s. Use TUI mode to create a mapping.\n", b)
			continue
		}
		if !ok {
			fmt.Fprintf(os.Stderr, "No mapping found for %s, skipping (auto mode)\n", b)
			continue
		}

		targetClient, err := gitlab.NewClient(cfg.TargetGitLabURL, "", cfg.InsecureTLS)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating target GitLab client: %v\n", err)
			continue
		}

		err = bundle.Extract(b, *usbPath, mapping, targetClient, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Import failed for %s: %v\n", b, err)
			continue
		}

		fmt.Printf("Successfully imported: %s → %s/%s\n", b, mapping.TargetGroup, mapping.TargetProject)
	}
}
