package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"mita/internal/bundle"
	"mita/internal/config"
	"mita/internal/gitlab"
	"mita/internal/tui"
	"mita/internal/usb"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	selfInstall()

	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "export":
		runExportCLI(os.Args[2:])
	case "import":
		runImportCLI(os.Args[2:])
	case "install":
		forceInstall()
	case "clean":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: mita clean <usb-path>")
			os.Exit(1)
		}
		if err := bundle.CleanAndEject(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("USB cleaned and unmounted successfully.")
	case "version":
		fmt.Printf("MITA %s (Moses In The Ark)\n", version)
	case "--version", "-v":
		fmt.Printf("MITA %s (Moses In The Ark)\n", version)
	default:
		runTUI()
	}
}

// selfInstall copies the binary to a system PATH location on first run.
func selfInstall() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}

	var installPath string
	if runtime.GOOS == "windows" {
		// On Windows, install to %LOCALAPPDATA%\mita\mita.exe
		appdata := os.Getenv("LOCALAPPDATA")
		if appdata == "" {
			return
		}
		installPath = filepath.Join(appdata, "mita", "mita.exe")
	} else {
		installPath = "/usr/local/bin/mita"
	}

	// Already running from the install path — nothing to do
	if exe == installPath {
		return
	}

	// Check if installed version exists and compare modification times
	installedInfo, err := os.Stat(installPath)
	if err == nil {
		// Installed version exists — check if current binary is newer
		currentInfo, err := os.Stat(exe)
		if err != nil {
			return
		}
		if !currentInfo.ModTime().After(installedInfo.ModTime()) {
			return // installed version is same or newer, skip
		}
		// Current binary is newer — auto-update
		fmt.Println("Updating MITA installation...")
	}

	doInstall(exe, installPath)
}

func forceInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot determine executable path: %v\n", err)
		os.Exit(1)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot resolve executable path: %v\n", err)
		os.Exit(1)
	}

	var installPath string
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("LOCALAPPDATA")
		if appdata == "" {
			fmt.Fprintln(os.Stderr, "LOCALAPPDATA not set")
			os.Exit(1)
		}
		installPath = filepath.Join(appdata, "mita", "mita.exe")
	} else {
		installPath = "/usr/local/bin/mita"
	}

	if exe == installPath {
		fmt.Println("Already installed at", installPath)
		return
	}

	doInstall(exe, installPath)
}

func doInstall(src, dst string) {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Install: cannot create dir: %v\n", err)
		return
	}

	in, err := os.Open(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Install: cannot open source: %v\n", err)
		return
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		if os.IsPermission(err) && runtime.GOOS != "windows" {
			fmt.Printf("\n  To install MITA globally, run:\n  sudo cp %s %s\n\n", src, dst)
		} else {
			fmt.Fprintf(os.Stderr, "Install: %v\n", err)
		}
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		fmt.Fprintf(os.Stderr, "Install: copy failed: %v\n", err)
		return
	}

	fmt.Printf("Installed MITA to %s — you can now run 'mita' from anywhere.\n", dst)
}

func runTUI() {
	app := tui.NewApp(version)
	p := tea.NewProgram(app, tea.WithAltScreen())
	app.SetProgram(p)
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
	user := fs.String("user", "", "GitLab username (basic auth)")
	pass := fs.String("pass", "", "GitLab password (basic auth)")

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
		cfg.SourceAuthMode = "token"
	}
	if *user != "" {
		cfg.SourceUsername = *user
		cfg.SourceAuthMode = "basic"
	}
	if *pass != "" {
		cfg.SourcePassword = *pass
	}

	var client *gitlab.Client
	if cfg.SourceAuthMode == "basic" {
		client, err = gitlab.NewClientWithBasicAuth(cfg.SourceGitLabURL, cfg.SourceUsername, cfg.SourcePassword, cfg.InsecureTLS)
	} else {
		client, err = gitlab.NewClient(cfg.SourceGitLabURL, cfg.SourceToken, cfg.InsecureTLS)
	}
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

	cliProgress := func(step string) {
		fmt.Println(step)
	}
	bundlePath, err := bundle.Create(client, *project, *branch, shallow, *usbPath, cliProgress)
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
