package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type identityServer struct {
	csi.UnimplementedIdentityServer
	version string
}

// NewIdentityServer creates a new Identity gRPC server.
func NewIdentityServer(version string) csi.IdentityServer {
	return &identityServer{
		version: version,
	}
}

func (ids *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("GetPluginInfo: called", "args", *req)
	if ids.version == "" {
		return nil, status.Error(codes.Unavailable, "Driver is missing version")
	}

	resp := &csi.GetPluginInfoResponse{
		Name:          DriverName,
		VendorVersion: ids.version,
	}

	return resp, nil
}

func (ids *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("Probe: called", "args", *req)

	return &csi.ProbeResponse{}, nil
}

func (ids *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("Probe: called", "args", *req)

	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}

	return resp, nil
}
