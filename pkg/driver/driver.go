// Package driver provides the implementation of the CSI plugin.
//
// It contains the gRPC server implementation of CSI specification.
package driver

import (
	"context"

	"github.com/leaseweb/cloudstack-csi-driver/pkg/cloud"
	"github.com/leaseweb/cloudstack-csi-driver/pkg/mount"
)

// Interface is the CloudStack CSI driver interface.
type Interface interface {
	// Run the CSI driver gRPC server
	Run(ctx context.Context) error
}

type cloudstackDriver struct {
	endpoint string
	nodeName string
	version  string

	connector cloud.Interface
	mounter   mount.Interface
}

// New instantiates a new CloudStack CSI driver.
func New(endpoint string, csConnector cloud.Interface, mounter mount.Interface, nodeName string, version string) (Interface, error) {
	return &cloudstackDriver{
		endpoint:  endpoint,
		nodeName:  nodeName,
		version:   version,
		connector: csConnector,
		mounter:   mounter,
	}, nil
}

func (cs *cloudstackDriver) Run(ctx context.Context) error {
	ids := NewIdentityServer(cs.version)
	ctrls := NewControllerServer(cs.connector)
	ns := NewNodeServer(cs.connector, cs.mounter, cs.nodeName)

	return cs.serve(ctx, ids, ctrls, ns)
}
