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

Update your `~/.zshrc` to include:

```zsh
source ~/.config/duckhist/zsh-duckhist.zsh
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
- `duckhist force-version`: Force database schema version
  - `--config`: Specify configuration file path
  - `--update-to`: Specify version number to set (default: 2)

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

- Schema version management through migration files
  - Each migration file has a version number and is applied in order
  - Schema version is managed in the `schema_migrations` table in the database
  - Version can be forcibly set using the `force-version` command
- Migration files in `internal/migrations/` directory
  - `000001_create_history_table.up.sql`: Initial schema creation (history table)
  - `000002_add_primary_key_and_index.up.sql`: Add index on id column
- Rollback functionality support through down migration files

### History Management

- Generate ULID and convert to UUID for sortable IDs by timestamp
- Record command execution time, hostname, directory, and username
- Prioritize command history from current directory
