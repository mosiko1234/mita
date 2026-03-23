# AirLift

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Automated Code Transfer System for Air-Gapped Networks**

Architecture Document — March 2026 — Version 1.0

---

## Table of Contents

1. [Overview](#1-overview)
2. [High-Level Architecture](#2-high-level-architecture)
3. [Project Structure](#3-project-structure)
4. [Bundle Format](#4-bundle-format)
5. [Export Module — Internet Side](#5-export-module--internet-side)
6. [Import Module — Classified Network Side](#6-import-module--classified-network-side)
7. [Cross-Platform USB Detection](#7-cross-platform-usb-detection)
8. [Security & Integrity](#8-security--integrity)
9. [Build & Cross Compilation](#9-build--cross-compilation)
10. [Terminal UI Design](#10-terminal-ui-design)
11. [Go Dependencies](#11-go-dependencies)
12. [Usage Scenarios](#12-usage-scenarios)
13. [Future Considerations](#13-future-considerations)

---

## 1. Overview

In many organizations working on classified projects, there is a constant need to transfer source code between two isolated networks: the internet network (connected to the outside world) and a classified air-gapped network with no external connectivity. The only way to transfer data between these networks is via a USB device (Disk-on-Key) through a dedicated intermediary computer.

Today, the code transfer process is performed manually — cloning a project from GitLab on the internet network, copying it to USB, and manually uploading it to GitLab on the classified network. This process is time-consuming, error-prone, and does not properly handle Git LFS, version history, or data integrity.

**AirLift** is a CLI + TUI tool written in Go that enables automated and secure code transfer between two GitLab instances via USB. The tool is delivered as a single binary with zero external dependencies, supports macOS and Windows, and provides an interactive Terminal UI powered by Bubbletea.

### Key Capabilities

- Package a complete Git project (including LFS) into a secure bundle on USB
- Automated bundle extraction and push to target GitLab
- Support for shallow (latest commit only) or full history transfer
- Automatic USB drive detection
- Project name mapping between networks (Dictionary Mapping)
- Integrity verification using SHA-256 checksums
- Interactive TUI with search, filtering, and progress bars

### Network Comparison

| Detail | Internet Network | Classified Network |
|--------|-----------------|-------------------|
| GitLab URL | `https://192.168.102.104/` | Configured in settings |
| TLS | Self-signed (insecure) | Secure |
| Authentication | Private Token / User+Password | User + Password |
| Operating System | macOS / Windows | macOS / Windows |

---

## 2. High-Level Architecture

The system is built as a single binary operating in two modes:

- **Export Mode** (Internet side) — Connects to the source GitLab, creates a bundle package on USB
- **Import Mode** (Classified side) — Reads the bundle from USB, pushes to the target GitLab

The user runs the same executable on both networks. The only difference is the selected operating mode. Below is a schematic diagram of the workflow:

```
┌─────────────────────┐     USB      ┌──────────────────────┐
│   Internet Network  │  ========>   │  Classified Network  │
│                     │  Disk-on-Key │                      │
│  GitLab (Source)    │              │  GitLab (Target)     │
│  192.168.102.104    │              │  <configured URL>    │
│         │           │              │         ▲            │
│         ▼           │              │         │            │
│  ┌─────────────┐    │              │  ┌─────────────┐     │
│  │  AirLift    │    │              │  │  AirLift    │     │
│  │  (Export)   │────┼──────────────┼──│  (Import)   │     │
│  │  TUI + CLI  │    │              │  │  TUI + CLI  │     │
│  └─────────────┘    │              │  └─────────────┘     │
└─────────────────────┘              └──────────────────────┘
```

### Architectural Principles

- **Zero Dependencies** — Single binary with no additional software installation required (except `git` and `git-lfs`)
- **Cross-Platform** — Full support for macOS (Intel + Apple Silicon) and Windows (amd64)
- **Offline-First** — All logic operates locally, no internet connection needed during bundle transfer
- **Security by Design** — Integrity verification, separation of classified and unclassified data, no credentials stored in bundles
- **Idempotent** — Import can be run multiple times without damaging existing data

---

## 3. Project Structure

The directory structure follows standard Go project layout with clear module separation:

```
airlift/
├── cmd/
│   └── airlift/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   ├── config.go            # Configuration management
│   │   └── mapping.go           # Project name mapping
│   ├── gitlab/
│   │   ├── client.go            # GitLab API client (supports insecure TLS)
│   │   ├── clone.go             # Git clone/fetch operations
│   │   └── push.go              # Git push operations
│   ├── bundle/
│   │   ├── create.go            # Create transfer bundle
│   │   ├── extract.go           # Extract transfer bundle
│   │   ├── metadata.go          # Bundle metadata handling
│   │   └── lfs.go               # Git LFS handling
│   ├── usb/
│   │   ├── detect.go            # Cross-platform USB detection interface
│   │   ├── detect_darwin.go     # macOS-specific USB detection
│   │   └── detect_windows.go    # Windows-specific USB detection
│   ├── tui/
│   │   ├── app.go               # Main TUI application (Bubbletea)
│   │   ├── export.go            # Export mode screens
│   │   ├── import.go            # Import mode screens
│   │   ├── config_screen.go     # Configuration screens
│   │   └── styles.go            # Lipgloss styles
│   └── crypto/
│       └── integrity.go         # SHA-256 checksums
├── go.mod
├── go.sum
├── Makefile                     # Cross-compilation targets
└── README.md
```

### Module Descriptions

| Module | Description |
|--------|-------------|
| `cmd/airlift` | Main entry point — argument parsing, TUI or CLI initialization |
| `internal/config` | Configuration management — reading/writing settings, project name mapping |
| `internal/gitlab` | GitLab API client — project listing, clone, push, self-signed TLS support |
| `internal/bundle` | Bundle creation and extraction — ZIP archive with Git bundle, LFS objects, and metadata |
| `internal/usb` | USB drive detection — unified interface with OS-specific implementations |
| `internal/tui` | Terminal UI — screens, navigation, styles (Bubbletea + Lipgloss + Bubbles) |
| `internal/crypto` | SHA-256 checksum calculation and verification for data integrity |

---

## 4. Bundle Format

The bundle is a ZIP file with the `.airlift.zip` extension containing all information needed to transfer a complete Git project, including LFS objects:

```
<project-name>-<timestamp>.airlift.zip
├── manifest.json                # Metadata about the transfer
├── repo.bundle                  # Git bundle file (full or shallow)
├── lfs/                         # Git LFS objects directory
│   ├── <oid1>                   # LFS object files by OID
│   ├── <oid2>
│   └── ...
└── checksum.sha256              # SHA-256 checksums of all files
```

### manifest.json — Structure

The manifest file contains metadata about the transfer, allowing the target side to identify the source project and perform verification:

```json
{
  "version": "1.0",
  "tool": "airlift",
  "created_at": "2026-03-22T10:00:00Z",
  "source_project": "my-project",
  "source_gitlab": "192.168.102.104",
  "branch": "main",
  "commit_sha": "abc123...",
  "transfer_type": "shallow|full",
  "has_lfs": true,
  "lfs_objects_count": 5,
  "bundle_size_bytes": 1234567
}
```

### Manifest Fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Bundle format version |
| `tool` | string | Name of the tool that created the bundle (always "airlift") |
| `created_at` | ISO 8601 | Bundle creation date and time |
| `source_project` | string | Project name on the internet network |
| `source_gitlab` | string | Source GitLab address |
| `transfer_type` | string | Transfer type: `shallow` (latest commit) or `full` (complete history) |
| `has_lfs` | boolean | Whether the bundle contains LFS objects |
| `lfs_objects_count` | integer | Number of LFS objects in the package |

### checksum.sha256

The checksum file contains a list of SHA-256 hashes for every file in the package. The file format matches the standard `sha256sum` output:

```
e3b0c44298fc1c149afbf4c8996fb924...  manifest.json
a7ffc6f8bf1ed76651c14756a061d662...  repo.bundle
b5bb9d8014a0f9b1d61e21e796d78dcc...  lfs/abc123def456
```

---

## 5. Export Module — Internet Side

The Export module is responsible for packaging a Git project from GitLab on the internet network into a bundle on USB. The module supports both an interactive TUI interface and a CLI interface for automation.

### Export Workflow

1. User runs `airlift export` or selects Export from the TUI main menu
2. TUI displays a list of available projects from the GitLab API (with search and filtering)
3. User selects a project and branch
4. User chooses transfer type: **shallow** (latest commit only, `--depth 1`) or **full** (complete history)
5. System detects connected USB drives and displays them
6. User selects a target USB drive
7. System executes the packaging steps:
   1. `git clone` (or `git clone --depth 1`) from source GitLab
   2. For GitLab with self-signed TLS: uses `GIT_SSL_NO_VERIFY=true` and custom HTTP transport
   3. `git lfs fetch --all` to download all LFS objects
   4. Create a bare repository: `git init --bare`
   5. Push all branches and tags to the bare repo
   6. `git lfs push --all` to the bare repo
   7. Create Git bundle: `git bundle create repo.bundle --all`
   8. Copy LFS objects from the bare repo's `lfs/objects` directory
   9. Create `manifest.json` with metadata
   10. Calculate SHA-256 checksums
   11. Package everything into `.airlift.zip`
   12. Copy the ZIP to the USB drive
8. Display completion summary with bundle details

### CLI — Command Line Interface

```bash
airlift export --project "my-project" --branch "main" --depth shallow --usb /Volumes/USB_DRIVE
```

### GitLab API — Endpoints Used

Authentication with GitLab is performed using Private Token or Username/Password, via the `gitlab.com/gitlab-org/api/client-go` library.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v4/projects` | GET | List projects |
| `/api/v4/projects/:id` | GET | Project details |
| `/api/v4/projects/:id/repository/branches` | GET | List branches |

### Handling Insecure TLS

Since the GitLab on the internet network uses a self-signed certificate, the Export module configures `TLSClientConfig` with `InsecureSkipVerify: true` in the HTTP transport for both the GitLab client and Git operations. The environment variable `GIT_SSL_NO_VERIFY=true` is set for all external Git commands.

---

## 6. Import Module — Classified Network Side

### First-Time Setup

On first launch on the classified network, the user needs to configure:

1. **GitLab URL** — Target GitLab address on the classified network
2. **Credentials** — Username and password (stored encrypted in config or entered each session)
3. **Project Mapping Dictionary** — Maps project names from the internet network to names on the classified network

Project mapping file structure:

```json
{
  "mappings": {
    "public-project-alpha": {
      "target_project": "classified-name-1",
      "target_group": "group/subgroup",
      "target_branch": "main"
    },
    "public-project-beta": {
      "target_project": "classified-name-2",
      "target_group": "another-group",
      "target_branch": "develop"
    }
  }
}
```

Configuration is saved at `~/.airlift/config.json` (or a specified path).

### Import Workflow

1. User runs `airlift import` or selects Import from the main menu
2. System detects connected USB drives
3. User selects a USB drive
4. System scans for `.airlift.zip` files on the USB
5. For each bundle found:
   1. Extract and verify SHA-256 checksums
   2. Read `manifest.json` to identify the source project
   3. Look up project mapping in configuration
   4. If no mapping exists — display a TUI form to create a new mapping
   5. Display summary: `Project X → Classified Name Y in group Z`
6. User confirms the import
7. System executes:
   1. Extract `repo.bundle`
   2. Create bare repo from bundle: `git clone --bare repo.bundle temp-repo`
   3. If LFS objects exist — copy them into the bare repo's LFS storage
   4. Add target GitLab as remote
   5. `git push --all` to target GitLab
   6. `git push --tags` to target GitLab
   7. `git lfs push --all` to target
8. Display completion summary with push results

### CLI — Command Line Interface

```bash
airlift import --usb /Volumes/CLASSIFIED_USB --auto
```

The `--auto` flag skips confirmations and uses existing mappings from configuration. Useful for automation.

---

## 7. Cross-Platform USB Detection

The USB module provides a unified interface for USB drive detection, with OS-specific implementations using Go build tags.

### macOS — Implementation

- Use `diskutil list` command to scan disks
- Filter by type: External, Removable
- Get mount point from `diskutil info`
- Monitor changes via FSEvents or polling

```go
// detect_darwin.go
func DetectUSBDrives() ([]USBDrive, error) {
    // Run: diskutil list -plist external
    // Parse plist output
    // Filter removable drives
    // Get mount point via diskutil info
    return drives, nil
}
```

### Windows — Implementation

- Use Windows API via `golang.org/x/sys/windows` or `syscall`
- Scan drives with `GetLogicalDrives()`
- Check drive type with `GetDriveType()` — filter `DRIVE_REMOVABLE`
- Get volume name with `GetVolumeInformation()`

```go
// detect_windows.go
func DetectUSBDrives() ([]USBDrive, error) {
    // Call GetLogicalDrives() to get bitmask
    // For each drive letter:
    //   Call GetDriveType() - filter DRIVE_REMOVABLE
    //   Call GetVolumeInformation() for label
    return drives, nil
}
```

### USBDrive — Data Structure

```go
type USBDrive struct {
    Path       string  // Mount point (e.g., /Volumes/USB or E:\)
    Label      string  // Volume label
    SizeBytes  int64   // Total capacity
    FreeBytes  int64   // Available space
    FileSystem string  // e.g., exFAT, NTFS, FAT32
}
```

> **Important:** USB drives should be formatted as **exFAT** for full compatibility between macOS and Windows, and to support files larger than 4GB.

---

## 8. Security & Integrity

Security and data integrity are central considerations in AirLift, which operates in a sensitive security environment.

### Data Integrity

- **SHA-256 Checksums** — Every file in the bundle receives a SHA-256 signature. The `checksum.sha256` file is created during export and verified during import
- **Verification Before Push** — The system verifies complete integrity of all files before performing a push to the target GitLab. If verification fails, the process stops with a detailed error message

### Data Separation

- **Clean Bundle** — The bundle contains only information from the internet side (project name, source GitLab address). There is no classified information in the bundle
- **Manifest Without Classified Names** — The `manifest.json` does not contain project names from the classified network. Mapping is performed only on the classified side
- **Local Mapping Only** — The project mapping dictionary is stored only on the classified network machine and is never copied to USB

### Credential Storage

- **OS Keychain** — Support for storing encrypted credentials in macOS Keychain and Windows Credential Manager
- **Prompt Mode** — User can choose to enter credentials each session without local storage
- **No Credentials in Bundle** — Passwords, tokens, and login details are never included in bundle files

### Access Control

- All GitLab access is authenticated — both on the source (Export) and target (Import) sides
- The tool supports Private Tokens and Username/Password authentication
- The GitLab API client supports rate limiting and retry with exponential backoff

---

## 9. Build & Cross Compilation

One of Go's key advantages is its built-in cross-compilation capability. Binaries for any platform can be built from a single machine.

### Makefile

```makefile
BINARY_NAME=airlift
VERSION=$(shell git describe --tags --always)

.PHONY: build-all build-darwin build-windows clean

build-all: build-darwin build-windows

build-darwin:
	GOOS=darwin GOARCH=amd64 go build \
	  -ldflags "-s -w -X main.version=$(VERSION)" \
	  -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/airlift/
	GOOS=darwin GOARCH=arm64 go build \
	  -ldflags "-s -w -X main.version=$(VERSION)" \
	  -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/airlift/

build-windows:
	GOOS=windows GOARCH=amd64 go build \
	  -ldflags "-s -w -X main.version=$(VERSION)" \
	  -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/airlift/

clean:
	rm -rf dist/
```

### Build Artifacts

| File | Platform | Architecture |
|------|----------|-------------|
| `airlift-darwin-amd64` | macOS | Intel (x86_64) |
| `airlift-darwin-arm64` | macOS | Apple Silicon (M1/M2/M3) |
| `airlift-windows-amd64.exe` | Windows | x86_64 |

### Build Flags

- `-s -w` — Remove symbol table and debug info to reduce binary size
- `-X main.version=$(VERSION)` — Inject version number from Git tags at build time
- Each binary is completely self-contained — zero dependencies. Can be copied and run directly

### Prerequisites on Target Machine

- **git** — Installed and accessible from PATH
- **git-lfs** — Installed and configured (`git lfs install`)

Both tools are typically already installed on developer machines. The AirLift binary itself requires no additional dependencies.

---

## 10. Terminal UI Design

The AirLift user interface is built on the Bubbletea framework from Charm, styled with Lipgloss and using ready-made components from Bubbles. The interface provides an intuitive terminal experience with keyboard navigation, search, and progress bars.

### Main Menu

```
╭─── AirLift v1.0 ────────────────────────────╮
│                                               │
│   ⚡ AirLift - Air-Gap Code Transfer          │
│                                               │
│   [1] 📤 Export (Internet → USB)              │
│   [2] 📥 Import (USB → Classified)            │
│   [3] ⚙️  Configuration                       │
│   [4] ❌ Exit                                  │
│                                               │
│   ← → Navigate  │  Enter Select  │  q Quit   │
╰───────────────────────────────────────────────╯
```

### Export Screens

| Screen | Description | Bubbles Component |
|--------|-------------|-------------------|
| Project Selection | Filtered list of projects from GitLab | `list.Model` |
| Branch Selection | Branch list for selected project | `list.Model` |
| Transfer Type | Shallow or full history | Radio buttons (custom) |
| USB Selection | List of connected USB drives | `list.Model` |
| Progress | Progress bar with current step status | `progress.Model` + `spinner.Model` |
| Summary | Details of the created bundle | Static view |

### Import Screens

| Screen | Description | Bubbles Component |
|--------|-------------|-------------------|
| USB Selection | List of connected USB drives | `list.Model` |
| Bundle Detection | USB scan and list of `.airlift.zip` files | `list.Model` |
| Mapping Verification | Check existing mapping or create new one | `textinput.Model` |
| Confirmation | Import operation summary for user approval | Confirm dialog (custom) |
| Progress | Progress bar with push status | `progress.Model` + `spinner.Model` |
| Summary | Push results and transfer details | Static view |

### Configuration Screen

- **GitLab URL** — Set target GitLab address
- **Credentials** — Set username and password (save to Keychain or enter each time)
- **Project Mappings** — Mapping table with add, edit, and delete options (`table.Model`)

---

## 11. Go Dependencies

Below is the list of main modules used in the project:

### External Dependencies

| Module | Description |
|--------|-------------|
| `charmbracelet/bubbletea` | TUI framework — Terminal UI engine (Elm architecture) |
| `charmbracelet/lipgloss` | TUI styling — component design, colors, borders |
| `charmbracelet/bubbles` | TUI components — text input, list, progress, spinner, table |
| `gitlab.com/gitlab-org/api/client-go` (v2) | GitLab API client — project management, branches, authentication |
| `github.com/go-git/go-git/v5` | Pure Go Git — for operations that don't require external git |

### Standard Library Packages

| Package | Usage |
|---------|-------|
| `os/exec` | Running external git/git-lfs commands |
| `crypto/sha256` | Checksum calculation and verification |
| `archive/zip` | Bundle packaging and extraction (ZIP) |
| `encoding/json` | Reading and writing config and manifest |
| `crypto/tls` | TLS management — insecure skip verify support |

> **Note:** The tool requires `git` and `git-lfs` to be installed on both machines (source and target). These tools are typically already available on developer machines. The AirLift binary itself is zero dependencies — a single file that can be copied and run.

---

## 12. Usage Scenarios

### Scenario 1: First-Time Setup on Classified Network

The first time the tool is used on the classified network, an initial setup is required:

1. Copy the `airlift` binary to a machine on the classified network
2. Run `airlift` and select **Configuration** from the main menu
3. Set the target GitLab URL
4. Set credentials (username and password)
5. Create project mappings

### Scenario 2: Daily Code Transfer

This is the most common use case — routine transfer of code updates:

1. On the internet machine: Run `airlift export` → select project → select USB → done
2. Physical transfer of USB through the intermediary computer (manual security step)
3. On the classified machine: Run `airlift import` → select USB → auto-detect → confirm → done

> Estimated transfer time for a typical project (without LFS): less than one minute per direction.

### Scenario 3: Multiple Project Transfer

- Export mode supports selecting multiple projects at once
- Each project gets a separate `.airlift.zip` file on the USB
- Import mode detects all bundles and processes them sequentially
- A summary report shows results for each project separately

### Scenario 4: Updating an Existing Mapping

When a new project is added on the internet network:

1. On the internet side: Regular export of the new project
2. On the classified side: During import, the system detects that no mapping exists
3. A TUI form is displayed to create a new mapping — target project name, group, and branch
4. The mapping is automatically saved to configuration for future use

---

## 13. Future Considerations

Below is a list of possible capabilities and improvements for future versions of the system:

### Incremental Transfers

Currently, each transfer includes a complete bundle (or shallow). In the future, incremental transfers can be implemented using `git bundle create --since=<commit>`. The system would track the last transferred commit per project and create bundles containing only changes since then.

### Bidirectional Transfer

If the need arises to transfer code from the classified network back to the internet network, a reverse Export/Import mode can be added. This scenario requires additional security considerations and appropriate approvals.

### Bundle Encryption

Adding an encryption layer (AES-256-GCM) to the bundle itself, so that even if the USB is lost, the data is protected. The user would configure a shared encryption key on both sides.

### Audit Log

Maintaining a detailed log of all transfers — when, what, from where, and to where. The log is stored locally on each side and enables tracking and reporting.

### CI/CD Integration

Connecting AirLift to CI/CD pipelines on both sides. For example: a pipeline on the internet network automatically generates a bundle after a merge, and a pipeline on the classified network performs an automatic import when a USB is detected.

### Merge Request Metadata Transfer

Support for transferring Merge Request metadata (title, description, discussion) along with the code, to preserve work context.

---

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**End of Document — AirLift Architecture Document v1.0**
