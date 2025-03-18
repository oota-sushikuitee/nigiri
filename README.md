# Nigiri

Nigiri is a tool for managing upstream VCS repositories and build artifacts. It allows you to easily build, run, and manage different versions of upstream projects.

## Features

- Build projects from Git repositories
- Manage multiple versions of the same project using different commits
- Run built binaries with convenient command-line syntax
- Support for private repositories using GitHub tokens
- Configurable build commands for different operating systems
- Working directory support for repositories with subdirectories
- Storage optimization with binary-only mode and source code compression

## Installation

### From Source

```bash
git clone https://github.com/oota-sushikuitee/nigiri.git
cd nigiri
make build
```

After building, you can install the binary to your PATH or use it directly from the `bin` directory.

## Getting Started

1. Initialize nigiri configuration:

```bash
./bin/nigiri init
```

This creates a configuration file at `~/.nigiri/.nigiri.yml`.

2. Edit the configuration file to add your targets.

3. Build and run your targets as needed.

## Configuration

The configuration file is located at `~/.nigiri/.nigiri.yml`. Here's an example configuration:

```yaml
targets:
  sample-project:
    source: https://github.com/octocat/Hello-World
    default-branch: main
    # The directory within the repository to run the build command (optional)
    working-directory: ""
    # Whether to keep only the binary and remove source code after build (optional)
    binary-only: false
    build-command:
      linux: make build
      windows: make build
      darwin: make build
      # Path to the built binary (relative to working directory or repository root)
      binary-path: bin/myapp
    env:
      - "GO111MODULE=on"
      - "CGO_ENABLED=0"
```

### Configuration Options

- `source`: Git repository URL
- `default-branch`: Default branch to use if no commit is specified
- `working-directory`: Subdirectory within the repository to run build commands (optional)
- `binary-only`: Whether to keep only the binary and remove source code after building (optional)
- `build-command`: OS-specific build commands
  - `linux`, `windows`, `darwin`: Build commands for each OS
  - `binary-path`: Path to the built binary relative to the repository root
- `env`: Environment variables to set during build and run

## Commands

### Initialize

Create a new nigiri configuration file:

```bash
nigiri init
```

### List

List all configured targets:

```bash
nigiri list
```

### Build

Build a target at a specific commit:

```bash
nigiri build <target> [commit]
```

To build a target with GitHub token authentication (for private repositories):

```bash
nigiri build <target> --use-token
# or
nigiri build <target> -t
```

To force rebuild even if the target has already been built:

```bash
nigiri build <target> --force
# or
nigiri build <target> -f
```

To build with a specific clone depth:

```bash
nigiri build <target> --depth <depth>
# or
nigiri build <target> -d <depth>
```

For verbose output:

```bash
nigiri build <target> --verbose
# or
nigiri build <target> -v
```

### Run

Run a built target:

```bash
nigiri run <target> [commit] [args...]
```

If the commit is not specified, the latest built commit will be used.

#### Examples

Run the latest build of a target:
```bash
nigiri run <target>
```

Run a specific commit:
```bash
nigiri run <target> <commit>
```

Run with HEAD (latest commit) explicitly:
```bash
nigiri run <target> HEAD
```

Run and pass arguments to the target:
```bash
nigiri run <target> <commit> arg1 arg2
```

Run with arguments including flags:
```bash
nigiri run <target> HEAD -v --flag=value
```

Explicitly separate nigiri arguments from target arguments:
```bash
nigiri run <target> <commit> -- -v --flag=value
```

Note: When the second argument starts with `-`, it's treated as an argument for the target program, not a commit hash.

### Remove

Remove a built target:

```bash
nigiri remove <target> [commit]
```

If the commit is not specified, all builds of the target will be removed.

### Cleanup

Remove all builds:

```bash
nigiri cleanup
```

## Advanced Features

### Private Repositories

For private repositories, you need to provide authentication. Nigiri supports GitHub token authentication:

```bash
nigiri build <target> --use-token
```

The token is automatically sourced from:
1. GitHub CLI (`gh auth token`)
2. GITHUB_TOKEN environment variable

### Working Directory

If your project requires building from a specific subdirectory, use the `working-directory` option in your configuration:

```yaml
targets:
  my-project:
    source: https://github.com/octocat/project-example
    working-directory: "cmd/app"
    # ... other options
```

### Binary-Only Mode

To save disk space, you can enable binary-only mode, which only keeps the compiled binary and removes the source code:

```yaml
targets:
  my-project:
    source: https://github.com/example/repository
    binary-only: true
    # ... other options
```

When binary-only is disabled (default), nigiri will compress the source code to save space while still keeping it available.

## License

Nigiri is licensed under the MIT License. See [LICENSE](./LICENSE) for more information.
