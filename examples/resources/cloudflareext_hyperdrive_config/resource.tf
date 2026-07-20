variable "db_password" {
  type      = string
  ephemeral = true
}

resource "cloudflareext_hyperdrive_config" "example" {
  name = "my-hyperdrive"
  origin = {
    host                = "db.example.com"
    port                = 5432
    database            = "mydb"
    user                = "dbuser"
    password_wo         = var.db_password
    password_wo_version = "1"
    scheme              = "postgresql"
  }
  caching = {
    disabled               = false
    max_age                = 60
    stale_while_revalidate = 15
  }

  lifecycle {
    ignore_changes = [origin.password]
  }
}

# Query caching can be disabled entirely. Do not set max_age /
# stale_while_revalidate together with disabled = true — the Cloudflare API
# rejects that combination.
resource "cloudflareext_hyperdrive_config" "no_cache_example" {
  name = "my-hyperdrive-no-cache"
  origin = {
    host                = "db.example.com"
    port                = 5432
    database            = "mydb"
    user                = "dbuser"
    password_wo         = var.db_password
    password_wo_version = "1"
    scheme              = "postgresql"
  }
  caching = {
    disabled = true
  }
}

# Omitting the `caching` block entirely (as opposed to setting it explicitly)
# leaves the remote caching configuration unmanaged by Terraform: whatever is
# configured in Cloudflare is left as-is on create/update, and is tracked in
# (but not driven from) state. An explicit `caching = {}` block re-enables
# caching if it was disabled, but preserves the current max_age /
# stale_while_revalidate values; to reset those to Cloudflare's defaults, set
# them explicitly (max_age = 60, stale_while_revalidate = 15).
resource "cloudflareext_hyperdrive_config" "unmanaged_caching_example" {
  name = "my-hyperdrive-unmanaged-caching"
  origin = {
    host                = "db.example.com"
    port                = 5432
    database            = "mydb"
    user                = "dbuser"
    password_wo         = var.db_password
    password_wo_version = "1"
    scheme              = "postgresql"
  }
}

# Omitting the `mtls` block leaves any remote mTLS configuration unmanaged by
# Terraform and preserves it during full-replace updates. To explicitly remove
# the remote mTLS configuration, set an empty block:
#
#   mtls = {}
