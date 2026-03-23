# Smith CLI Installation and Usage

`smith` is the primary command-line interface for managing Smith autonomous development loops.

`smith` is the single CLI entrypoint for control-plane and runtime workflows.

## Installation

### From Source

If you have Go installed, you can install `smith` directly from the repository:

```bash
go install ./cmd/smith
```

Alternatively, you can use the provided `Makefile` to build binaries:

```bash
make build
```

This places the binaries in the `bin/` directory.

### Downloading Binaries

In the future, pre-built binaries for various platforms (Linux, macOS, Windows) will be available for download from the [GitHub Releases](https://github.com/smith-org/smith/releases) page.

Current repository for release artifacts: [https://github.com/callmeradical/smith/releases](https://github.com/callmeradical/smith/releases).

## Configuration

`smith` uses a configuration file located at `~/.smith/config.json` by default. You can also use environment variables or command-line flags to configure it.

### Config File Format

```json
{
  "current_context": "default",
  "contexts": {
    "default": {
      "server": "http://127.0.0.1:8080",
      "token": "your-operator-token"
    }
  }
}
```

### Environment Variables

- `SMITH_API_URL`: The URL of the Smith API server (e.g., `http://localhost:8080`).
- `SMITH_OPERATOR_TOKEN`: Your operator bearer token.
- `SMITH_CONTEXT`: The named configuration context to use.

## Usage

### Global Flags

- `--server URL`: Smith API server URL.
- `--token TOKEN`: Operator bearer token.
- `--config PATH`: Path to smith config file (default: `~/.smith/config.json`).
- `--context NAME`: Named config context to use.
- `--output text|json`: Output format (default: `text`).

### Commands

Command taxonomy:

- Preferred form: `smith <resource> <command>`
- Alias form: `smith ctl <resource> <command>`

`smith ctl` is a compatibility alias. Use top-level resources for docs, scripts, and examples.

Provider/project onboarding sequence:

1. Configure provider profiles first
2. Configure projects that bind to those provider profiles
3. Start/create loops

`smith` enforces this ordering for `project add` and `loop create` by checking onboarding readiness. If provider prerequisites are missing, responses include ordered missing requirements and a suggested next command.

#### Managing Provider Profiles

- **List provider profiles:**
  ```bash
  smith provider list
  ```

- **Add a provider profile:**
  ```bash
  smith provider add --id codex-default --type codex --default-model gpt-5-codex --credential-id codex-default-key --api-key "$SMITH_CODEX_API_KEY"
  ```

- **Configure an existing provider profile:**
  ```bash
  smith provider configure codex-default --credential-id openai-key --api-key "$SMITH_CODEX_API_KEY"
  ```

Supported provider types: `codex`, `claude`, `gemini`.

#### Managing Projects

- **List projects:**
  ```bash
  smith project list
  ```

- **Add a project:**
  ```bash
  smith project add --id smith --repo-url https://github.com/acme/smith --provider-profile-id codex-default
  ```

- **Configure an existing project:**
  ```bash
  smith project configure smith --github-user octocat
  ```

#### Managing Loops

- **List loops:**
  ```bash
  smith loop list
  ```

- **Get loop details:**
  ```bash
  smith loop get <loop-id>
  ```

- **View loop journal (logs):**
  ```bash
  smith loop logs <loop-id>
  ```
  Use `--follow` to stream live journal entries.

- **Create a loop from a GitHub issue:**
  ```bash
  smith loop create --title "Fix bug" --source-type github_issue --source-ref "org/repo#123"
  ```

- **Create a loop from a PRD:**
  ```bash
  smith loop create --from-prd docs/prd1.md
  ```

- **Cancel a loop:**
  ```bash
  smith loop cancel <loop-id> --reason "User requested"
  ```

> **Notice (Gated / Under Development):** Task Contract UI/API surfaces (`/tasks`, `/v1/tasks`) are feature-flagged and may be disabled by default depending on runtime configuration.

Task contract management currently ships through API/Console (`/v1/tasks`, `/tasks` route). `smith` does not yet expose a dedicated `task` resource command group.

#### Interactive Control

- **Attach to a running loop:**
  ```bash
  smith loop attach <loop-id>
  ```

- **Execute a command in a loop:**
  ```bash
  smith loop command <loop-id> --command "ls -la"
  ```

- **Detach from a loop:**
  ```bash
  smith loop detach <loop-id>
  ```

#### Managing PRDs

- **Create a PRD template:**
  ```bash
  smith prd create "New Feature" --template feature --out docs/feature.md
  ```

- **Submit a PRD to trigger loops:**
  ```bash
  smith prd submit --file docs/feature.md
  ```

## Examples

For more examples and detailed API documentation, refer to [docs/loop-ingress-and-cli.md](loop-ingress-and-cli.md).
