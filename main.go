package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/mjlshen/mirrosa/pkg/mirrosa"
)

func main() {
	clusterId := flag.String("cluster-id", "", "Cluster ID")
	flag.Parse()

	if *clusterId == "" {
		panic("cluster id must not be empty")
	}

	mirrosa, err := mirrosa.NewClient(context.TODO(), *clusterId)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", mirrosa)
}
