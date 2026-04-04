variable "secret_value" {
  type      = string
  ephemeral = true
}

resource "cloudflareext_workers_script_secret" "example" {
  script_name      = "my-worker-script"
  name             = "MY_SECRET"
  text_wo          = var.secret_value
  text_wo_version  = "1"
}
