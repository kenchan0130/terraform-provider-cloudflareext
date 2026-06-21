data "cloudflareext_workers_observability_destination" "example" {
  name = "workers-observability-example"
}

output "destination_slug" {
  value = data.cloudflareext_workers_observability_destination.example.slug
}
