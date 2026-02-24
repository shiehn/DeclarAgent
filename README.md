```
    ____            __              ___                    __
   / __ \___  _____/ /___ ______  /   | ____ ____  ____  / /_
  / / / / _ \/ ___/ / __ `/ ___/ / /| |/ __ `/ _ \/ __ \/ __/
 / /_/ /  __/ /__/ / /_/ / /    / ___ / /_/ /  __/ / / / /_
/_____/\___/\___/_/\__,_/_/    /_/  |_\__, /\___/_/ /_/\__/
                                     /____/
```

**Declarative runbook executor for AI agents.**
Validate, dry-run, and safely execute multi-step YAML workflows from any LLM tool-use loop.

---

## Why DeclarAgent?

LLM agents are great at reasoning but dangerous when executing. DeclarAgent gives agents a
**structured, auditable, and safe** way to run real CLI workflows:

- **YAML plans** — human-readable, version-controllable runbooks
- **Dry-run & explain** — inspect exactly what will happen before it does
- **Destructive-step gating** — steps marked `destructive: true` require explicit `--approve`
- **Structured JSON results** — every run returns machine-readable output with typed errors
- **Built-in actions** — file I/O, JSON manipulation, and env access without shell gymnastics
- **Template engine** — reference outputs from prior steps with `{{steps.<id>.outputs.<key>}}`
- **MCP server** — expose plans as tools over the Model Context Protocol

## Quick Start

```bash
# Install
go install github.com/stevehiehn/declaragent@latest

# Validate a plan
declaragent validate plan.yaml

# See what it would do
declaragent explain plan.yaml --input branch=main

# Dry-run (resolves templates, shows commands)
declaragent dry-run plan.yaml --input branch=main

# Execute for real
declaragent run plan.yaml --input branch=main

# Execute with destructive steps allowed
declaragent run plan.yaml --input branch=main --approve
```

## Plan Schema

Plans are YAML files with a simple structure:

```yaml
name: deploy-service
description: Build, test, and deploy the service
inputs:
  env:
    required: true
    description: Target environment
  tag:
    default: latest
steps:
  - id: test
    run: go test ./...
    outputs:
      result: stdout

  - id: build
    run: docker build -t myapp:{{inputs.tag}} .

  - id: write_manifest
    action: file.write
    params:
      path: manifest.txt
      content: "Deployed {{inputs.tag}} to {{inputs.env}}"

  - id: deploy
    run: kubectl apply -f k8s/{{inputs.env}}.yaml
    destructive: true
```

### Key Fields

| Field | Description |
|-------|-------------|
| `name` | Plan identifier |
| `inputs` | Named parameters with `required`, `description`, and `default` |
| `steps[].id` | Unique step identifier |
| `steps[].run` | Shell command to execute |
| `steps[].action` | Built-in action (alternative to `run`) |
| `steps[].params` | Parameters passed to built-in actions |
| `steps[].outputs` | Capture step output (e.g., `stdout`) |
| `steps[].destructive` | If `true`, blocked unless `--approve` is passed |

## CLI Commands

| Command | Description |
|---------|-------------|
| `validate <plan.yaml>` | Check plan structure and references |
| `explain <plan.yaml>` | Show resolved steps without executing |
| `dry-run <plan.yaml>` | Simulate execution, resolve templates |
| `run <plan.yaml>` | Execute the plan |
| `mcp` | Start MCP stdio server |

All commands accept `--json` for machine-readable output and `--input key=value` for plan inputs.

## Built-in Actions

| Action | Params | Description |
|--------|--------|-------------|
| `file.write` | `path`, `content` | Write content to a file |
| `file.append` | `path`, `content` | Append content to a file |
| `json.get` | `file`, `path` | Read a value from a JSON file |
| `json.set` | `file`, `path`, `value` | Set a value in a JSON file |
| `env.get` | `name` | Read an environment variable |

## Structured Results

Every execution returns a JSON result:

```json
{
  "run_id": "a1b2c3",
  "success": true,
  "steps": [
    {"id": "test", "status": "success", "exit_code": 0, "duration": "1.2s"}
  ],
  "outputs": {"test.result": "ok"},
  "artifacts": [".declaragent/a1b2c3/test/stdout"],
  "errors": []
}
```

Errors are typed for agent decision-making:

| Error Type | Retryable | Meaning |
|-----------|-----------|---------|
| `VALIDATION_ERROR` | No | Bad plan or missing inputs |
| `STEP_FAILED` | No | A command returned non-zero |
| `PERMISSION_DENIED` | No | Destructive step without `--approve` |
| `SIDE_EFFECT_BLOCKED` | No | Destructive step blocked in dry-run |
| `TRANSIENT` | Yes | Temporary failure, safe to retry |
| `TIMEOUT` | Yes | Step exceeded time limit |

## MCP Integration

Run `declaragent mcp` to start a [Model Context Protocol](https://modelcontextprotocol.io/) stdio server.
This exposes your plans as tools that any MCP-compatible agent can discover and invoke.

## License

See [LICENSE](LICENSE).
