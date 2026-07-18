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
