package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mjlshen/mirrosa/pkg/mirrosa"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("unable to setup logger: %s", err)
	}
	defer logger.Sync()

	clusterId := flag.String("cluster-id", "", "Cluster ID")
	flag.Parse()

	if *clusterId == "" {
		panic("cluster id must not be empty")
	}

	mirrosa, err := mirrosa.NewRosaClient(context.TODO(), logger.Sugar(), *clusterId)
	if err != nil {
		panic(err)
	}

	if err := mirrosa.ValidateComponents(context.TODO(),
		mirrosa.NewVpc(),
		mirrosa.NewSecurityGroup(),
		mirrosa.NewVpcEndpointService(),
		mirrosa.NewPublicHostedZone(),
		mirrosa.NewPrivateHostedZone()); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s, \"Mirror mirror on the wall, who's the fairest of them all?\"\n%+v\n", mirrosa.ClusterInfo.Name, *mirrosa.ClusterInfo)
	os.Exit(0)
}
