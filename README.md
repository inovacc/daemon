# daemon

daemon is a CLI application

## Installation

```bash
go install github.com/inovacc/daemon@latest
```

## Usage

```bash
daemon --help
```

## Commands

| Command | Description |
|---------|-------------|
| `version` | Print version information |

## Development

```bash
# Build
task build

# Run
task run

# Test
task test

# Lint
task lint
```

## Release

```bash
# Create a snapshot release
task release:snapshot

# Create a production release (requires git tag)
git tag v1.0.0
task release
```

## License

BSD-3
