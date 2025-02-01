# mkbrr

```
         __   ___.                 
  _____ |  | _\_ |________________ 
 /     \|  |/ /| __ \_  __ \_  __ \
|  Y Y  \    < | \_\ \  | \/|  | \/
|__|_|  /__|_ \|___  /__|   |__|   
      \/     \/    \/              
```

mkbrr is a command-line tool to create and inspect torrent files. Fast, single binary, no dependencies. Written in Go.

## Table of Contents

- [Installation](#installation)
  - [Prebuilt Binaries](#prebuilt-binaries)
  - [Go Install](#go-install)
  - [Build from Source](#build-from-source)
- [Usage](#usage)
  - [Create a Torrent](#create-a-torrent)
    - [Single Mode](#single-mode)
    - [Batch Mode](#batch-mode)
    - [Preset Mode](#preset-mode)
    - [Create Flags](#create-flags)
    - [Batch Configuration Format](#batch-configuration-format)
    - [Preset Configuration Format](#preset-configuration-format)
  - [Inspect a Torrent](#inspect-a-torrent)
  - [Modify a Torrent](#modify-a-torrent)
  - [Version Information](#version-information)
  - [Update](#update)
- [Performance](#performance)
- [License](#license)

## Installation

### Prebuilt Binaries

Download the latest release from the [releases page](https://github.com/autobrr/mkbrr/releases).

### Go Install

If you have Go installed:

```bash
go install github.com/autobrr/mkbrr@latest
```

### Build from Source

Requirements:

- Go 1.23.4 or later

```bash
# Clone the repository
git clone https://github.com/autobrr/mkbrr.git
cd mkbrr

# Build the binary to ./build/mkbrr
make build

# Install the binary to $GOPATH/bin
make install

# Or install system-wide (requires sudo)
sudo make install    # installs to /usr/local/bin
```

The build process will automatically include version information and build time in the binary. The version is determined from git tags, defaulting to "dev" if no tags are found.

## Usage

### Create a Torrent

```bash
mkbrr create [path] [flags]
```

#### Single Mode

Create a torrent from a single file or directory:

```bash
mkbrr create path/to/file -t https://please.passthe.tea
```

#### Batch Mode

Create multiple torrents using a YAML configuration file:

```bash
mkbrr create -b batch.yaml
```

Example batch.yaml:

```yaml
version: 1
jobs:
  - output: ubuntu.torrent
    path: /path/to/ubuntu.iso
    trackers:
      - https://tracker.openbittorrent.com/announce
    webseeds:
      - https://releases.ubuntu.com/22.04/ubuntu-22.04.3-desktop-amd64.iso
    comment: "Ubuntu 22.04.3 LTS Desktop AMD64"
    private: false
    # piece_length is automatically optimized based on file size:
    # piece_length: 22  # manual override if needed (2^n: 14-24)
    # max_piece_length: 23  # limits the automatically calculated maximum piece length

  - output: release.torrent
    path: /path/to/release
    trackers:
      - https://tracker.openbittorrent.com/announce
    private: true
    source: "GROUP"
    comment: "My awesome release"
    no_date: false
```

Batch mode will process all jobs in parallel (up to 4 concurrent jobs) and provide a summary of results.

#### Preset Mode

Create torrents using predefined settings from a preset configuration:

```bash
# Use a preset from a config file
mkbrr create -P private path/to/file

# Use a preset from a custom config file
mkbrr create -P emp --preset-file custom-presets.yaml path/to/file

# Override preset settings with command line flags
mkbrr create -P private --source "CUSTOM" path/to/file
```

> [!TIP]
> The preset file is searched for in the following locations (in order):
> 1. File specified by `--preset-file` flag
> 2. `presets.yaml` in the current directory
> 3. `~/.config/mkbrr/presets.yaml` in the user's home directory
> 4. `~/.mkbrr/presets.yaml` in the user's home directory

Example presets.yaml:

```yaml
version: 1

# Defaults that always apply
default:
  private: true
  no_date: true

presets:
  # opentrackr preset
  ptp:
    source: "PTP"
    trackers:
      - "https://please.passthe.tea/announce"
    # piece_length is automatically optimized based on file size
    # piece_length: 20  # manual override if needed (2^n: 14-24)
    # max_piece_length: 23  # limits the automatically calculated maximum piece length

  # Public tracker preset
  public:
    private: false  # overrides default preset
    trackers:
      - "udp://tracker.opentrackr.org:1337/announce"
      - "udp://open.tracker.cl:1337/announce"
      - "udp://9.rarbg.com:2810/announce"
    # piece_length is automatically optimized based on file size
    # piece_length: 22  # manual override if needed (2^n: 14-24)
    # max_piece_length: 23  # limits the automatically calculated maximum piece length
```

#### Create Flags

General flags:

- `-b, --batch <file>`: Use batch configuration file (YAML)
- `-P, --preset <name>`: Use preset from config
- `--preset-file <file>`: Preset config file (default: ~/.config/mkbrr/presets.yaml)
- `-v, --verbose`: Be verbose

Single mode flags:

- `-t, --tracker <url>`: Tracker URL
- `-w, --web-seed <url>`: Add web seed URLs (can be specified multiple times)
- `-p, --private`: Make torrent private (default: true)
- `-c, --comment <text>`: Add comment
- `-l, --piece-length <n>`: Set piece length to 2^n bytes (14-24, automatic if not specified)
- `-m, --max-piece-length <n>`: Limit maximum piece length to 2^n bytes (14-24, unlimited if not specified)
- `-o, --output <path>`: Set output path (default: <name>.torrent)
- `-s, --source <text>`: Add source string
- `-d, --no-date`: Don't write creation date

Note: When using batch mode (-b), torrent settings are specified in the YAML configuration file instead of command line flags.
When using preset mode (-P), command line flags will override the preset settings.

#### Batch Configuration Format

The batch configuration file uses YAML format with the following structure:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/autobrr/mkbrr/main/schema/batch.json
version: 1  # Required, must be 1
jobs:       # List of torrent creation jobs
  - output: string         # Required: Output path for .torrent file
    path: string           # Required: Path to source file/directory
    trackers:              # Optional: List of tracker URLs
      - string
    webseeds:              # Optional: List of webseed URLs
      - string
    private: bool          # Optional: Make torrent private (default: true)
    piece_length: int      # Optional: Piece length exponent (14-24)
    max_piece_length: int  # Optional: Limits the automatically calculated maximum piece length
    comment: string        # Optional: Torrent comment
    source: string         # Optional: Source tag
    no_date: bool          # Optional: Don't write creation date (default: false)
```

#### Preset Configuration Format

The preset configuration file uses YAML format with the following structure:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/autobrr/mkbrr/main/schema/presets.json
version: 1    # Required, must be 1

# Optional: Default settings that apply to all presets unless overridden
default:
  private: true
  no_date: true
  trackers:
    - string
  # ... other settings as needed

presets:      # Map of preset names to their configurations
  preset-name:
    trackers:              # Optional: List of tracker URLs (overrides default)
      - string
    webseeds:              # Optional: List of webseed URLs (overrides default)
      - string
    private: bool          # Optional: Make torrent private (overrides default)
    piece_length: int      # Optional: Piece length exponent (14-24)
    max_piece_length: int  # Optional: Limits the automatically calculated maximum piece length
    comment: string        # Optional: Torrent comment (overrides default)
    source: string         # Optional: Source tag (overrides default)
    no_date: bool          # Optional: Don't write creation date (overrides default)
```

Any settings specified in a preset will override the corresponding default settings. This allows you to set common values in the `default` section and only specify differences in individual presets.

Example presets.yaml:

```yaml
version: 1

# Defaults that always apply unless overridden
default:
  private: true
  no_date: true

presets:
  # opentrackr preset
  ptp:
    source: "PTP"
    trackers:
      - "https://please.passthe.tea/announce"
    # piece_length is automatically optimized based on file size
    # piece_length: 20  # manual override if needed (2^n: 14-24)
    # max_piece_length: 23  # limits the automatically calculated maximum piece length

  # Public tracker preset
  public:
    private: false  # overrides default preset
    trackers:
      - "udp://tracker.opentrackr.org:1337/announce"
      - "udp://open.tracker.cl:1337/announce"
      - "udp://9.rarbg.com:2810/announce"
    # piece_length is automatically optimized based on file size
    # piece_length: 22  # manual override if needed (2^n: 14-24)
    # max_piece_length: 23  # limits the automatically calculated maximum piece length
```

### Inspect a Torrent

```bash
mkbrr inspect <torrent-file>
```

The inspect command displays detailed information about a torrent file, including:

- Name and size
- Number of pieces and piece length
- Private flag status
- Info hash
- Tracker URL(s)
- Creation information
- Magnet link
- File list (for multi-file torrents)

### Modify a Torrent

```bash
mkbrr modify [torrent files...] [flags]
```

The modify command allows batch modification of existing torrent files using presets. Original files are preserved and new files are created with `-[preset]` or `-modified` suffix.

```bash
# Modify a single torrent using a preset
mkbrr modify -P public original.torrent

# Modify multiple torrents using a preset
mkbrr modify -P private file1.torrent file2.torrent

# Modify all torrent files in current directory
mkbrr modify -P public *.torrent
```

#### Modify Flags

- `-P, --preset <name>`: Use preset from config
- `--preset-file <file>`: Preset config file (default: ~/.config/mkbrr/presets.yaml)
- `--output-dir <dir>`: Output directory for modified files
- `-n, --dry-run`: Show what would be modified without making changes
- `-d, --no-date`: Don't update creation date
- `-v, --verbose`: Be verbose

The modify command uses the same preset configuration format as the create command. See [Preset Configuration Format](#preset-configuration-format) for details.

### Version Information

```bash
mkbrr version
```

Displays the version and build time of mkbrr.

### Update

```bash
mkbrr update
```

Self-updates the mkbrr binary if there is a new version available.

## Performance

mkbrr is blazingly fast, matching, and sometimes outperforming other popular torrent creation tools. Here are some benchmarks:

### 76GB Remux (Single File) [Ryzen 5 3600 / HDD]

```bash
# mktorrent
time mktorrent -p
Duration: 98.45s user 41.83s system 51% cpu 4:32.48 total

# mkbrr
time mkbrr create -p
Duration: 74.16s user 36.52s system 56% cpu 3:17.26 total
```

### 3.6GB Episode (Single File) [Apple Silicon M3 / NVME]

```bash
# mktorrent
time mktorrent -p
Duration: 1.34s user 0.49s system 103% cpu 1.766 total

# mkbrr
time mkbrr create -p
Duration: 1.27s user 0.67s system 122% cpu 1.587 total
```

### 350MB Music Album (15 Files) [Apple Silicon M3 / NVME]

```bash
# mktorrent
time mktorrent -p
Duration: 0.14s user 0.06s system 96% cpu 0.201 total

# mkbrr
time mkbrr create -p
Duration: 0.13s user 0.05s system 94% cpu 0.189 total
```

## License

This program is free software; you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation; either version 2 of the License, or (at your option) any later version.

See [LICENSE](LICENSE) for the full license text.
