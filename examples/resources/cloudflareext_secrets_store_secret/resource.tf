resource "cloudflareext_secrets_store" "example" {
  name = "my-secret-store"
}

resource "cloudflareext_secrets_store_secret" "example" {
  store_id = cloudflareext_secrets_store.example.id
  name     = "MY_SECRET"
  comment  = "Managed by Terraform"
  scopes   = ["workers"]

  lifecycle {
    ignore_changes = [value]
  }
}
