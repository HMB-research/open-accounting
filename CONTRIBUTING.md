# Contributing to Open Accounting

Thank you for your interest in contributing to Open Accounting! This document provides guidelines and information for contributors.

## Development Status

This project is currently under active development. We appreciate your patience as we build out the core features and stabilize the APIs.

## Getting Started

1. **Fork the repository** and clone it locally
2. **Set up the development environment** following the README
3. **Create a branch** for your changes
4. **Make your changes** and test them thoroughly
5. **Submit a pull request**

## Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/open-accounting.git
cd open-accounting

# Start the development environment
make dev

# Run tests
make test

# Run linter
make lint
```

## Code Style

### Go
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Run `gofmt -w .` before committing
- Run `golangci-lint run` to check for issues
- Write tests for new functionality

### TypeScript/Svelte
- Follow existing code patterns
- Run `npm run check` in the frontend directory
- Use TypeScript for type safety

## Testing Requirements

### Test Coverage Standards

We maintain high test coverage to ensure code quality:

| Component | Target |
|-----------|--------|
| Backend packages | 90%+ average |
| Frontend | 95%+ |
| Critical paths | 95%+ |

### Running Tests

```bash
# Backend unit tests
go test -race -cover ./...

# Backend integration tests (requires DATABASE_URL)
go test -tags=integration -race -cover ./...

# Frontend tests
cd frontend && npm test

# Frontend with coverage
cd frontend && npm run test:coverage
```

### Writing Integration Tests

For database operations, write integration tests:

```go
//go:build integration

package mypackage

import (
    "testing"
    "github.com/HMB-research/open-accounting/internal/testutil"
)

func TestRepository_Create(t *testing.T) {
    pool := testutil.SetupTestDB(t)
    tenant := testutil.CreateTestTenant(t, pool)
    repo := NewPostgresRepository(pool)

    // Test implementation
}
```

### CI Checks

Pull requests must pass:
- `go test` - Unit tests
- `go test -tags=integration` - Integration tests
- `golangci-lint run` - Linting
- Codecov patch coverage threshold

## Pull Request Process

1. Update documentation if needed
2. Add tests for new functionality
3. Ensure all tests pass (`make test`)
4. Ensure linting passes (`make lint`)
5. Update the CHANGELOG.md if applicable
6. Request review from maintainers

## Reporting Issues

- Use the GitHub issue tracker
- Check if the issue already exists
- Provide detailed reproduction steps
- Include relevant logs or error messages

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow

## Questions?

Open an issue with the "question" label or start a discussion in the Discussions tab.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
