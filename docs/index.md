---
layout: default
title: "7zkpxc — Secure 7-Zip + KeePassXC"
description: "CLI tool that generates unique passwords for every 7-Zip archive and stores them in KeePassXC. Zero shell leakage, zero clipboard exposure."
image: https://opengraph.githubassets.com/1/lxstig/7zkpxc
---

# 7zkpxc

**Secure 7-Zip wrapper with KeePassXC integration.**

7zkpxc is a security-focused CLI tool that bridges [7-Zip](https://7-zip.org/) and [KeePassXC](https://keepassxc.org/). It generates a unique, cryptographically strong password for every archive you create, securely stores it in your KeePassXC database, and automatically retrieves it when you need to extract or list the archive — all without the password ever touching your shell history, process list, or clipboard.

**Remember one master password. Protect unlimited archives.**

[![GitHub](https://img.shields.io/github/stars/lxstig/7zkpxc?style=social)](https://github.com/lxstig/7zkpxc)
[![Go Report Card](https://goreportcard.com/badge/github.com/lxstig/7zkpxc)](https://goreportcard.com/report/github.com/lxstig/7zkpxc)
[![AUR](https://img.shields.io/aur/version/7zkpxc)](https://aur.archlinux.org/packages/7zkpxc)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

---

## The Problem

```bash
# The typical workflow — every step leaks
7z a -p"Summer2024!" backup.7z ~/documents/

# Password is now in:
#   • Shell history (~/.zsh_history, ~/.bash_history)
#   • Process list (visible to every user via `ps aux`)
#   • Possibly your clipboard
#   • Your memory (so you reuse it everywhere)
```

## The Solution

```bash
# The 7zkpxc workflow — nothing leaks
7zkpxc a backup.7z ~/documents/

# ✓ Random 64-char password generated
# ✓ Saved in KeePassXC
# ✓ Piped to 7z via PTY (invisible to ps, history, everything)
# ✓ Zeroed from memory

# Later:
7zkpxc x backup.7z   # password fetched automatically
```

---

## Features

- **Unique passwords per archive** 64-character cryptographically random, generated fresh every time.
- **KeePassXC as your vault** Passwords are stored and retrieved from your `.kdbx` database automatically.
- **Zero shell leakage** Passwords are piped to 7-Zip via PTY. Nothing in `ps aux`, nothing in history.
- **Memory safety** Secrets are zeroed in memory immediately after use.
- **Metadata-driven relinking** File-size fingerprints stored in KeePass Notes enable fast recovery of renamed/moved archives.
- **Split volume support** Automatically resolves passwords for split archives (`.7z.001`, `.part001.rar`, etc.).
- **Rename/move support** `7zkpxc mv` moves the file on disk and updates the KeePassXC entry.
- **Relink command** `7zkpxc relink` finds the correct entry for an archive by brute-forcing passwords with size pre-filtering.
- **Cloud-ready** Encrypt locally, upload anywhere. Only you (with your KeePassXC database) can decrypt.
- **Dependency checking** Tells you exactly what's missing before doing anything.
- **Tab-completing init** Interactive setup with real filesystem tab completion.
- **Shell completions** Native Zsh, Bash, and Fish autocomplete.

## Commands

| Command | Description |
|---------|-------------|
| `7zkpxc init` | Interactive setup wizard |
| `7zkpxc a <archive> [files...]` | Create encrypted archive |
| `7zkpxc l <archive>` | List archive contents |
| `7zkpxc x <archive>` | Extract with full paths |
| `7zkpxc e <archive> [files...]` | Extract flat |
| `7zkpxc u <archive> [files...]` | Update files in archive |
| `7zkpxc d <archive> [files...]` | Delete files from archive |
| `7zkpxc rn <archive> <old> <new>` | Rename files inside archive |
| `7zkpxc t <archive>` | Test archive integrity |
| `7zkpxc mv <old> <new>` | Move/rename archive + update KeePassXC |
| `7zkpxc remove <archive>` | Delete entry and archive file |
| `7zkpxc relink <archive\|dir>` | Relink archives to their KeePassXC entries |

## Install

### Pre-built Binaries

Available on the [Releases](https://github.com/lxstig/7zkpxc/releases) page.

### From Source

```bash
git clone https://github.com/lxstig/7zkpxc.git
cd 7zkpxc
make build
sudo make install
```

### Arch Linux (AUR)

```bash
yay -S 7zkpxc
```

## Security Model

| Threat | Mitigation |
|--------|------------|
| Shell history exposure | Password never typed or passed as argument |
| Process list snooping | Password piped via PTY, not CLI args |
| Weak / reused passwords | Every archive gets a unique 64-char random password |
| Memory forensics | Secrets zeroed after use |
| Credential sprawl | Single KeePassXC database holds all archive passwords |

---

## Requirements

| Dependency | Install |
|-----------|---------|
| [7-Zip](https://7-zip.org/) | Arch: `pacman -S 7zip` · Debian: `apt install p7zip-full` |
| [KeePassXC](https://keepassxc.org/) | Arch: `pacman -S keepassxc` · Debian: `apt install keepassxc` |

---

<p align="center">
  <a href="https://github.com/lxstig/7zkpxc">View on GitHub</a> ·
  <a href="https://github.com/lxstig/7zkpxc/releases">Releases</a> ·
  <a href="https://aur.archlinux.org/packages/7zkpxc">AUR Package</a>
</p>

<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "7zkpxc",
  "description": "Secure 7-Zip wrapper with KeePassXC integration. Generates unique passwords per archive, stores them in KeePassXC, and pipes them via PTY with zero shell leakage.",
  "applicationCategory": "SecurityApplication",
  "operatingSystem": "Linux, macOS",
  "url": "https://github.com/lxstig/7zkpxc",
  "downloadUrl": "https://github.com/lxstig/7zkpxc/releases",
  "license": "https://www.gnu.org/licenses/gpl-3.0",
  "author": {
    "@type": "Person",
    "name": "lxstig",
    "url": "https://github.com/lxstig"
  },
  "offers": {
    "@type": "Offer",
    "price": "0",
    "priceCurrency": "USD"
  },
  "keywords": ["7zip", "keepassxc", "encryption", "password manager", "archive", "security", "cli", "golang"]
}
</script>
