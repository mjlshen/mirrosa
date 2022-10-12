package main

import (
	"context"
	"fmt"

	"github.com/mjlshen/mirrosa/pkg/rosa"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

func main() {
	// clusterId := flag.String("cluster-id", "", "Cluster ID")
	// flag.Parse()

	// ocmClient, err := ocm.CreateConnection()
	// if err != nil {
	// 	panic(err)
	// }
	// defer ocmClient.Close()

	// ocmClient.GetCluster

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	r := rosa.RosaClient{
		Ec2Client:     ec2.NewFromConfig(cfg),
		Route53Client: route53.NewFromConfig(cfg),
	}

	if err := r.ValidateVpcAttributes(context.TODO(), "vpc-12345"); err != nil {
		panic(err)
	}

	fmt.Println(r)
}
