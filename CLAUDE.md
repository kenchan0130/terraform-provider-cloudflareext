# CLAUDE.md

## Project Overview

Terraform provider for Cloudflare resources that require write-only attribute support (Terraform 1.11+). Prevents secrets from being stored in Terraform state.

## Architecture

```
internal/
  provider/
    shared/          # Shared types: CloudflareClient, DoRequest[T], DoRequestNoBody
    provider.go      # Provider configuration (api_token, account_id, base_url)
  services/
    hyperdrive/      # cloudflareext_hyperdrive_config resource
    secretsstore/
      store/         # cloudflareext_secrets_store resource/data source
      secret/        # cloudflareext_secrets_store_secret resource
  testutil/          # Shared test helpers (provider factories, config helper)
```

- **Provider**: `internal/provider/provider.go` - Configuration, resource/data source registration
- **Shared**: `internal/provider/shared/client.go` - Generic Cloudflare API client with `DoRequest[T]()` / `DoRequestNoBody()`
- **Resources**:
  - `cloudflareext_hyperdrive_config` - Hyperdrive database proxy configs (`password`/`password_wo`)
  - `cloudflareext_secrets_store` - Secrets Store container (`name` is immutable, no update API)
  - `cloudflareext_secrets_store_secret` - Secrets Store secrets (`value`/`value_wo`)
- **Data Sources**:
  - `cloudflareext_secrets_store` - Look up a Secrets Store by name

## Development Commands

```bash
make build      # Build the provider binary
make test       # Run unit tests (go vet + go test with race detector)
make testacc    # Run acceptance tests (requires `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`)
make lint       # Run golangci-lint
make fmt        # Format code
make generate   # Generate docs via tfplugindocs
make install    # Install provider locally for testing
```

## Testing Patterns

- Unit tests use `resource.UnitTest()` with `httpmock` to intercept HTTP calls
- Shared test helpers in `internal/testutil/`: `ProtoV6ProviderFactories()`, `TestConfig()`, `CheckResourceAttr()`
- Tests use external test packages (`_test` suffix) to ensure clean API boundaries
- Each service test file has a `setup*Mock()` function that registers CRUD responders
- Mock responses use `shared.CloudflareResponse[T]` types
- Tests use `_wo` attributes (`password_wo`, `value_wo`) to avoid path expression issues with nested validators

## Key Conventions

- Go module: `github.com/kenchan0130/terraform-provider-cloudflareext`
- Provider type name: `cloudflareext`
- Services organized by Cloudflare API domain (`hyperdrive/`, `secretsstore/`)
- API types are unexported within each service package
- Only constructor functions are exported from service packages (`NewConfigResource`, `NewStoreResource`, etc.)
- Resources support both legacy sensitive attrs and write-only `_wo` attrs with `ExactlyOneOf` validators
- Use absolute paths (`path.MatchRoot(...)`) for cross-attribute validators in nested attributes
- Version managed in `version` file (read via `//go:embed`)
- GitHub Actions pinned to commit SHAs with `# vX.Y.Z` comments
- DCO sign-off required for contributions (`git commit -s`). All commits must have a `Signed-off-by` line — the DCO bot will reject PRs with unsigned commits
- License: Apache 2.0
- When modifying resource/data source schemas, always update the corresponding example files in `examples/` as well. These examples are used by `make generate` (tfplugindocs) to produce documentation
- Before creating a pull request, always run `make fmt`, `go generate ./...`, `make generate`, and `make test` to ensure no formatting, code generation, documentation generation, or test failures
