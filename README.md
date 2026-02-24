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
**structured, auditable, and safe** way to run real workflows:

- **YAML plans** — human-readable, version-controllable runbooks
- **Three step types** — shell commands (`run`), built-in actions (`action`), and HTTP requests (`http`)
- **Dry-run & explain** — inspect exactly what will happen before it does
- **Destructive-step gating** — steps marked `destructive: true` require explicit `--approve`
- **Structured JSON results** — every run returns machine-readable output with typed errors
- **Built-in actions** — file I/O, JSON manipulation, and env access without shell gymnastics
- **Template engine** — reference outputs from prior steps with `{{steps.<id>.outputs.<key>}}`
- **MCP server** — expose plans as directly callable tools over the Model Context Protocol
- **Plan-as-tool** — drop YAML files in a directory, each becomes an MCP tool automatically

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

# Start MCP server with a plans directory
declaragent mcp --plans ./plans
```

## Plan Schema

Plans are YAML files with a simple structure. Each step does exactly one thing: run a shell command, call a built-in action, or send an HTTP request.

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

  - id: check_health
    http:
      url: "https://{{inputs.env}}.example.com/health"
      method: GET
    outputs:
      status: stdout

  - id: deploy
    run: kubectl apply -f k8s/{{inputs.env}}.yaml
    destructive: true
```

### Step Types

| Type | Field | Description |
|------|-------|-------------|
| Shell | `run` | Runs a shell command via `sh -c`. Captures stdout/stderr. |
| Action | `action` | Calls a built-in action (file I/O, JSON, env). |
| HTTP | `http` | Sends an HTTP request. Response body captured as `stdout`. |

Each step must have **exactly one** of `run`, `action`, or `http`.

### HTTP Step Fields

```yaml
- id: call_api
  http:
    url: "https://api.example.com/data"    # required
    method: POST                            # default: GET
    headers:
      Authorization: "Bearer {{inputs.token}}"
    body: '{"key": "value"}'               # template-resolved string
  outputs:
    response: stdout                        # response body
```

### Key Fields

| Field | Description |
|-------|-------------|
| `name` | Plan identifier |
| `inputs` | Named parameters with `required`, `description`, and `default` |
| `steps[].id` | Unique step identifier |
| `steps[].run` | Shell command to execute |
| `steps[].action` | Built-in action (alternative to `run`) |
| `steps[].http` | HTTP request (alternative to `run` and `action`) |
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
| `mcp [--plans DIR]` | Start MCP stdio server |

All commands accept `--json` for machine-readable output and `--input key=value` for plan inputs.

## Built-in Actions

| Action | Params | Description |
|--------|--------|-------------|
| `file.write` | `path`, `content` | Write content to a file |
| `file.append` | `path`, `content` | Append content to a file |
| `json.get` | `file`, `path` | Read a value from a JSON file |
| `json.set` | `file`, `path`, `value` | Set a value in a JSON file |
| `env.get` | `name` | Read an environment variable |
| `http` | `url`, `method`, `body`, `header_*` | Send an HTTP request |

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

### Plan-as-Tool

When you pass `--plans <directory>`, every YAML plan in that directory becomes a **directly callable MCP tool**:

```bash
declaragent mcp --plans ./plans
```

- Tool `name` = plan `name`
- Tool `description` = plan `description`
- Tool `inputSchema` = derived from plan `inputs`
- Calling the tool = executing the plan with the provided inputs

This means an LLM agent can discover and invoke your plans without knowing anything about DeclarAgent's internal plan format.

### MCP Configuration

Add to your Claude Code config (`.claude/settings.json`):

```json
{
  "mcpServers": {
    "declaragent": {
      "command": "/path/to/declaragent",
      "args": ["mcp", "--plans", "/path/to/your/plans"]
    }
  }
}
```

### Built-in MCP Tools

These meta-tools are always available regardless of `--plans`:

| Tool | Description |
|------|-------------|
| `plan.validate` | Validate a plan YAML file |
| `plan.explain` | Explain a plan without executing |
| `plan.dry_run` | Dry-run a plan |
| `plan.run` | Execute a plan |
| `plan.schema` | Return the plan YAML schema |

---

## Docs: 3 Example Plans You Can Copy and Run

Copy these into a `plans/` directory and start the MCP server:

```bash
mkdir -p plans
# (paste each YAML below into the corresponding file)
declaragent mcp --plans ./plans
```

Or run any plan directly:

```bash
declaragent run plans/hello.yaml --input name=Alice
```

### Plan 1: `plans/hello.yaml` — Simple Shell Command

The simplest possible plan. Runs `echo` and captures the output.

```yaml
name: hello
description: Say hello to someone
inputs:
  name:
    description: Who to greet
    default: World
steps:
  - id: greet
    run: echo "Hello, {{inputs.name}}!"
    outputs:
      message: stdout
```

**Run it:**
```bash
declaragent run plans/hello.yaml --input name=Alice
```

**Result:**
```
Plan "hello" completed successfully.
Run ID: <uuid>
```

With `--json`:
```json
{
  "run_id": "...",
  "success": true,
  "steps": [{"id": "greet", "status": "success", "stdout_ref": "Hello, Alice!\n"}]
}
```

### Plan 2: `plans/ip_lookup.yaml` — HTTP Request

Fetches your public IP address from a free API.

```yaml
name: ip-lookup
description: Look up your public IP address
steps:
  - id: fetch_ip
    http:
      url: "https://httpbin.org/ip"
      method: GET
    outputs:
      response: stdout
```

**Run it:**
```bash
declaragent run plans/ip_lookup.yaml --json
```

**Result:**
```json
{
  "success": true,
  "steps": [{
    "id": "fetch_ip",
    "status": "success",
    "stdout_ref": "{\"origin\": \"203.0.113.42\"}"
  }]
}
```

### Plan 3: `plans/build_and_notify.yaml` — Multi-Step with Chaining

Runs a build, then POSTs the result to a webhook. Demonstrates step chaining — the HTTP step uses the shell step's output via `{{steps.build.outputs.result}}`.

```yaml
name: build-and-notify
description: Run a build command and POST the result to a webhook
inputs:
  webhook_url:
    required: true
    description: URL to POST the build result to
steps:
  - id: build
    run: echo "build-ok"
    outputs:
      result: stdout

  - id: notify
    http:
      url: "{{inputs.webhook_url}}"
      method: POST
      headers:
        Content-Type: application/json
      body: '{"event": "build_complete", "output": "{{steps.build.outputs.result}}"}'
    outputs:
      response: stdout
```

**Run it:**
```bash
declaragent run plans/build_and_notify.yaml \
  --input webhook_url=https://httpbin.org/post --json
```

This runs `echo "build-ok"`, captures the output, then POSTs it to the webhook URL.

---

## Testing

```bash
go test ./...
```

## License

See [LICENSE](LICENSE).
