package main

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider"
)

//go:embed version
var version string

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/kenchan0130/cloudflareext",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(strings.TrimSpace(version)), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
