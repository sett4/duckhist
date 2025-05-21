# import-history Subcommand

The `import-history` subcommand imports commands from your Zsh history file (`~/.zsh_history`) into the Duckhist database.

## Usage

```bash
duckhist import-history
```

This command does not take any flags or arguments.

## Description

The `import-history` command reads commands directly from the Zsh history file, typically located at `~/.zsh_history`, and saves them to the history database. This is useful for populating Duckhist with your existing shell history.

## Details

### Zsh History File

- **Path:** The command attempts to read from `~/.zsh_history`. This path is determined by first finding the user's home directory.
- **File Not Found:** If the `~/.zsh_history` file is not found at the expected location, the command will print a "History file not found" message and exit gracefully without importing any commands.

### Parsing Zsh History Lines

The importer is designed to parse common Zsh history formats:

1.  **Extended Format:** Lines starting with `: <timestamp>:<duration>;<command>`
    *   Example: `: 1678886400:0;ls -l`
    *   The `<timestamp>` is a Unix epoch timestamp.
    *   The `<duration>` is ignored.
    *   The `<command>` is the actual command executed.

2.  **Simple Format:** Lines containing only the command text.
    *   Example: `echo "hello world"`

Lines that do not conform to these patterns, or malformed extended lines, might be skipped or an attempt will be made to parse them as simple commands. Any leading/trailing whitespace from the command text is trimmed. Empty commands (after trimming) are skipped.

### Timestamp Handling

- **From Extended Format:** If a line is in the extended format and a valid Unix timestamp is found, that timestamp is used for the `executed_at` field of the imported command.
- **No Timestamp / Invalid Timestamp:** If a line is in the simple format, or if the timestamp in the extended format is missing or cannot be parsed, the current system time (`time.Now()`) at the moment of import is used as the `executed_at` timestamp.

### Populating Other Fields

When importing commands from `~/.zsh_history`, other contextual fields are populated as follows:

- **`hostname`**: Filled with the current system's hostname (obtained via `os.Hostname()`).
- **`directory`**: Filled with the current working directory from which the `duckhist import-history` command is run (obtained via `os.Getwd()`), not the directory where the original command was executed in Zsh.
- **`user`**: Filled with the current user's username (obtained from the `USER` environment variable).
- **`tty`**: Stored as an empty string, as this information is not available in the Zsh history file.
- **`sid`** (Session ID): Stored as an empty string, as this information is not available in the Zsh history file.

### Deduplication

The `import-history` command imports all entries it can parse from the `~/.zsh_history` file. It uses the equivalent of the `--no-dedup` flag internally, meaning it will add entries to the Duckhist database even if similar command texts already exist. The primary goal is to get a bulk import of raw history.

### Output

Upon completion, the command prints a summary message indicating the number of commands successfully imported and the path to the Zsh history file from which they were read.

Example output:
```
Imported 1523 commands from /Users/youruser/.zsh_history
```

If errors occur during the processing of specific lines (e.g., parsing issues), messages may be logged to standard error, but the command will attempt to continue importing other valid entries.
