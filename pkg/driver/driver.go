// Package driver provides the implementation of the CSI plugin.
//
// It contains the gRPC server implementation of CSI specification.
package driver

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog/v2"

	"github.com/leaseweb/cloudstack-csi-driver/pkg/cloud"
	"github.com/leaseweb/cloudstack-csi-driver/pkg/mount"
)

// Interface is the CloudStack CSI driver interface.
type Interface interface {
	// Run the CSI driver gRPC server
	Run(ctx context.Context) error
}

type cloudstackDriver struct {
	controller csi.ControllerServer
	identity   csi.IdentityServer
	node       csi.NodeServer
	options    *Options
}

// New instantiates a new CloudStack CSI driver.
func New(ctx context.Context, csConnector cloud.Interface, options *Options, mounter mount.Interface) (Interface, error) {
	logger := klog.FromContext(ctx)
	logger.Info("Driver starting", "Driver", DriverName, "Version", driverVersion)

	if err := validateMode(options.Mode); err != nil {
		return nil, fmt.Errorf("invalid driver options: %w", err)
	}

	driver := &cloudstackDriver{
		options: options,
	}

	driver.identity = NewIdentityServer(driverVersion)
	switch options.Mode {
	case ControllerMode:
		driver.controller = NewControllerServer(csConnector)
	case NodeMode:
		driver.node = NewNodeServer(csConnector, mounter, options)
	case AllMode:
		driver.controller = NewControllerServer(csConnector)
		driver.node = NewNodeServer(csConnector, mounter, options)
	default:
		return nil, fmt.Errorf("unknown mode: %s", options.Mode)
	}

	return driver, nil
}

func (cs *cloudstackDriver) Run(ctx context.Context) error {
	return cs.serve(ctx)
}

func validateMode(mode Mode) error {
	if mode != AllMode && mode != ControllerMode && mode != NodeMode {
		return fmt.Errorf("mode is not supported (actual: %s, supported: %v)", mode, []Mode{AllMode, ControllerMode, NodeMode})
	}

	return nil
}
