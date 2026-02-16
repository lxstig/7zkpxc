# 7zkpxc

[![CI](https://github.com/lxstig/7zkpxc/actions/workflows/ci.yml/badge.svg)](https://github.com/lxstig/7zkpxc/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/lxstig/7zkpxc)](https://goreportcard.com/report/github.com/lxstig/7zkpxc)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

**7zkpxc** is a security-focused CLI tool that bridges [7-Zip](https://7-zip.org/) and [KeePassXC](https://keepassxc.org/). It generates a unique, cryptographically strong password for every archive you create, securely stores it in your KeePassXC database and automatically retrieves it when you need to extract/list the archive, all without the password ever touching your shell history, process list, or clipboard.

**Remember one master password. Protect unlimited archives.**

## Why?

Encrypting files with 7-Zip is common practice before uploading to cloud storages like Dropbox, Google Drive, Nextcloud, S3, you name it. But how most people do it is fundamentally broken:

```bash
# The typical workflow > every step leaks
7z a -p"Summer2024!" backup.7z ~/documents/

# Password is now in:
#   • Shell history (~/.zsh_history, ~/.bash_history)
#   • Process list (visible to every user via `ps aux`)
#   • Possibly your clipboard
#   • Your memory (so you reuse it for the next archive, and the next...)
```

**7zkpxc** eliminates all of that:

```bash
# The 7zkpxc workflow > nothing leaks
7zkpxc a backup.7z ~/documents/

# What happens:
#   1. A random 64-character password is generated
#   2. It's saved in your KeePassXC database
#   3. It's piped to 7z via PTY (invisible to `ps`, `history`, everything)
#   4. It's zeroed from memory
#
# Extracting later:
7zkpxc x backup.7z
#   → Looks up the password in KeePassXC, pipes it to 7z. Done.
```

Every archive gets its own unique password. You never see it, never type it, never reuse it. Your KeePassXC database is the single source of truth, protected by your master password, your key file, or your hardware key.

## Features

- **Unique passwords per archive** 64-character cryptographically random, generated fresh every time.
- **KeePassXC as your vault** Passwords are stored and retrieved from your `.kdbx` database automatically.
- **Zero shell leakage** Passwords are piped to 7-Zip via PTY. Nothing in `ps aux`, nothing in history.
- **Memory safety** Secrets are zeroed in memory immediately after use.
- **Split volume support** Automatically resolves passwords for split archives (`.7z.001`, `.part001.rar`, etc.).
- **Cloud-ready** Encrypt locally, upload anywhere. Only you (with your KeePassXC database) can decrypt.
- **Dependency checking** Tells you exactly what's missing before doing anything.
- **Tab-completing init** Interactive setup with real filesystem tab completion.
- **Shell completions** Native Zsh, Bash, and Fish autocomplete.

## Security Model

| Threat | Mitigation |
|--------|------------|
| Shell history exposure | Password is never typed or passed as argument. |
| Process list snooping (`ps`) | Password is piped via PTY, not CLI args. |
| Weak / reused passwords | Every archive gets a unique 64-char random password. |
| Memory forensics | Secrets are zeroed in memory after use. |
| Credential sprawl | Single KeePassXC database holds all archive passwords. |

### What 7zkpxc Does NOT Protect Against

- A compromised machine with root access (memory inspection, keyloggers).
- A compromised or weakly protected KeePassXC database —> **use a strong master password**.
- Physical access attacks (cold boot, evil maid).

## Requirements

7zkpxc requires two external tools. It checks for them automatically and tells you exactly what's missing:

| Dependency | What it does | Install |
|-----------|-------------|---------|
| [**7-Zip**](https://7-zip.org/) (`7z`) | Creates and extracts encrypted archives | Arch: `sudo pacman -S 7zip`<br>Debian/Ubuntu: `sudo apt install p7zip-full` |
| [**KeePassXC**](https://keepassxc.org/) (`keepassxc-cli`) | Stores and retrieves archive passwords | Arch: `sudo pacman -S keepassxc`<br>Debian/Ubuntu: `sudo apt install keepassxc` |

## Installation

### Arch Linux (AUR)

```bash
# Using an AUR helper (recommended)
yay -S 7zkpxc
# or
paru -S 7zkpxc

# Manual installation
git clone https://aur.archlinux.org/7zkpxc.git
cd 7zkpxc
makepkg -si
```

### From Source

```bash
git clone https://github.com/lxstig/7zkpxc.git
cd 7zkpxc
make build
sudo make install
```

> **Note:** Always `make build` as your normal user first. `sudo make install` only copies the binary and completions, it does not rebuild.

### From Releases

Pre-built binaries for Linux and macOS are available on the [Releases](https://github.com/lxstig/7zkpxc/releases) page.

### Uninstall

```bash
sudo make uninstall          # Removes binary + completions (keeps config)
sudo make purge              # Removes everything including ~/.config/7zkpxc
```

## Quick Start

```bash
# 1. Set up — point to your KeePassXC database (Tab completion works here)
7zkpxc init

# 2. Create an encrypted archive
7zkpxc a secrets.7z ~/documents/

# 3. List contents
7zkpxc l secrets.7z

# 4. Extract
7zkpxc x secrets.7z

# 5. Remove the KeePassXC entry when you no longer need the archive
7zkpxc d secrets.7z
```

## Command Reference

| Command | Description |
|---------|-------------|
| `7zkpxc init` | Interactive setup wizard (Tab completion for paths) |
| `7zkpxc a <archive> [files...]` | Create encrypted archive with auto-generated password |
| `7zkpxc x <archive>` | Extract (password fetched from KeePassXC automatically) |
| `7zkpxc l <archive>` | List archive contents |
| `7zkpxc d <archive>` | Delete the KeePassXC entry for an archive |
| `7zkpxc version` | Print version, commit, and build date |

### Flags

```bash
# Compression
7zkpxc a --fast archive.7z files/        # Fastest (-mx=1)
7zkpxc a --best archive.7z files/        # Best (-mx=9)

# Split volumes
7zkpxc a --volume 100m archive.7z files/

# Extract to specific directory
7zkpxc x -o /tmp/output archive.7z

# Pass raw 7z flags
7zkpxc a archive.7z files -- -sfx -m0=lzma2

# Delete without confirmation
7zkpxc d -f archive.7z

# Split volumes resolve automatically
7zkpxc x archive.7z.001
```

## Configuration

`7zkpxc init` creates `~/.config/7zkpxc/config.yaml`:

```yaml
general:
  kdbx_path: "/home/user/passwords.kdbx"
  default_group: "Archives/AutoGenerated"
  use_keyring: true
  # generated password length (min: 32, max: 128)
  password_length: 64
sevenzip:
  binary_path: "7z"
  default_args: ["-mhe=on", "-mx=9"]
```

Override any value via environment variables with the `SZKPXC_` prefix:

```bash
export SZKPXC_GENERAL_KDBX_PATH="/other/db.kdbx"
```

## How It Works

```
  ┌──────────┐       ┌───────────┐       ┌─────────┐
  │  7zkpxc  │─────▶│ KeePassXC │       │  7-Zip  │
  │          │◀─────│           │       │         │
  └────┬─────┘       └───────────┘       └────▲────┘
       │                                      │
       │           password via PTY           │
       └──────────────────────────────────────┘
```

**Creating an archive (`a`):**
1. Generates a 64-character random password.
2. Saves it as a new entry in your KeePassXC database.
3. Spawns `7z` in a PTY and pipes the password when prompted.
4. Zeroes the password from memory.

**Extracting or listing (`x`, `l`):**
1. Looks up the password in KeePassXC using a smart fallback chain:
   - Normalized name (`archive.7z.001` → `archive.7z`)
   - Original filename
   - Base name without extension (last resort for split archives)
2. Pipes the password to `7z` via PTY.


## Credits

7zkpxc wouldn't exist without these excellent projects:

- [**KeePassXC**](https://github.com/keepassxreboot/keepassxc) Cross-platform password manager that keeps your secrets safe. The foundation of 7zkpxc's trust model.
- [**7-Zip**](https://7-zip.org/) The legendary file archiver with strong AES-256 encryption. Igor Pavlov's work speaks for itself.
- [**Cobra**](https://github.com/spf13/cobra) & [**Viper**](https://github.com/spf13/viper) The Go CLI and configuration libraries powering the interface.

## License

[GNU General Public License v3.0](LICENSE)
