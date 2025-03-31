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
```

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

# Create with randomized info hash
mkbrr create path/to/file -t https://example-tracker.com/announce -e
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
export PATH=$PATH:$(go env GOPATH)/bin
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

# Create with randomized info hash
mkbrr create path/to/file -t https://example-tracker.com/announce -e

# Create a torrent excluding specific file patterns (comma-separated)
mkbrr create path/to/file -t https://example-tracker.com/announce --exclude "*.nfo,*.jpg"
```

> [!NOTE]
> The exclude patterns feature supports standard glob pattern matching (like `*` for any number of characters, `?` for a single character) and is case-insensitive.

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

# Randomize info hash
mkbrr modify original.torrent -e
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
> The preset file can be placed in the current directory, `~/.config/mkbrr/`, or `~/.mkbrr/`. You can also specify a custom location with `--preset-file`. Presets support the `exclude_patterns` field, allowing you to define default or preset-specific file exclusions.

### Batch Mode

Create multiple torrents at once using a YAML configuration file:

```bash
mkbrr create -b batch.yaml
```

See [batch example](examples/batch.yaml) here.

> [!TIP]
> Batch mode processes jobs in parallel (up to 4 at once) and shows a summary when complete. Batch mode also support the `exclude_patterns` field.

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

mkbrr is optimized for speed and consistently outperforms other popular torrent creation tools in our benchmarks.

### Benchmark Results

| Hardware | Test File Size | mkbrr | mktorrent | torrenttools | torf |
|----------|---------------|-------|-----------|--------------|------|
| **Leaseweb Server (NVME)** | 21 GiB | **6.49s** | 111.12s | 14.05s | 13.18s |
| **Hetzner Server (HDD)** | 14 GiB | **40.58s** | 53.72s | 44.65s | 45.73s |
| **Macbook Pro M4** | 30 GiB | **9.80s** | 10.40s | - | 10.41s |

### Speed Comparison

| Hardware | vs mktorrent | vs torrenttools | vs torf |
|----------|-------------|----------------|---------|
| **Leaseweb Server (NVME)** | 17.1× faster | 2.2× faster | 2.0× faster |
| **Hetzner Server (HDD)** | 1.3× faster | 1.1× faster | 1.1× faster |
| **Macbook Pro M4** | 1.1× faster | - | 1.1× faster |

### Consistency

Besides raw speed, mkbrr shows more consistent performance between runs, with standard deviation percentages between 1.2-2.6% across platforms compared to much higher variances for other tools (up to 35%). This predictable performance is particularly noticeable on mechanical storage where other tools showed wider fluctuations.

### Hardware Specifications

#### Leaseweb Dedicated Server (NVME)
- CPU: Intel Xeon E-2274G @ 4.00GHz
- RAM: 32GB
- Storage: 1 × 1.7TB NVME

#### Hetzner Dedicated Server (HDD)
- CPU: AMD Ryzen 5 3600 (12) @ 4.71GHz
- RAM: 64GB
- Storage: 4 × TOSHIBA MG08ACA16TEY in RAID0

#### Macbook Pro M4
- CPU: Apple M4
- RAM: 16GB
- Storage: 512GB NVME

### Benchmark Methodology

