# maestro-cli

CLI for Maestro ManifestWork lifecycle management.

## Installation

```bash
make build
# Binary: bin/maestro-cli
```

### Building with Version Information

The build system automatically injects version information using ldflags:

```bash
# Default build (uses git info)
make build

# Override version/commit
make build VERSION=1.2.3 GIT_COMMIT=abc123

# Manual build with ldflags
go build -ldflags="-X github.com/openshift-hyperfleet/maestro-cli/cmd.Version=1.2.3 \
                   -X github.com/openshift-hyperfleet/maestro-cli/cmd.Commit=abc123 \
                   -X github.com/openshift-hyperfleet/maestro-cli/cmd.Date=2024-01-01T10:00:00Z" \
  ./cmd/maestro-cli

# Check version info
./bin/maestro-cli version
```

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `MAESTRO_GRPC_ENDPOINT` | gRPC server address | `localhost:8090` |
| `MAESTRO_HTTP_ENDPOINT` | HTTP API endpoint | `http://localhost:8000` |
| `MAESTRO_SOURCE_ID` | Source ID for CloudEvents | `maestro-cli` |

## Global Flags

```text
--grpc-endpoint string       Maestro gRPC server address
--http-endpoint string       Maestro HTTP API endpoint
--grpc-insecure              Skip TLS verification
--timeout duration           Operation timeout (default: 5m)
--output string              Output format: yaml, json (default: yaml)
--results-path string        Path to write results for status-reporter
--verbose                    Enable debug logging
```

## Commands

### apply

Apply a ManifestWork to a target cluster.

```bash
# Apply without waiting
maestro-cli apply --manifest-file=manifest.yaml --consumer=agent1

# Apply and wait for Available condition (default)
maestro-cli apply --manifest-file=manifest.yaml --consumer=agent1 --wait

# Apply and wait for specific condition
maestro-cli apply --manifest-file=job.yaml --consumer=agent1 --wait="Job:Complete"

# Apply and wait for complex condition
maestro-cli apply --manifest-file=job.yaml --consumer=agent1 \
  --wait="Job:Complete OR Job:Failed" --timeout=10m
```

### delete

Delete a ManifestWork.

```bash
# Delete a ManifestWork
maestro-cli delete --name=my-manifestwork --consumer=agent1

# Delete and wait for completion
maestro-cli delete --name=my-manifestwork --consumer=agent1 --wait

# Dry run
maestro-cli delete --name=my-manifestwork --consumer=agent1 --dry-run
```

### list

List all ManifestWorks for a consumer.

```bash
# List all ManifestWorks
maestro-cli list --consumer=agent1

# Filter by manifest content
maestro-cli list --consumer=agent1 --filter=nginx
maestro-cli list --consumer=agent1 --filter=Deployment/default/nginx

# Output as JSON
maestro-cli list --consumer=agent1 --output=json
```

### describe

Show detailed information about a ManifestWork.

```bash
maestro-cli describe --name=my-manifestwork --consumer=agent1
```

### get

Get a ManifestWork definition.

```bash
# Get as YAML
maestro-cli get --name=my-manifestwork --consumer=agent1

# Get as JSON
maestro-cli get --name=my-manifestwork --consumer=agent1 --output=json
```

### wait

Wait for a ManifestWork to reach a condition (like `kubectl wait`).

```bash
# Wait for Available condition (default)
maestro-cli wait --name=my-manifestwork --consumer=agent1

# Wait for specific condition
maestro-cli wait --name=my-job --consumer=agent1 --for="Job:Complete"

# Wait with timeout
maestro-cli wait --name=my-job --consumer=agent1 \
  --for="Job:Complete OR Job:Failed" --timeout=10m
```

### watch

Continuously stream ManifestWork status changes (like `kubectl get --watch`).

```bash
# Watch status changes
maestro-cli watch --name=my-manifestwork --consumer=agent1

# Watch with custom poll interval
maestro-cli watch --name=my-manifestwork --consumer=agent1 --poll-interval=5s
```

### validate

Validate a ManifestWork file without applying.

```bash
maestro-cli validate --manifest-file=manifest.yaml
```

### diff

Compare local ManifestWork with remote state.

```bash
maestro-cli diff --manifest-file=manifest.yaml --consumer=agent1
```

### tui

