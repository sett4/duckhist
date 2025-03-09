# Add Subcommand

The `add` subcommand is responsible for adding command history entries to the DuckDB database. It captures not only the command itself but also contextual information such as the working directory, TTY, session ID, hostname, and username.

## Usage

```bash
duckhist add [flags] -- <command>
```

The `--` separator is required to prevent flags in the command from being interpreted as flags for the `add` subcommand.

## Flags

- `--verbose, -v`: Enable verbose output
- `--directory, -d`: Specify the directory to record (defaults to current directory)
- `--tty`: Specify the TTY (defaults to $TTY environment variable)
- `--sid`: Specify the Session ID
- `--no-dedup`: Allow duplicate commands (by default, duplicate commands are skipped)

## Implementation Details

### Command Processing

1. The command is processed through the `CommandAdder` struct, which handles:

   - Trimming whitespace from the command
   - Validating that the command is not empty
   - Loading configuration from the specified config file
   - Managing database connections through the history manager

2. Context Information Collection:
   - Working Directory: Uses the specified directory or current working directory
   - TTY: Uses the specified TTY or $TTY environment variable
   - Session ID: Uses the specified SID
   - Hostname: Automatically retrieved from the system
   - Username: Retrieved from the USER environment variable

### Deduplication

By default, the add subcommand will skip duplicate commands in the same context (same directory, hostname, and username). This helps keep the history clean and avoids redundant entries. A command is considered a duplicate if:

- The command text is exactly the same
- It was executed in the same directory
- It was executed on the same host
- It was executed by the same user

You can override this behavior with the `--no-dedup` flag to force adding duplicate commands.

When using verbose mode (`-v`):

- Skipped duplicates will show: "Skipping duplicate command: [command]"
- Forced duplicates will show: "Adding duplicate command: [command]"

### Error Handling

- Empty commands are rejected with an "empty command" error
- Configuration loading errors are reported with detailed messages
- Database connection errors are handled gracefully
- Command addition failures are reported with specific error messages

### Verbose Mode

When verbose mode is enabled (`-v` flag), the following information is output:

- Confirmation when a command is successfully added
- Empty command notifications when applicable

## Examples

1. Add a simple command:

```bash
duckhist add -- ls -la
```

2. Add a command with a specific directory:

```bash
duckhist add -d /path/to/directory -- git status
```

3. Add a command with verbose output:

```bash
duckhist add -v -- npm install express
```

4. Add a command with TTY and Session ID:

```bash
duckhist add --tty /dev/pts/1 --sid 12345 -- docker ps
```

## Database Schema

The command is stored in the history table with the following information:

- Command text
- Working directory
- TTY (if available)
- Session ID (if specified)
- Hostname
- Username
- Timestamp (automatically added)

## Integration

The add subcommand is typically used through shell integration (e.g., zsh-duckhist.zsh) to automatically record commands as they are executed. However, it can also be used manually to add specific commands to the history.

## Error Codes

- Exit code 1: Empty command
- Exit code 1: Configuration or database errors
