# duckhist

A Go program that stores zsh command history in DuckDB.

## Overview

duckhist is a Go language program designed to store zsh command history in DuckDB. It manages command history chronologically and enables efficient searching.

## Features

- Store zsh command history in DuckDB
- Display command history in chronological order (newest first)
- Filter command history based on current directory
- Incremental search integration with peco/fzf
- Schema management through migrations

## Installation

```bash
go build -o duckhist ./cmd
```

## Usage

### Initial Setup

```bash
duckhist init
```

This creates the default configuration file (`~/.config/duckhist/duckhist.toml`) and an empty database file (`~/.duckhist.duckdb`).

### zsh Configuration

Add the following to your `~/.zshrc`:

```zsh
zshaddhistory() {
    /path/to/duckhist add -- "$1"
}
```

### Display Command History

```bash
duckhist list
```

### Search Command History with Incremental Search

```zsh
# Using peco
duckhist history | peco

# Using fzf
duckhist history | fzf
```

## Command Line

### Global Options

- `--config CONFIG_FILE`: Path to configuration file (default: `~/.config/duckhist/duckhist.toml`)

### Subcommands

- `duckhist init`: Initialize settings
- `duckhist add -- <command>`: Add a command to history
- `duckhist list`: Display saved history in chronological order (newest first)
- `duckhist history`: Output command history for incremental search tools
- `duckhist schema-migrate`: Update database schema to the latest version

## Configuration File

The configuration file (default: `~/.config/duckhist/duckhist.toml`) allows you to set the following options:

```toml
# Path to DuckDB database file
database_path = "~/.duckhist.duckdb"

# Number of history entries to display for current directory (default: 5)
current_directory_history_limit = 5
```

## Dependencies

- `github.com/duckdb/duckdb-go`: DuckDB Go bindings
- `github.com/spf13/cobra`: Used for building command-line interfaces

## Project Directory Structure

- `cmd/`: Command-line tool entry points
- `internal/`: Internal logic
  - `history/`: History management logic
  - `config/`: Configuration file management
  - `migrations/`: Database schema migrations
- `scripts/`: Scripts and utilities
- `test/`: Test code

## Implementation Details

### Schema Management

- Schema version management using golang-migrate/migrate
- Safe schema updates through migration files
- Rollback functionality support
- SQLite driver used to ensure compatibility with DuckDB

### History Management

- Generate ULID and convert to UUID for sortable IDs by timestamp
- Record command execution time, hostname, directory, and username
- Prioritize command history from current directory
