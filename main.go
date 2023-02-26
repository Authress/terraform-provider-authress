package main

import (
    "context"
    "terraform-provider-authress/src"

    "github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
    providerserver.Serve(context.Background(), authress.New, providerserver.ServeOpts{
        Address: "localhost.com/authress/authress",
    })
}