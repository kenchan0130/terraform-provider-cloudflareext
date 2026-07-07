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
make generate      # Run go generate ./...
make generate/docs # Generate docs via tfplugindocs (CI checks this)
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
- When modifying resource/data source schemas, always update the corresponding example files in `examples/` as well. These examples are used by `make generate` ([tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs)) to produce documentation. If a new resource or data source is added, create the necessary example files and templates following the tfplugindocs conventions
- Before creating a pull request, always run `make fmt`, `go generate ./...`, `make generate/docs`, and `make test` to ensure no formatting, code generation, documentation generation, or test failures. `make generate/docs` runs tfplugindocs to regenerate `docs/` from schemas and examples — CI will fail if generated docs are out of date

## Provider Bug Review Checklist

Bug classes found (and fixed) in past reviews of this codebase. When writing or reviewing resource code, check every item; when hunting for bugs, verify each suspect with a failing httpmock unit test before fixing (never report or fix on speculation alone). Cross-check implementation against the official OpenAPI schema at `github.com/cloudflare/api-schemas` (`openapi.json`), not just the SDK types.

- **404 on Read must remove state, not error.** If the resource was deleted out-of-band, `Read` must call `resp.State.RemoveResource(ctx)` so Terraform plans a re-create instead of failing. Use `shared.IsNotFoundError(err)` (detects `*cloudflare.Error` with `StatusCode == 404`). Resources that look up via List (e.g. secrets store, observability destination) must treat "not found in list" the same way.
- **Map every response field into state.** Any API response field that has a schema attribute (e.g. `scopes`, `comment`) must be written to state on Create/Read/Update, or remote drift is silently invisible. Guards like `if result.X != "" { set }` leave stale values in state when the remote value is cleared — on Read, map empty responses to `types.StringNull()`.
- **Never let a response zero-value overwrite a planned value the API doesn't echo.** Some API variants omit fields entirely (an access-protected Hyperdrive origin has no `port`); unconditionally assigning `types.Int64Value(result.Origin.Port)` writes `0` over the planned default and causes "Provider produced inconsistent result after apply". Only overwrite when the response actually carries a value, and on import fall back to the schema default.
- **PATCH semantics: omitted fields are retained by the API.** For PATCH-based updates (Secrets Store secret Edit), removing an optional attribute from config must explicitly send an empty value (`""`) to clear it server-side; simply not sending the field leaves the old value live and out of sync with config.
- **Create/Update must never write a value into state that contradicts a known plan value** — the framework errors on any mismatch between planned and applied state for non-computed attributes. When the API echoes a value the plan had as null, prefer the plan value (or clear it server-side, see above).
- **Provider `Configure` must check `IsUnknown()` before `IsNull()`** for every config attribute — an unknown value (reference to an unapplied resource) otherwise reads as `""` and produces a misleading "missing" error.
- **Write-only (`_wo`) attributes are absent from the plan**; read them from `req.Config` in Create/Update (`applyWriteOnlyAttributes` pattern) and pair each with a `_wo_version` trigger attribute.
- **Watch for value-format mismatches between docs and API** (hyphen vs underscore): logpush datasets are hyphenated (`opentelemetry-traces`), secrets store scope docs show both `ai_gateway` and `ai-gateway`. Normalize API responses before writing to state if the API's casing/format can differ from config.

Verified Cloudflare API facts (from the official OpenAPI schema, 2026-07):

- Hyperdrive: `PUT` is a full replace and requires the complete origin including `password`; `PATCH` is partial. Secrets (`password`, `access_client_secret`) are write-only and never returned. Access-protected origins have no `port`; public origins require it. Disabling caching discards custom caching settings server-side.
- Secrets Store: store has no update API (`name` is immutable → `RequiresReplace`). Secret create is a **bulk array** request/response; secret values can never be read back; update is PATCH (`value`/`scopes`/`comment` all optional, omitted = retained). List endpoints paginate with `page`/`per_page` + `result_info`.
- Workers script secrets: `PUT` upserts one secret; GET/List return only `{name, type}` (never the text).
- Workers Observability destinations: request/response config fields are **camelCase** (`logpushDataset`, `logpushJob`) except `destination_conf`; create returns **201**; `logpushDataset` and `name` cannot be patched (→ `RequiresReplace`); List (not Get) is the only read, and its error envelope entries may carry `message` without `code`.

Testing techniques for these bug classes: drift/404 tests use a `RefreshState: true` step with `RefreshPlanChecks` (e.g. `plancheck.ExpectResourceAction(..., ResourceActionCreate)` after a mocked 404); clearing-on-update tests assert on the captured PATCH request body inside the httpmock responder.
