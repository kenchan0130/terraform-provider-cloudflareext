terraform {
  required_providers {
    cloudflareext = {
      source = "kenchan0130/cloudflareext"
    }
  }
}

provider "cloudflareext" {
  api_token  = var.cloudflare_api_token
  account_id = var.cloudflare_account_id
}
