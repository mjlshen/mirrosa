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

	fmt.Printf("%s, \"Mirror mirror on the wall, who's the fairest of them all?\"\n%+v\n", mirrosa.ClusterInfo.Name, *mirrosa.ClusterInfo)
}
