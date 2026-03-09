resource "cloudflareext_hyperdrive_config" "example" {
  name = "my-hyperdrive"
  origin = {
    host        = "db.example.com"
    port        = 5432
    database    = "mydb"
    user        = "dbuser"
    password_wo = var.db_password
    scheme      = "postgresql"
  }
  caching = {
    disabled               = false
    max_age                = 60
    stale_while_revalidate = 15
  }
}
