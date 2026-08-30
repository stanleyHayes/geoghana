package ghanageo_test

import (
	"context"
	"fmt"
	"log"

	ghanageo "github.com/ghanageo/ghanageo-go"
)

func ExampleClient_Regions() {
	client, err := ghanageo.New()
	if err != nil {
		log.Fatal(err)
	}
	_ = client
	// Network calls take context.Context first and are anonymous by default.
	// page, err := client.Regions(context.Background(), ghanageo.PageOptions{Limit: 16})
	fmt.Println(ghanageo.APIVersion)
	// Output: v1
	_ = context.Background()
}
