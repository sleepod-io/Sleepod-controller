# Contributing to SleePod-controller

Thank you for your interest in contributing to SleePod-controller! We welcome contributions from everyone.

## Getting Started

### Prerequisites

- Go (v1.24+)
- Docker
- Kubernetes Cluster (Kind, Minikube, or remote)
- Kubectl

### Local Development Setup

1. **Fork and Clone**
   Fork the repository and clone it locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/SleePod-controller.git
   cd SleePod-controller
   ```

2. **Run Tests**
   Ensure everything is working correctly by running the tests:
   ```bash
   make test
   ```

3. **Run Locally**
   You can run the controller locally against your configured Kubernetes cluster:
   ```bash
   make run
   ```

## Workflow

1.  Create a new branch for your feature or fix: `git checkout -b feature/my-new-feature`
2.  Make your changes.
3.  Add tests for your changes.
4.  Ensure all tests pass: `make test`
5.  Commit your changes following [Conventional Commits](https://www.conventionalcommits.org/).
6.  Push to your fork and submit a Pull Request.

## Coding Standards

- Follow idiomatic Go practices.
- Ensure `go fmt` and `go vet` run without issues (run `make fmt` and `make vet`).
- Add comments for exported functions and complex logic.

## Reporting Issues

If you find a bug or have a feature request, please search existing issues to see if it has already been reported. If not, please open a new issue using the provided templates.
