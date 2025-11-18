# Local CI Testing Scripts

This directory contains scripts to automate local CI pipeline testing using [act](https://github.com/nektos/act).

## Prerequisites

- **act** is installed (automatically managed by mise)
- **Docker** is running (required by act to run containers)

## Quick Start

The easiest way to run local CI tests is using mise tasks:

```bash
# Run the full CI pipeline locally
mise run ci:local

# Run just the test job
mise run ci:test

# List all available jobs
mise run ci:list

# Simulate a pull request event
mise run ci:pr

# Run with verbose output
mise run ci:verbose

# Dry run (show what would run)
mise run ci:dry-run
```

## Available Mise Tasks

| Task | Description |
|------|-------------|
| `ci:local` | Run full CI pipeline locally using act |
| `ci:test` | Run test job locally |
| `ci:verbose` | Run CI locally with verbose output |
| `ci:list` | List available CI jobs |
| `ci:pr` | Run CI as pull_request event |
| `ci:dry-run` | Show what would run without executing |

## Direct Script Usage

You can also use the script directly for more control:

```bash
# Basic usage
./scripts/run-ci-local.sh

# Run specific job
./scripts/run-ci-local.sh -j test

# Run with verbose output
./scripts/run-ci-local.sh -v

# Simulate pull request
./scripts/run-ci-local.sh -e pull_request

# Show help
./scripts/run-ci-local.sh -h
```

## Script Options

- `-j, --job JOB` - Run specific job (default: all jobs)
- `-w, --workflow FILE` - Workflow file to run (default: tests.yml)
- `-v, --verbose` - Enable verbose output
- `-l, --list` - List available jobs and exit
- `-n, --dry-run` - Show what would be run without executing
- `-e, --event EVENT` - Event type: push, pull_request (default: push)
- `-p, --platform PLATFORM` - Custom platform mapping
- `-h, --help` - Show help message

## How It Works

The script uses `act` to run GitHub Actions workflows locally in Docker containers. This allows you to:

1. Test workflow changes before pushing
2. Debug failing CI pipelines locally
3. Iterate faster without waiting for GitHub Actions
4. Run CI checks offline

## Troubleshooting

### Docker not running
If you see connection errors, make sure Docker is running:
```bash
docker ps
```

### Missing act
If act is not found, install it via mise:
```bash
mise install act
```

### Platform issues
If you encounter platform-specific issues, you can specify a custom platform:
```bash
./scripts/run-ci-local.sh -p "ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-22.04"
```

## Integration with Development Workflow

Before pushing changes, run:
```bash
# Run local checks
mise run check

# Run local CI pipeline
mise run ci:local
```

This ensures your code passes all checks before creating a pull request.
