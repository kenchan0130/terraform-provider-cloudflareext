data "cloudflareext_secrets_store" "example" {
  name = "my-secret-store"
}

output "store_id" {
  value = data.cloudflareext_secrets_store.example.id
}
