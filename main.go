package main

import (
	"context"
	"flag"
	"log"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mjlshen/mirrosa/pkg/mirrosa"
	"github.com/mjlshen/mirrosa/pkg/tui"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	clusterId := f.String("cluster-id", "", "OCM internal or external cluster id")
	interactive := f.Bool("i", false, "run in an interactive exploratory mode")
	verbose := f.Bool("v", false, "enable verbose logging")
	f.Parse(os.Args[1:])

	cfg := zap.NewDevelopmentConfig()
	if !*verbose {
		cfg.Level.SetLevel(zapcore.InfoLevel)
	}

	logger, err := cfg.Build()
	if err != nil {
		log.Fatalf("unable to setup logger: %s", err)
	}
	defer logger.Sync()
	sugared := logger.Sugar()

	if info, ok := debug.ReadBuildInfo(); ok {
		sugared.Debugf("Go Version: %s", info.GoVersion)
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				sugared.Debugf("Git SHA: %s", setting.Value)
			}
			if setting.Key == "vcs.time" {
				sugared.Debugf("From: %s", setting.Value)
			}
		}
	}

	if *interactive {
		p := tea.NewProgram(tui.InitModel())
		if _, err := p.Run(); err != nil {
			sugared.Fatal(err)
		}
		os.Exit(0)
	}

	if *clusterId == "" {
		sugared.Fatal("cluster id must not be empty")
	}

	mirrosa, err := mirrosa.NewRosaClient(context.TODO(), sugared, *clusterId)
	if err != nil {
		sugared.Fatal(err)
	}

	sugared.Debugf("cluster info from OCM: %+v", *mirrosa.ClusterInfo)
	sugared.Infof("%s: \"Mirror mirror on the wall, who's the fairest of them all?\"", mirrosa.ClusterInfo.Name)

	if err := mirrosa.ValidateComponents(context.TODO(),
		mirrosa.NewVpc(),
		mirrosa.NewDhcpOptions(),
		mirrosa.NewSecurityGroup(),
		mirrosa.NewVpcEndpointService(),
		mirrosa.NewPublicHostedZone(),
		mirrosa.NewPrivateHostedZone(),
		mirrosa.NewApiLoadBalancer(),
		mirrosa.NewInstances(),
	); err != nil {
		sugared.Error(err)
		os.Exit(1)
	}

	sugared.Infof("mirrosa: \"%s is the fairest of them all!\"", mirrosa.ClusterInfo.Name)
	os.Exit(0)
}
