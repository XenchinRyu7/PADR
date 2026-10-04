# Contributing to PADR

Thank you for your interest in contributing to **PADR (Personal Autonomous Development Runner)**! We welcome contributions of all kinds: bug reports, documentation improvements, feature ideas, and pull requests.

---

## 🧭 Code of Conduct

All contributors are expected to uphold our [Code of Conduct](CODE_OF_CONDUCT.md) in all project spaces.

---

## 🛠️ Development Setup

### Prerequisites
- **Go**: Version `1.22+` (1.25 recommended)
- **Git**: Installed and accessible in your `$PATH`
- **OpenCode CLI** (optional for local mock testing, required for end-to-end LLM agent runs)

### Steps
1. Fork and clone the repository:
   ```bash
   git clone https://github.com/<your-username>/PADR.git
   cd PADR
   ```
2. Download Go module dependencies:
   ```bash
   go mod download
   ```
3. Run the unit and integration test suite:
   ```bash
   go test -v ./...
   ```
4. Build the binary locally:
   ```bash
   go build -o bin/padr ./cmd/padr
   ```

---

## 📐 Project Structure

```
├── cmd/
│   └── padr/           # Cobra CLI entry point and subcommands
├── pkg/
│   ├── agent/          # AgentEngine abstraction & OpenCode CLI runner
│   ├── config/         # Global & project YAML configurations
│   ├── git/            # Git safety checks, clean verification, commits
│   ├── router/         # Model swapping, fallback chain, budget guard
│   ├── runner/         # Workflow orchestrator connecting git, agent, tests
│   ├── scheduler/      # OS Task Scheduler integration (Windows / Unix)
│   └── store/          # Pure-Go SQLite state and execution log store
└── .github/            # Workflows, issue templates, and funding
```

---

## 📝 Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat(...)`: A new feature
- `fix(...)`: A bug fix
- `docs(...)`: Documentation changes
- `refactor(...)`: Code changes that neither fix bugs nor add features
- `test(...)`: Adding or correcting tests
- `chore(...)`: Maintenance tasks, dependencies, tooling

Example:
```bash
feat(router): add adaptive backoff on 429 rate limit
fix(git): resolve branch checkout issue when tracking remote
docs: update architecture diagram in README
```

---

## 🚀 Submitting a Pull Request

1. Create a feature branch:
   ```bash
   git checkout -b feat/your-feature-name
   ```
2. Make your changes with focused, meaningful commits.
3. Ensure all tests pass and code compiles:
   ```bash
   go test -v ./...
   ```
4. Push your branch to GitHub:
   ```bash
   git push origin feat/your-feature-name
   ```
5. Open a Pull Request against the `main` branch with a clear description of the problem solved.
