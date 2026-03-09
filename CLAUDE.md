# CLAUDE.md

## Project Overview

Terraform provider for Cloudflare resources that require write-only attribute support (Terraform 1.11+). Prevents secrets from being stored in Terraform state.

## Architecture

- **Provider**: `internal/provider/provider.go` - Configuration (api_token, account_id, base_url)
- **Resources**:
  - `cloudflareext_hyperdrive_config` - Hyperdrive database proxy configs (password/password_wo)
  - `cloudflareext_secrets_store_secret` - Secrets Store secrets (value/value_wo)
- **Ephemeral Resources**:
  - `cloudflareext_secrets_store_secret` - Read-only secret metadata (never stored in state)
- **API Layer**: `internal/provider/api.go` - Generic Cloudflare API client with `doRequest[T]()` / `doRequestNoBody()`
- **Tests**: `internal/provider/*_test.go` - Unit tests using `jarcoal/httpmock` for HTTP mocking

## Development Commands

```bash
make build      # Build the provider binary
make test       # Run unit tests (go vet + go test with race detector)
make testacc    # Run acceptance tests (requires CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID)
make lint       # Run golangci-lint
make fmt        # Format code
make generate   # Generate docs via tfplugindocs
make install    # Install provider locally for testing
```

## Testing Patterns

- Unit tests use `resource.UnitTest()` with `httpmock` to intercept HTTP calls
- Provider factories: `testUnitTestProtoV6ProviderFactories()` in `provider_test.go`
- Test config helper: `testUnitTestConfig(hcl)` prepends provider block
- Each resource test file has a `setup*Mock()` function that registers CRUD responders
- Mock responses use the same `cloudflareResponse[T]` types as production code
- Tests use `_wo` attributes (password_wo, value_wo) to avoid path expression issues with nested validators

## Key Conventions

- Go module: `github.com/kenchan0130/terraform-provider-cloudflareext`
- Provider type name: `cloudflareext`
- All API types prefixed with `api` (e.g., `apiHyperdriveCreateRequest`)
- Resources support both legacy sensitive attrs and write-only `_wo` attrs with `ExactlyOneOf` validators
- Use absolute paths (`path.MatchRoot(...)`) for cross-attribute validators in nested attributes
- Version managed in `version` file (read via `//go:embed`)
- GitHub Actions pinned to commit SHAs with `# vX.Y.Z` comments
- DCO sign-off required for contributions (`git commit -s`)
- License: Apache 2.0
