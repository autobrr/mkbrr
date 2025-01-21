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
    - [Create Flags](#create-flags)
    - [Batch Configuration Format](#batch-configuration-format)
  - [Inspect a Torrent](#inspect-a-torrent)
  - [Version Information](#version-information)
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
    name: "Ubuntu 22.04 LTS"
    trackers:
      - udp://tracker.opentrackr.org:1337/announce
    webseeds:
      - https://releases.ubuntu.com/22.04/ubuntu-22.04.3-desktop-amd64.iso
    piece_length: 20  # 1MB pieces (2^20)
    comment: "Ubuntu 22.04.3 LTS Desktop AMD64"
    private: false

  - output: release.torrent
    path: /path/to/release
    name: "My Release"
    trackers:
      - udp://tracker.example.com:1337/announce
    piece_length: 18  # 256KB pieces (2^18)
    private: true
    source: "GROUP"
```

Batch mode will process all jobs in parallel (up to 4 concurrent jobs) and provide a summary of results.

#### Create Flags

General flags:

- `-b, --batch <file>`: Use batch configuration file (YAML)
- `-v, --verbose`: Be verbose

Single mode flags:

- `-t, --tracker <url>`: Tracker URL
- `-w, --web-seed <url>`: Add web seed URLs (can be specified multiple times)
- `-p, --private`: Make torrent private
- `-c, --comment <text>`: Add comment
- `-l, --piece-length <n>`: Set piece length to 2^n bytes (14-24, automatic if not specified)
- `-o, --output <path>`: Set output path (default: <name>.torrent)
- `-n, --name <name>`: Set torrent name (default: basename of target)
- `-s, --source <text>`: Add source string
- `-d, --no-date`: Don't write creation date

Note: When using batch mode (-b), torrent settings are specified in the YAML configuration file instead of command line flags.

#### Batch Configuration Format

The batch configuration file uses YAML format with the following structure:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/autobrr/mkbrr/main/schema/batch.json
version: 1  # Required, must be 1
jobs:       # List of torrent creation jobs
  - output: string      # Required: Output path for .torrent file
    path: string        # Required: Path to source file/directory
    name: string        # Optional: Torrent name (default: basename of path)
    trackers:           # Optional: List of tracker URLs
      - string
    webseeds:           # Optional: List of webseed URLs
      - string
    private: bool       # Optional: Make torrent private (default: false)
    piece_length: int   # Optional: Piece length exponent (14-24)
    comment: string     # Optional: Torrent comment
    source: string      # Optional: Source tag
    no_date: bool       # Optional: Don't write creation date (default: false)
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

### Version Information

```bash
mkbrr version
```

Displays the version and build time of mkbrr.

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
