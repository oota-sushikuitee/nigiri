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

### Homebrew

```bash
brew install Okabe-Junya/tap/nigiri
```

### go install

```bash
go install github.com/oota-sushikuitee/nigiri/cmd/nigiri@latest
```

Replace `@latest` with a tag such as `@v0.2.1` to pin a specific version.

### Release archives

Prebuilt archives for Linux, macOS and Windows are attached to each [release](https://github.com/oota-sushikuitee/nigiri/releases). Download the archive for your platform and put the `nigiri` binary on your PATH.

### From Source

```bash
git clone https://github.com/oota-sushikuitee/nigiri.git
cd nigiri
make build
```

After building, you can install the binary to your PATH or use it directly from the `bin` directory. A source build reports its version as `dev`.

## Getting Started

1. Initialize nigiri configuration:

```bash
./bin/nigiri init
```

This creates a configuration file at `~/.nigiri/.nigiri.yml`.

2. Edit the configuration file to add your targets.

3. Build and run your targets as needed.

## Configuration

The configuration file is located at `~/.nigiri/.nigiri.yml`. `nigiri init` writes an annotated sample, [`pkg/commands/sample-config.yml`](./pkg/commands/sample-config.yml), to that location.

### Configuration Options

- `source`: Git repository URL
- `default-branch`: Default branch to use if no commit is specified
- `working-directory`: Subdirectory within the repository to run build commands (optional)
- `binary-only`: Whether to keep only the binary and remove source code after building (optional)
- `build-command`: OS-specific build commands
  - `linux`, `windows`, `darwin`: Build commands for each OS
  - `binary-path`: Path to the built binary, relative to `working-directory` (or to the repository root when `working-directory` is unset)
- `env`: Environment variables to set during build and run
- `defaults` (top level): build commands used by targets that define none of their own

## Commands

`nigiri --help` lists the commands; `nigiri <command> --help` shows each command's flags, their defaults, and usage examples.

A few rules that `--help` does not spell out:

- `build` accepts only a hexadecimal commit hash of 7 to 40 characters; branch and tag names are not accepted.
- `run`: when the second argument starts with `-`, it is passed to the target program instead of being read as a commit hash.
- `list` shows only targets that have been built; configured but unbuilt targets are not listed.

## Advanced Features

### Private Repositories

For private repositories, you need to provide authentication. Nigiri supports GitHub token authentication:

```bash
nigiri build <target> --use-token
```

The token is automatically sourced from, in order:
1. `GITHUB_TOKEN` environment variable
2. GitHub CLI (`gh auth token`)

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
