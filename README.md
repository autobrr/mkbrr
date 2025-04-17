
<h1 align="center">⚡ mkbrr</h1> 
<p align="center"> 
  <strong>Simple. Smart. Fast.</strong><br> 
  A powerful CLI tool to create, inspect, and modify torrent files. Private by default. Tracker aware.
</p> 
<p align="center"> 
  <img src="https://img.shields.io/badge/Go-1.23-blue?logo=go" alt="Go version">
  <img src="https://img.shields.io/badge/build-passing-brightgreen" alt="Build Status">
  <img src="https://img.shields.io/github/v/release/autobrr/mkbrr" alt="Latest Release">
  <img src="https://img.shields.io/discord/881212911849209957?label=discord&logo=discord" alt="Discord">
</p>

<img src=".github/assets/mkbrr-dark.png" alt="mkbrr gopher" width="60" align="right"/>

## Overview

[Website](https://mkbrr.com) | [Releases](https://github.com/autobrr/mkbrr/releases)

**mkbrr** (pronounced "make-burr") is a simple yet powerful tool for:
- Creating torrent files
- Inspecting torrent files
- Modifying torrent metadata
- Supporting tracker-specific requirements automatically

**Key Features:**
- **Fast**: Blazingly fast hashing beating the competition
- **Simple**: Easy to use CLI
- **Portable**: Single binary with no dependencies
- **Smart**: Detects possible missing files when creating torrents for season packs

For comprehensive documentation and guides, visit [mkbrr.com](https://mkbrr.com).

## Quick Start

### Install

#### Pre-built binaries

Download a ready-to-use binary for your platform from the [releases page](https://github.com/autobrr/mkbrr/releases).

#### Homebrew

```bash
brew tap autobrr/mkbrr
brew install mkbrr
```

### Basic Usage

```bash
# Create a private torrent (default)
mkbrr create path/to/file -t https://example-tracker.com/announce

# Create a public torrent
mkbrr create path/to/file -t https://example-tracker.com/announce --private=false

# Create with randomized info hash
mkbrr create path/to/file -t https://example-tracker.com/announce -e

# Inspect a torrent
mkbrr inspect my-torrent.torrent

# Modify a torrent
mkbrr modify original.torrent --tracker https://new-tracker.com
```

## Additional Information

For detailed documentation on:
- Installation options
- Advanced usage
- Preset and batch modes
- Tracker-specific features
- Season pack detection
- Performance benchmarks

Please visit [mkbrr.com](https://mkbrr.com).

## License

This program is free software; you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation; either version 2 of the License, or (at your option) any later version.

See [LICENSE](LICENSE) for the full license text.
