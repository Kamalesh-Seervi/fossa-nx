# Contributing to FOSSA-NX

Thank you for your interest in contributing to FOSSA-NX! This document provides guidelines and instructions for contributing to this project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Environment](#development-environment)
- [Coding Guidelines](#coding-guidelines)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Release Process](#release-process)
- [Documentation](#documentation)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone. Please report any unacceptable behavior to the project maintainers.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork**:
   ```bash
   git clone https://github.com/YOUR-USERNAME/fossa-nx.git
   cd fossa-nx
   ```
3. **Set up the upstream remote**:
   ```bash
   git remote add upstream https://github.com/kamalesh-seervi/fossa-nx.git
   ```
4. **Create a new branch** for your feature or bugfix:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Environment

### Prerequisites

- Go 1.20 or later
- NX (for testing)
- FOSSA CLI
- Git

### Setup

1. Install required dependencies:
   ```bash
   go mod download
   ```

2. Build the project:
   ```bash
   go build -o fossa-nx cmd/fossa-nx/main.go
   ```

3. Run the CLI:
   ```bash
   ./fossa-nx --help
   ```

## Coding Guidelines

### Code Style

- Follow standard Go coding conventions
- Use `gofmt` or `goimports` to format your code
- Run linting before submitting your code:
  ```bash
  go vet ./...
  golint ./...
  ```

### Project Structure

- Keep the main application structure in the `cmd/fossa-nx` directory
- Place core functionality in the `internal` directory
- Tests should accompany all new code

### Commit Messages

- Use clear and meaningful commit messages
- Begin with a short (50 chars or less) summary line
- Follow with a blank line and then a more detailed explanation if necessary
- Reference issues and pull requests where appropriate

## Pull Request Process

1. **Update your fork** with the latest upstream changes:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all tests** to ensure your changes don't break existing functionality:
   ```bash
   go test ./...
   ```

3. **Create a pull request** against the `main` branch of the original repository
   - Clearly describe the problem and solution
   - Include the relevant issue number if applicable
   - Add details about any new dependencies

4. **Address review feedback** and update your PR as needed

5. **Wait for approval** from maintainers before merging

## Testing

- Write tests for all new functionality
- Ensure all tests pass before submitting a PR:
  ```bash
  go test ./...
  ```
- Include both unit tests and integration tests where appropriate
- For complex features, consider adding examples in the `/examples` directory

## Release Process

Releases are managed using GoReleaser and GitHub Actions:

1. Releases are triggered by creating a tag with a version number:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. GitHub Actions will automatically:
   - Build binaries for multiple platforms
   - Create a GitHub release
   - Update the Homebrew formula

3. Versioning follows [Semantic Versioning](https://semver.org/)

## Documentation

- Update documentation for any user-facing changes
- Include inline code comments for complex logic
- Consider updating the README.md for major features

## Questions?

If you have any questions or need help, please open an issue on GitHub or reach out to the maintainers directly.

Thank you for contributing to FOSSA-NX!
