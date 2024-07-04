package cloud

import (
	"context"

	"k8s.io/klog/v2"
)

func (c *client) GetNodeInfo(ctx context.Context, vmName string) (*VM, error) {
	logger := klog.FromContext(ctx)

	// First, try to read the instance ID from meta-data.
	if id := c.metadataInstanceID(ctx); id != "" {
		// Instance ID found using metadata
		logger.V(4).Info("Looking up node info using VM ID found in metadata", "nodeID", id)

		// Use CloudStack API to get VM info
		return c.GetVMByID(ctx, id)
	}

	// VM ID was not found using metadata, fall back to using VM name instead.
	logger.V(4).Info("Looking up node info using VM name", "nodeName", vmName)

	return c.getVMByName(ctx, vmName)
}
