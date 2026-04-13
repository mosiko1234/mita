# MITA — User Guide

**Moses In The Ark** — a single-binary tool for transferring Git repositories (including LFS) between air-gapped networks via USB.

---

## Table of Contents

1. [What MITA Does](#what-mita-does)
2. [Prerequisites](#prerequisites)
3. [Installation](#installation)
   - [macOS / Linux](#macos--linux)
   - [Windows](#windows)
4. [First-Time Setup](#first-time-setup)
   - [Source (Internet) Network](#source-internet-network)
   - [Target (Classified) Network](#target-classified-network)
5. [Daily Workflow](#daily-workflow)
   - [Exporting a Repo (Internet Network)](#exporting-a-repo-internet-network)
   - [Importing a Repo (Classified Network)](#importing-a-repo-classified-network)
6. [Managing Project Mappings](#managing-project-mappings)
7. [CLI Reference](#cli-reference)
8. [Troubleshooting](#troubleshooting)

---

## What MITA Does

MITA bridges two GitLab servers on networks that have **no direct connection**:

```
┌─────────────────────┐     ┌──────┐     ┌──────────────────────┐
│  Internet Network   │     │ USB  │     │ Classified Network   │
│                     │     │ 💾   │     │                      │
│  GitLab (source) ───┼────►│.zip  │────►├── GitLab (target)    │
│                     │     │      │     │                      │
│  mita export        │     │      │     │  mita import         │
└─────────────────────┘     └──────┘     └──────────────────────┘
```

Each `.mita.zip` bundle contains:

- `repo.bundle` — a Git bundle with all branches and tags
- `lfs/` — Git LFS objects (if any)
- `manifest.json` — metadata (project, branch, commit, timestamp)
- `checksum.sha256` — integrity verification

---

## Prerequisites

Both machines need:

- **Git** (`git --version`)
- **Git LFS** (`git lfs version`) — only if your repos use LFS

Source machine additional:

- Access to the source GitLab
- A **private token** OR **username/password** for the source GitLab

Target (classified) machine additional:

- Access to the target GitLab
- A **username/password** with `Developer` (or higher) on the target group
- The target project **must exist** (create it empty via the GitLab web UI first)

---

## Installation

### macOS / Linux

1. **Download** the correct binary from the [GitHub Releases](https://github.com/mosiko1234/mita/releases):
   - macOS Intel: `mita-darwin-amd64`
   - macOS Apple Silicon: `mita-darwin-arm64`
   - Linux x86_64: `mita-linux-amd64`

2. **Run it once** — MITA will auto-install itself to `/usr/local/bin/mita`:
   ```bash
   chmod +x ~/Downloads/mita-darwin-arm64
   ~/Downloads/mita-darwin-arm64
   ```

   You'll see:
   ```
   Installed MITA to /usr/local/bin/mita — you can now run 'mita' from anywhere.
   ```

3. **Verify**:
   ```bash
   mita version
   # MITA v0.x.x (Moses In The Ark)
   ```

#### macOS — Grant Full Disk Access (important!)

MITA needs to delete macOS metadata files (`.Spotlight-V100`, `.Trashes`, `.fseventsd`) from your USB so the classified-side Linux scanner accepts it.

1. Open **System Settings → Privacy & Security → Full Disk Access**
2. Click **+** and add your Terminal app (`Terminal.app`, `iTerm`, `Warp`, etc.)
3. Restart the Terminal

Without this, you'll see:
```
Error: cannot delete: .Spotlight-V100, .Trashes, .fseventsd
FIX: Open System Settings > Privacy & Security > Full Disk Access
```

### Windows

1. Download `mita-windows-amd64.exe` from GitHub Releases.
2. Run it once — it installs itself to `%LOCALAPPDATA%\mita\mita.exe`.
3. Add that path to your `PATH`, or run with the full path.

---

## First-Time Setup

Both networks need the same initial setup: launch `mita` (no arguments) to open the TUI, go to **Configuration**, and fill in the relevant sections.

### Source (Internet) Network

```
┌────────────────────────────────────────────────┐
│ MITA v0.x.x                                    │
│ Air-Gap Code Transfer                          │
│                                                │
│ > [1] >> Export (Internet -> USB)              │
│   [2] << Import (USB -> Classified)            │
│   [3] ** Configuration                         │
│   [4] XX Exit                                  │
│                                                │
│ Up/Down: Navigate  |  Enter: Select  |  q: Quit│
└────────────────────────────────────────────────┘
```

1. Select **Configuration → GitLab URLs** and set:
   - **Source GitLab URL**: `https://gitlab-internet.example.com/`
2. Select **Configuration → Source Auth Mode** — pick one:
   - **Token** (recommended) — a Personal Access Token with `read_api`, `read_repository`
   - **Username + Password**
3. Select **Configuration → Source Credentials** and fill in your token or user/pass.
4. If the source GitLab uses a self-signed certificate, go to **Configuration → Security (TLS)** and toggle on **Insecure TLS**.

### Target (Classified) Network

1. **Configuration → GitLab URLs** → set **Target GitLab URL** (e.g. `https://gitlab.classified.local/`).
2. **Configuration → Target Credentials** → username + password for the target GitLab.
3. **Configuration → Security (TLS)** → enable if the classified GitLab uses a self-signed cert.

The config is stored at `~/.mita/config.json` (Windows: `%USERPROFILE%\.mita\config.json`).

---

## Daily Workflow

### Exporting a Repo (Internet Network)

#### Using the TUI

```bash
mita
```

1. Pick **Export**.
2. **Project selection** — browse all projects you can see on the source GitLab; filter-as-you-type supported.
   ```
   ┌─ Select Project ────────────────────────────┐
   │ > observability           devops/obs        │
   │   crosslink               infra/net         │
   │   ipsw-downloader         tools/ipsw        │
   │                                             │
   │ / to filter  Enter to select                │
   └─────────────────────────────────────────────┘
   ```
3. **Branch** — pick the branch to export (defaults to the project's default branch).
4. **Transfer type**:
   - **Shallow** — latest commit only. Small, fast. Use for routine updates.
   - **Full** — complete history, all branches, all tags.
5. **USB** — pick the drive (MITA auto-detects).
6. Watch the progress. When done:
   ```
   ┌─ Export - Complete ─────────────────────────┐
   │ ✓ Bundle created successfully!              │
   │                                             │
   │   Project:   observability                  │
   │   Branch:    main                           │
   │   Type:      Full                           │
   │   USB Drive: KINGSTON                       │
   │   Bundle:    /Volumes/KINGSTON/observability│
   │              -20260329-045424.mita.zip      │
   │                                             │
   │ > Export another project to same USB        │
   │   Clean & Unmount USB                       │
   │   Back to main menu                         │
   └─────────────────────────────────────────────┘
   ```
7. Choose **Clean & Unmount USB** — this removes all macOS metadata files and unmounts the drive. **Critical for the classified-side scanner to accept it.**
8. Physically remove the USB.

#### Using the CLI

```bash
mita export \
  --project "observability" \
  --branch main \
  --depth full \
  --usb /Volumes/KINGSTON

# Then clean & unmount:
mita clean /Volumes/KINGSTON
```

### Importing a Repo (Classified Network)

Before you start: **the target project must already exist** (empty, no README) on the classified GitLab.

#### Using the TUI

```bash
mita
```

1. Pick **Import**.
2. Select the USB drive (MITA auto-detects).
3. MITA scans for `*.mita.zip` files and shows them with mapping status:
   ```
   ┌─ Detected Bundles (press M to set mapping, Enter to import) ─┐
   │ > observability-20260329-045424.mita.zip                      │
   │   ⚠ No mapping — press M to set target                        │
   │                                                               │
   │   crosslink-20260329-041115.mita.zip                          │
   │   → infra/net/crosslink                                       │
   └───────────────────────────────────────────────────────────────┘
   ```
4. For any bundle with **⚠ No mapping**, select it and press **M** to open the mapping form:
   ```
   ┌─ Import - New Project Mapping ──────────────┐
   │ No mapping found for: observability...      │
   │                                             │
   │ Target Project: observability               │
   │ Target Group:   devops/infra                │
   │ Target Branch:  main                        │
   │                                             │
   │ Tab: Next field  |  Enter: Save  |  Esc: Cancel │
   └─────────────────────────────────────────────┘
   ```
5. Press **Enter** on the last field to save. The mapping is stored in `~/.mita/config.json` for future imports.
6. Once **all bundles have mappings**, press **Enter** to start the import.
7. MITA:
   - Verifies SHA-256 checksums
   - Extracts the bundle
   - Restores LFS objects
   - Pushes all branches and tags to the target GitLab

#### Using the CLI — Interactive

```bash
mita import --usb /media/usb
```
MITA finds the bundles and prompts you for `group` / `project` / `branch` for each one that has no mapping.

#### Using the CLI — Fully Automated

```bash
mita import --usb /media/usb \
  --group devops/infra \
  --project observability \
  --branch main \
  --gitlab-url https://gitlab.classified.local/ \
  --user myuser \
  --pass "my-password"
```

With `--auto` it uses existing saved mappings and skips unmapped bundles:
```bash
mita import --usb /media/usb --auto
```

---

## Managing Project Mappings

A mapping tells MITA where to push a bundle on the target GitLab:

```
Source project: observability
     ↓
Target: devops/infra/observability (branch: main)
```

### View / Edit / Delete Mappings

```bash
mita
```
Pick **Configuration → Project Mappings**:

```
┌─ Configuration - Project Mappings ─────────────────────────────────────┐
│ Source                 Target Project     Group          Branch        │
│ ──────────────────────────────────────────────────────────             │
│ > observability        observability      devops/infra   main          │
│   crosslink            crosslink          infra/net      main          │
│                                                                         │
│ A: Add  |  Enter/E: Edit  |  D: Delete  |  Esc: Back                   │
└─────────────────────────────────────────────────────────────────────────┘
```

- **A** — Add a new mapping manually (before importing)
- **Enter / E** — Edit the selected mapping
- **D** — Delete the selected mapping

### Pre-configure Mappings Before Import

Useful if you know in advance where each project should go:

1. Open **Configuration → Project Mappings**
2. Press **A**
3. Fill in:
   - **Source project**: exact project name as it was exported (e.g. `observability`)
   - **Target group**: full group path on the target GitLab (e.g. `devops/infra`)
   - **Target project**: name on the target (can be the same or different)
   - **Target branch**: usually `main`
4. **Enter** to save

When you later import a bundle, MITA sees the existing mapping and uses it automatically.

---

## CLI Reference

### Global

```bash
mita                # open the TUI
mita version        # show version
mita install        # force re-install to /usr/local/bin
```

### Export

```bash
mita export [flags]

Flags:
  --project    Project name on the source GitLab (required)
  --branch     Branch to export (default: main)
  --depth      'shallow' (latest only) or 'full' (default: shallow)
  --usb        USB drive mount path (required)
  --gitlab-url Override source GitLab URL
  --token      Private token (overrides config)
  --user       Username for basic auth
  --pass       Password for basic auth
```

### Import

```bash
mita import [flags]

Flags:
  --usb         USB drive mount path (required)
  --group       Target GitLab group path
  --project     Target project name
  --branch      Target branch (default: main)
  --gitlab-url  Override target GitLab URL
  --user        Target GitLab username
  --pass        Target GitLab password
  --auto        Use saved mappings only, skip unmapped bundles
```

### Clean (macOS only)

```bash
mita clean <usb-path>
```
Removes all hidden macOS metadata files (`.Spotlight-V100`, `.Trashes`, `.fseventsd`, `.DS_Store`, `._*`) and unmounts the drive. Required before transferring to the classified-side Linux scanner.

---

## Troubleshooting

### "Port number was not a decimal number"
Your password contains `@`, `:`, `/`, or another URL-special character.
**Fix**: Update to the latest MITA — it now URL-encodes credentials.

### "unable to mount device" on the classified Linux
The USB still has macOS metadata files. Run `mita clean /Volumes/XXX` on the source machine **before** unplugging.

### "Operation not permitted" on `.Spotlight-V100`
Your Terminal doesn't have Full Disk Access.
**Fix**: System Settings → Privacy & Security → Full Disk Access → add Terminal / iTerm / Warp.

### "git push --all: exit status 128"
The remote rejected the push. Check:
- The target project **exists** on the classified GitLab (create it empty first)
- Your target credentials are correct
- Your user has `Developer` or higher on the target group
- The error text after the status line contains the git message (look for `fatal: ...`)

### "USB not detected"
- macOS: check `diskutil list external`
- Make sure the drive is mounted (not just inserted)
- The drive should be exFAT or FAT32 (NTFS mounts read-only on macOS)

### Source GitLab uses self-signed certificate
Configuration → Security (TLS) → enable **Insecure TLS**.

### Reset everything
```bash
rm -rf ~/.mita           # clear config and mappings
rm /usr/local/bin/mita   # remove the binary
# Then download and re-install from GitHub Releases
```

---

## Architecture Quick Reference

```
cmd/mita/main.go                 # CLI entry point + auto-installer
internal/
  config/                        # ~/.mita/config.json handling
  gitlab/                        # GitLab API client, clone, push
  bundle/                        # export/import bundle format
  usb/                           # cross-platform USB detection
  tui/                           # Bubbletea terminal UI
  crypto/                        # SHA-256 integrity checks
```

Bundle format (`.mita.zip`):
```
observability-20260329-045424.mita.zip
├── manifest.json       # project, branch, commit, timestamp
├── repo.bundle         # git bundle (all refs)
├── lfs/                # LFS objects (if any)
│   └── <hash-prefix>/<hash-rest>
└── checksum.sha256     # integrity of all the above
```

---

## Support

- Issues: https://github.com/mosiko1234/mita/issues
- Source: https://github.com/mosiko1234/mita
