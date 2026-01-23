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
