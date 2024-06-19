// cloudstack-csi-driver binary.
//
// To get usage information:
//
//	cloudstack-csi-driver -h
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"

	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"

	"github.com/leaseweb/cloudstack-csi-driver/pkg/cloud"
	"github.com/leaseweb/cloudstack-csi-driver/pkg/driver"
)

var (
	endpoint         = flag.String("endpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	cloudstackconfig = flag.String("cloudstackconfig", "./cloud-config", "CloudStack configuration file")
	nodeName         = flag.String("nodeName", "", "Node name")
	showVersion      = flag.Bool("version", false, "Show version")

	// Version is set by the build process.
	version = ""
)

func main() {
	if err := logsapi.RegisterLogFormat(logsapi.JSONLogFormat, json.Factory{}, logsapi.LoggingBetaOptions); err != nil {
		klog.ErrorS(err, "failed to register JSON log format")
	}

	fg := featuregate.NewFeatureGate()
	err := logsapi.AddFeatureGates(fg)
	if err != nil {
		klog.ErrorS(err, "failed to add feature gates")
	}

	c := logsapi.NewLoggingConfiguration()
	logsapi.AddGoFlags(c, flag.CommandLine)
	flag.Parse()
	logs.InitLogs()
	logger := klog.Background()
	if err = logsapi.ValidateAndApply(c, fg); err != nil {
		logger.Error(err, "LoggingConfiguration is invalid")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if *showVersion {
		baseName := path.Base(os.Args[0])
		fmt.Println(baseName, version) //nolint:forbidigo
		os.Exit(0)
	}

	// Setup cloud connector.
	config, err := cloud.ReadConfig(*cloudstackconfig)
	if err != nil {
		logger.Error(err, "Cannot read CloudStack configuration")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	logger.Info("Successfully read CloudStack configuration", "cloudstackconfig", *cloudstackconfig)

	ctx := klog.NewContext(context.Background(), logger)
	csConnector := cloud.New(config)

	d, err := driver.New(*endpoint, csConnector, nil, *nodeName, version)
	if err != nil {
		logger.Error(err, "Failed to initialize driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if err = d.Run(ctx); err != nil {
		logger.Error(err, "Failed to run driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