All tests were performed using [hyperfine](https://github.com/sharkdp/hyperfine) with 5 runs per tool after a warm-up run. For the HDD test, caches were cleared between runs.

<details>
<summary>View Full Benchmark Commands & Results</summary>

#### Leaseweb Server (21 GiB 1080p season pack)

```bash
hyperfine --warmup 1 --runs 5 \
  'mkbrr create /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP' \
  'rm -f /home/user/Show.S01.DL.1080p.WEB.H264-GROUP.torrent && mktorrent /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP' \
  'torrenttools create --threads 8 ~/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP' \
  'rm -f /home/user/Show.S01.DL.1080p.WEB.H264-GROUP.torrent && torf --threads 8 /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP'

Benchmark 1: mkbrr create /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP
  Time (mean ± σ):      6.490 s ±  0.168 s    [User: 36.629 s, System: 8.008 s]
  Range (min … max):    6.263 s …  6.728 s    5 runs

Benchmark 2: rm -f /home/user/Show.S01.DL.1080p.WEB.H264-GROUP.torrent && mktorrent /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP
  Time (mean ± σ):     111.122 s ±  1.554 s    [User: 100.569 s, System: 8.460 s]
  Range (min … max):   109.896 s … 113.402 s    5 runs

Benchmark 3: torrenttools create --threads 8 ~/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP
  Time (mean ± σ):     14.049 s ±  4.973 s    [User: 31.210 s, System: 12.175 s]
  Range (min … max):    7.519 s … 20.537 s    5 runs

Benchmark 4: rm -f /home/user/Show.S01.DL.1080p.WEB.H264-GROUP.torrent && torf --threads 8 /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP
  Time (mean ± σ):     13.184 s ±  2.244 s    [User: 29.352 s, System: 11.466 s]
  Range (min … max):    9.486 s … 15.239 s    5 runs

Summary
  'mkbrr create /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP' ran
    2.03 ± 0.35 times faster than 'rm -f /home/user/Show.S01.DL.1080p.WEB.H264-GROUP.torrent && torf --threads 8 /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP'
    2.16 ± 0.77 times faster than 'torrenttools create --threads 8 ~/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP'
   17.12 ± 0.50 times faster than 'rm -f /home/user/Show.S01.DL.1080p.WEB.H264-GROUP.torrent && mktorrent /home/user/torrents/qbittorrent/Show.S01.DL.1080p.WEB.H264-GROUP'
```

#### Hetzner Server (14 GiB 1080p season pack)

```bash
hyperfine --warmup 1 --runs 5 \
  --setup 'sudo sync && sudo sh -c "echo 3 > /proc/sys/vm/drop_caches"' \
  --prepare 'sudo sync && sudo sh -c "echo 3 > /proc/sys/vm/drop_caches"' \
  'mkbrr create ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP' \
  'rm -f /home/user/mkbrr/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && mktorrent ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP' \
  'torrenttools create --threads 12 ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP' \
  'rm -f /home/user/mkbrr/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && torf --threads 12 ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP'

Benchmark 1: mkbrr create ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
  Time (mean ± σ):     40.579 s ±  0.476 s    [User: 13.620 s, System: 6.804 s]
  Range (min … max):   39.897 s … 40.961 s    5 runs

Benchmark 2: rm -f /home/user/mkbrr/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && mktorrent ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
  Time (mean ± σ):     53.719 s ± 15.986 s    [User: 17.921 s, System: 6.605 s]
  Range (min … max):   37.742 s … 73.110 s    5 runs

Benchmark 3: torrenttools create --threads 12 ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
  Time (mean ± σ):     44.648 s ±  1.837 s    [User: 6.981 s, System: 6.624 s]
  Range (min … max):   42.156 s … 46.974 s    5 runs

Benchmark 4: rm -f /home/user/mkbrr/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && torf --threads 12 ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
  Time (mean ± σ):     45.734 s ±  3.433 s    [User: 7.249 s, System: 6.483 s]
  Range (min … max):   42.116 s … 51.233 s    5 runs

Summary
  mkbrr create ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP ran
    1.10 ± 0.05 times faster than torrenttools create --threads 12 ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
    1.13 ± 0.09 times faster than rm -f /home/user/mkbrr/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && torf --threads 12 ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
    1.32 ± 0.39 times faster than rm -f /home/user/mkbrr/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && mktorrent ~/torrents/qbittorrent/tv/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
```

#### Macbook Pro M4 (30 GiB 1080p season pack)

```bash
hyperfine --warmup 1 --runs 5 \
  'mkbrr create ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP' \
  'rm -f Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && mktorrent ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP' \
  'rm -f Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && torf --threads 10 ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP'

Benchmark 1: mkbrr create ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
  Time (mean ± σ):      9.796 s ±  0.199 s    [User: 11.036 s, System: 4.612 s]
  Range (min … max):    9.628 s … 10.052 s    5 runs

Benchmark 2: rm -f Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && mktorrent ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
  Time (mean ± σ):     10.402 s ±  0.531 s    [User: 11.053 s, System: 3.137 s]
  Range (min … max):    9.918 s … 11.002 s    5 runs

Benchmark 3: rm -f Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && torf --threads 10 ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
  Time (mean ± σ):     10.407 s ±  0.601 s    [User: 10.920 s, System: 5.365 s]
  Range (min … max):    9.529 s … 11.202 s    5 runs

Summary
  mkbrr create ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP ran
    1.06 ± 0.06 times faster than rm -f Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && mktorrent ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
    1.06 ± 0.07 times faster than rm -f Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP.torrent && torf --threads 12 ~/Desktop/Show.S01.1080p.SRC.WEB-DL.DDP5.1.H.264-GRP
```

</details>

## License

This program is free software; you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation; either version 2 of the License, or (at your option) any later version.

See [LICENSE](LICENSE) for the full license text.
