# mkbrr

```
         __   ___.                 
  _____ |  | _\_ |________________ 
 /     \|  |/ /| __ \_  __ \_  __ \
|  Y Y  \    < | \_\ \  | \/|  | \/
|__|_|  /__|_ \|___  /__|   |__|   
      \/     \/    \/              

mkbrr is a tool to create and inspect torrent files.

Usage:
  mkbrr [command]

Available Commands:
  create      Create a new torrent file
  inspect     Inspect a torrent file
  modify      Modify existing torrent files using a preset
  update      Update mkbrr
  version     Print version information
  help        Help about any command

Flags:
  -h, --help   help for mkbrr

Use "mkbrr [command] --help" for more information about a command.
````

## What is mkbrr?

**mkbrr** (pronounced "make-burr") is a simple yet powerful tool for:
- Creating torrent files
- Inspecting torrent files
- Modifying torrent metadata
- Supports tracker-specific requirements automatically

**Why use mkbrr?**
- 🚀 **Fast**: Blazingly fast hashing beating the competition
- 🔧 **Simple**: Easy to use CLI
- 📦 **Portable**: Single binary with no dependencies
- 💡 **Smart**: Will attempt to detect possible missing files when creating torrents for season packs

## Quick Start

### Install

#### Pre-built binaries

Download a ready-to-use binary for your platform from the [releases page](https://github.com/autobrr/mkbrr/releases).

#### Homebrew

```bash
brew tap autobrr/mkbrr
brew install mkbrr
```

### Creating a Torrent

```bash
# torrents are private by default
mkbrr create path/to/file -t https://example-tracker.com/announce

# public torrent
mkbrr create path/to/file -t https://example-tracker.com/announce --private=false
```

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
  - [Creating Torrents](#creating-torrents)
  - [Inspecting Torrents](#inspecting-torrents)
  - [Modifying Torrents](#modifying-torrents)
- [Advanced Usage](#advanced-usage)
  - [Preset Mode](#preset-mode)
  - [Batch Mode](#batch-mode)
- [Tracker-Specific Features](#tracker-specific-features)
- [Incomplete Season Pack Detection](#incomplete-season-pack-detection)
- [Performance](#performance)
- [License](#license)

## Installation

Choose the method that works best for you:

### Prebuilt Binaries

Download a ready-to-use binary for your platform from the [releases page](https://github.com/autobrr/mkbrr/releases).

### Homebrew (macOS and Linux)

```bash
brew tap autobrr/mkbrr
brew install mkbrr
```

### Build from Source

Requirements:
See [go.mod](https://github.com/autobrr/mkbrr/blob/main/go.mod#L3) for Go version.

```bash
# Clone the repository
git clone https://github.com/autobrr/mkbrr.git
cd mkbrr

# Install the binary to $GOPATH/bin
make install

# Or install system-wide (requires sudo)
sudo make install    # installs to /usr/local/bin
```

### Go Install

If you have Go installed:

```bash
go install github.com/autobrr/mkbrr@latest

# make sure its in your PATH
export PATH="$PATH:$GOPATH/bin"
```

## Usage

### Creating Torrents

The basic command structure for creating torrents is:

```bash
mkbrr create [path] [flags]
```

For help:

```bash
mkbrr create --help
```

#### Basic Examples

```bash
# Create a private torrent (default)
mkbrr create path/to/file -t https://example-tracker.com/announce

# Create a public torrent
mkbrr create path/to/file -t https://example-tracker.com/announce --private=false

# Create with a comment
mkbrr create path/to/file -t https://example-tracker.com/announce -c "My awesome content"

# Create with a custom output path
mkbrr create path/to/file -t https://example-tracker.com/announce -o custom-name.torrent
```

### Inspecting Torrents

View detailed information about a torrent:

```bash
mkbrr inspect my-torrent.torrent
```

This shows:
- Name and size
- Piece information and hash
- Tracker URLs
- Creation date
- Magnet link
- File list (for multi-file torrents)

### Modifying Torrents

Update metadata in existing torrent files without access to the original content:

```bash
# Basic usage
mkbrr modify original.torrent --tracker https://new-tracker.com

# Modify multiple torrents
mkbrr modify *.torrent --private=false

# See what would be changed without making actual changes
mkbrr modify original.torrent --tracker https://new-tracker.com --dry-run
```

## Advanced Usage

### Preset Mode

Presets save you time by storing commonly used settings. Great for users who create torrents for the same trackers regularly.

See [presets example](examples/presets.yaml) here.

```bash
# Uses the ptp-preset (defined in your presets.yaml file)
mkbrr create -P ptp path/to/file

# Override some preset values
mkbrr create -P ptp --source "MySource" path/to/file
```

> [!TIP]
> The preset file can be placed in the current directory, `~/.config/mkbrr/`, or `~/.mkbrr/`. You can also specify a custom location with `--preset-file`.

### Batch Mode

Create multiple torrents at once using a YAML configuration file:

```bash
mkbrr create -b batch.yaml
```

See [batch example](examples/batch.yaml) here.

> [!TIP]
> Batch mode processes jobs in parallel (up to 4 at once) and shows a summary when complete.

## Tracker-Specific Features

mkbrr automatically enforces some requirements for various private trackers so you don't have to:

#### Piece Length Limits

Different trackers have different requirements:
- HDB, BHD, SuperBits: Max 16 MiB pieces
- Emp, MTV: Max 8 MiB pieces
- GazelleGames: Max 64 MiB pieces

#### Torrent Size Limits

Some trackers limit the size of the .torrent file itself:
- Anthelion: 250 KiB
- GazelleGames: 1 MB

> [!INFO]
> When creating torrents for these trackers, mkbrr automatically adjusts piece sizes to meet requirements, so you don't have to.

A full overview over tracker-specific limits can be seen in [trackers.go](internal/trackers/trackers.go)

## Incomplete Season Pack Detection

If the input is a folder with a name that indicates that its a pack, it will find the highest number and do a count to look for missing files.

```
mkbrr create ~/Kyles.Original.Sins.S01.1080p.SRC.WEB-DL.DDP5.1.H.264 -t https://tracker.com/announce/1234567

Files being hashed:
  ├─ Kyles.Original.Sins.S01E01.Business.and.Pleasure.1080p.SRC.WEB-DL.DDP5.1.H.264.mkv (3.3 GiB)
  ├─ Kyles.Original.Sins.S01E02.Putting.It.Back.In.1080p.SRC.WEB-DL.DDP5.1.H.264.mkv (3.4 GiB)
  └─ Kyles.Original.Sins.S01E04.Cursor.For.Life.1080p.SRC.WEB-DL.DDP5.1.H.264.mkv (3.3 GiB)


Warning: Possible incomplete season pack detected
  Season number: 1
  Highest episode number found: 4
  Video files: 3

This may be an incomplete season pack. Check files before uploading.

Hashing pieces... [3220.23 MB/s] 100% [========================================]

Wrote title.torrent (elapsed 3.22s)
```


## Performance

mkbrr is optimized for speed, and outperforms other popular tools.
We will post some benchmarks later on.

## License

This program is free software; you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation; either version 2 of the License, or (at your option) any later version.

See [LICENSE](LICENSE) for the full license text.
