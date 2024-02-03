package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mjlshen/mirrosa/pkg/mirrosa"
	"github.com/mjlshen/mirrosa/pkg/tui"
)

func main() {
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	clusterId := f.String("cluster-id", "", "OCM internal or external cluster id")
	interactive := f.Bool("i", false, "run in an interactive exploratory mode")
	verbose := f.Bool("v", false, "enable verbose logging")
	awsProxy := f.String("aws-proxy", "", "[optional] proxy to use for aws requests")
	f.Parse(os.Args[1:])

	opts := slog.HandlerOptions{}
	if *verbose {
		opts.AddSource = true
		opts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &opts))

	if info, ok := debug.ReadBuildInfo(); ok {
		logger.Debug(fmt.Sprintf("Go Version: %s", info.GoVersion))
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				logger.Debug(fmt.Sprintf("Git SHA: %s", setting.Value))
			}
			if setting.Key == "vcs.time" {
				logger.Debug(fmt.Sprintf("From: %s", setting.Value))
			}
		}
	}

	if *interactive {
		p := tea.NewProgram(tui.InitModel())
		if _, err := p.Run(); err != nil {
			logger.Error(err.Error())
		}
		os.Exit(0)
	}

	if *clusterId == "" {
		logger.Error("cluster id must not be empty")
		os.Exit(1)
	}

	m, err := mirrosa.NewRosaClient(context.Background(), logger, *clusterId, *awsProxy)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	logger.Debug("cluster info from OCM", "cluster info", *m.ClusterInfo)
	logger.Info("who's the fairest of them all", "cluster", m.ClusterInfo.Name)

	if err := m.ValidateComponents(context.TODO(),
		m.NewVpc(),
		m.NewDhcpOptions(),
		m.NewSecurityGroup(),
		m.NewVpcEndpointService(),
		m.NewPublicHostedZone(),
		m.NewPrivateHostedZone(),
		m.NewApiLoadBalancer(),
		m.NewInstances(),
	); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("%s is the fairest of them all!", m.ClusterInfo.Name))
	os.Exit(0)
}
