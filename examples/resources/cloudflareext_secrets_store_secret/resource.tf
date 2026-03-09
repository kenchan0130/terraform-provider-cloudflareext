resource "cloudflareext_secrets_store_secret" "example" {
  store_id = cloudflareext_secrets_store.example.id
  name     = "MY_SECRET"
  value_wo = var.secret_value
  comment  = "Managed by Terraform"
  scopes   = ["workers"]
}
