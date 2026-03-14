variable "secret_value" {
  type      = string
  ephemeral = true
}

resource "cloudflareext_secrets_store" "example" {
  name = "my-secret-store"
}

resource "cloudflareext_secrets_store_secret" "example" {
  store_id         = cloudflareext_secrets_store.example.id
  name             = "MY_SECRET"
  value_wo         = var.secret_value
  value_wo_version = "1"
  comment          = "Managed by Terraform"
  scopes           = ["workers"]

  lifecycle {
    ignore_changes = [value]
  }
}
