# search subcommand

The `search` subcommand provides an interactive interface to search through your command history.

## Usage

```bash
duckhist search [flags]
```

### Flags

- `-d, --directory string`: Directory to search history for (default is current directory)

## Features

### Display Format

Commands are displayed in a table format with the following columns:

| Date        | Command                                  | Directory           |
| ----------- | ---------------------------------------- | ------------------- |
| 2 hours ago | git commit -m "feat: add search command" | ~/projects/duckhist |

- Date: Displayed as relative time (e.g., "2 hours ago", "3 days ago")
- Command: The actual command that was executed
- Directory: The directory where the command was executed, with home directory shortened to `~`

The table format provides a clear and organized view of your command history, making it easy to scan through entries and find specific commands.

### Display Order

The command history is displayed with:

1. Commands executed in the current directory (or specified directory with `-d` flag)
2. Commands from other directories

Within each section, commands are displayed chronologically with:

- Older commands at the top
- Newer commands at the bottom

This matches the typical terminal behavior where new output appears at the bottom.

### Interactive Search

As you type in the search box:

- The list updates in real-time to show only commands matching your search query
- Matching is case-insensitive and searches within commands

### Key Bindings

- `Up/Down`: Navigate through the command list
- `Enter/Tab`: Select the current command and exit
- `Esc`: Exit without selecting a command

## Examples

1. Search in current directory:

```bash
duckhist search
```

2. Search in specific directory:

```bash
duckhist search -d /path/to/directory
```
