# mkbrr GUI Improvements

This document outlines planned improvements for the mkbrr GUI implementation.

## Core Functionality

- [x] Create basic GUI with Fyne toolkit
- [x] Implement the three main functions (Create, Inspect, Modify)
- [x] Connect GUI with existing CLI logic
- [x] Ensure cross-platform compatibility (Windows, macOS, Linux)

## Validation & Error Handling

- [ ] Add input validation for tracker URLs (proper URL format)
- [ ] Add validation for output file paths
- [ ] Improve error messages with clearer user guidance
- [ ] Add error logging to file for better debugging
- [ ] Add confirmation dialogs for potentially destructive actions

## UI/UX Improvements

- [ ] Improve file selection dialogs
  - [ ] Allow directory selection (currently only file selection works)
  - [ ] Add file filters for .torrent files in the appropriate dialogs
  - [ ] Remember last used directory
- [ ] Add progress reporting during torrent creation (percentage complete)
- [ ] Add ability to cancel long-running operations
- [ ] Improve display formatting of large numbers (file sizes, etc.)
- [ ] Add keyboard shortcuts for common actions

## New Features

- [ ] Add settings/preferences tab
  - [ ] Default tracker URLs
  - [ ] Default piece size
  - [ ] Default private flag setting
  - [ ] Default output directory
  - [ ] Save preferences to disk
- [ ] Add batch processing feature
  - [ ] Process multiple files in sequence
  - [ ] Import file list from text/CSV
  - [ ] Show batch progress
- [ ] Add preset support in GUI
  - [ ] Select from saved presets
  - [ ] Create/edit presets from GUI
- [ ] Add drag and drop support for files
- [ ] Add theme options
  - [ ] Dark theme toggle
  - [ ] Custom color schemes
- [ ] Add internationalization support

## Technical Debt & Optimization

- [ ] Refactor GUI code into separate package
- [ ] Add unit tests for GUI components
- [ ] Optimize memory usage for large torrents
- [ ] Add graceful shutdown and state saving
- [ ] Improve concurrent operations handling

## Documentation

- [ ] Add help tab with documentation
- [ ] Add tooltips for UI elements
- [ ] Create user guide for GUI
- [ ] Add keyboard shortcut reference

## Distribution

- [ ] Create platform-specific installers
- [ ] Add automatic updates for GUI version
- [ ] Create standalone GUI builds
- [ ] Add application icon and branding 