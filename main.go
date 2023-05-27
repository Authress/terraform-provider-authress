package main

import (
	"context"

	authress "github.com/authress/terraform-provider-authress/src"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
    providerserver.Serve(context.Background(), authress.New, providerserver.ServeOpts{
        Address: "hashicorp.com/authress/authress",
    })
}