Launch an interactive terminal UI to browse consumers and ManifestWorks.

```bash
# Launch with default endpoint
maestro-cli tui

# Launch pointing at a specific Maestro instance
maestro-cli tui --http-endpoint=http://maestro.example.com:8000

# Launch with a bearer token
maestro-cli tui --http-endpoint=http://maestro.example.com:8000 \
  --grpc-client-token=<token>
```

#### Layout

```
┌─ Consumers ──────────────┐┌─ ManifestWork Detail ─────────────────────────┐
│ > consumer-1             ││ Name:        my-work                           │
│   consumer-2             ││ Consumer:    consumer-1   Version: 3           │
└──────────────────────────┘│ Created:     2024-01-01T00:00:00Z              │
┌─ ManifestWorks ──────────┐│                                                │
│ [/] to filter            ││ Conditions:                                    │
│ > work-1  ✓              ││   ✓ Applied   ✓ Available                      │
│   work-2  ✗              ││                                                │
│   work-3  ?              ││ Manifests (2):                                 │
└──────────────────────────┘│   • Deployment/my-app (default)                │
                            │   • Service/my-svc (default)                   │
                            └────────────────────────────────────────────────┘
[Tab] panel  [n] new  [d] del  [w] watch  [/] filter  [↑↓] nav  [Ctrl+C] quit
```

#### Key bindings

| Context | Key | Action |
|---------|-----|--------|
| Global | `Tab` / `Shift+Tab` | Cycle focus between panels |
| Global | `Ctrl+C` | Quit |
| Consumers | `↑` / `↓` or `k` / `j` | Navigate list |
| Consumers | `Enter` | Load ManifestWorks for selected consumer |
| Consumers | `n` | Create new consumer |
| Consumers | `d` | Delete selected consumer (confirm prompt) |
| Consumers | `r` | Refresh consumer list |
| ManifestWorks | `↑` / `↓` or `k` / `j` | Navigate list |
| ManifestWorks | `/` | Filter by name |
| ManifestWorks | `Esc` | Clear filter |
| ManifestWorks | `w` | Toggle watch mode (auto-refresh every 5 s) |
| ManifestWorks | `v` | Cycle detail view: Formatted → JSON → YAML |
| ManifestWorks | `d` | Delete selected ManifestWork (confirm prompt) |
| ManifestWorks | `r` | Refresh list |
| ManifestWorks | `y` | Copy detail to clipboard |
| Detail | `↑` / `↓` / `PgUp` / `PgDn` | Scroll |
| Detail | `/` | Open inline search |
| Detail | `Enter` / `n` | Next search match |
| Detail | `N` | Previous search match |
| Detail | `Esc` | Close search |
| Detail | `w` | Toggle watch mode |
| Detail | `v` | Cycle view mode |
| Detail | `y` | Copy to clipboard |
| Detail | `r` | Refresh |

#### Features

- **Three view modes** — Formatted (human-readable), JSON, and YAML with syntax highlighting.
- **Inline search** — Press `/` in the detail panel to search; matches are highlighted in amber, the current match in green. `n`/`N` cycle through occurrences.
- **Watch mode** — Press `w` to auto-refresh the selected ManifestWork every 5 seconds. An amber `[WATCH]` badge appears in the panel title.
- **Filter** — Press `/` in the ManifestWorks panel to filter by name in real time.
- **Clipboard** — Press `y` to copy the current detail view to the system clipboard (plain text, no ANSI codes).
- **Mouse support** — Click to focus a panel or select an item; scroll wheel navigates lists and scrolls the detail viewport.

## Condition Expressions

The `--wait` and `--for` flags support condition expressions:

```bash
# ManifestWork-level conditions
--wait="Available"
--wait="Applied"

# Resource-specific conditions (from statusFeedback)
--wait="Job:Complete"
--wait="Job/test-job-1:Failed"

# Logical expressions
--wait="Job:Complete OR Job:Failed"
--wait="Available AND Job:Complete"
```

## Examples

```bash
# Full workflow: apply, wait, then check status
maestro-cli apply --manifest-file=job.yaml --consumer=agent1 \
  --wait="Job:Complete OR Job:Failed" \
  --results-path=/tmp/result.json \
  --timeout=10m

# Check result
cat /tmp/result.json
```

## License

Apache License 2.0
