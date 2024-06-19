package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/leaseweb/cloudstack-csi-driver/pkg/cloud"
	"github.com/leaseweb/cloudstack-csi-driver/pkg/mount"
	"github.com/leaseweb/cloudstack-csi-driver/pkg/util"
)

const (
	// default file system type to be used when it is not provided.
	defaultFsType = "ext4"
)

type nodeServer struct {
	csi.UnimplementedNodeServer
	connector   cloud.Interface
	mounter     mount.Interface
	nodeName    string
	volumeLocks *util.VolumeLocks
}

// NewNodeServer creates a new Node gRPC server.
func NewNodeServer(connector cloud.Interface, mounter mount.Interface, nodeName string) csi.NodeServer {
	if mounter == nil {
		mounter = mount.New()
	}

	return &nodeServer{
		connector:   connector,
		mounter:     mounter,
		nodeName:    nodeName,
		volumeLocks: util.NewVolumeLocks(),
	}
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("NodeStageVolume: called", "args", *req)

	// Check parameters

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if target == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	if acquired := ns.volumeLocks.TryAcquire(volumeID); !acquired {
		logger.Error(errors.New(util.ErrVolumeOperationAlreadyExistsVolumeID), "failed to acquire volume lock", "volumeID", volumeID)

		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer ns.volumeLocks.Release(volumeID)

	// Now, find the device path

	pubCtx := req.GetPublishContext()
	deviceID := pubCtx[deviceIDContextKey]

	devicePath, err := ns.mounter.GetDevicePath(ctx, volumeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot find device path for volume %s: %s", volumeID, err.Error())
	}

	logger.Info("Device found",
		"devicePath", devicePath,
		"deviceID", deviceID,
	)

	// If the access type is block, do nothing for stage
	if blk := volCap.GetBlock(); blk != nil {
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// The access type should now be "Mount".
	// We have to format the partition.

	mnt := volCap.GetMount()
	if mnt == nil {
		return nil, status.Error(codes.InvalidArgument, "Neither block nor mount volume capability")
	}

	// Verify whether mounted
	notMnt, err := ns.mounter.IsLikelyNotMountPoint(target)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	fsType := mnt.GetFsType()
	if fsType == "" {
		fsType = defaultFsType
	}

	var mountOptions []string
	for _, f := range mnt.GetMountFlags() {
		if !hasMountOption(mountOptions, f) {
			mountOptions = append(mountOptions, f)
		}
	}

	// Volume Mount
	if notMnt {
		logger.Info("NodeStageVolume: formatting and mounting",
			"devicePath", devicePath,
			"target", target,
			"fsType", fsType,
			"options", mountOptions,
		)
		err = ns.mounter.FormatAndMount(devicePath, target, fsType, mountOptions)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

// hasMountOption returns a boolean indicating whether the given
// slice already contains a mount option. This is used to prevent
// passing duplicate option to the mount command.
func hasMountOption(options []string, opt string) bool {
	for _, o := range options {
		if o == opt {
			return true
		}
	}

	return false
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("NodeUnstageVolume: called", "args", *req)

	// Check parameters

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if target == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	if acquired := ns.volumeLocks.TryAcquire(volumeID); !acquired {
		logger.Error(errors.New(util.ErrVolumeOperationAlreadyExistsVolumeID), "failed to acquire volume lock", "volumeID", volumeID)

		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer ns.volumeLocks.Release(volumeID)

	notMnt, err := ns.mounter.IsLikelyNotMountPoint(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Error(codes.NotFound, "Target path not found")
		}

		return nil, status.Error(codes.Internal, err.Error())
	}
	if notMnt {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	logger.Info("NodeUnstageVolume: unmounting",
		"target", target,
	)

	err = ns.mounter.CleanupMountPoint(target, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount target %q: %v", target, err)
	}

	logger.Info("NodeUnstageVolume: unmount successful",
		"target", target,
	)

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) { //nolint:gocognit
	logger := klog.FromContext(ctx)
	logger.V(6).Info("NodePublishVolume: called", "args", *req)

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	volumeID := req.GetVolumeId()

	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()

	if req.GetVolumeCapability().GetBlock() != nil &&
		req.GetVolumeCapability().GetMount() != nil {
		return nil, status.Error(codes.InvalidArgument, "Cannot have both block and mount access type")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging target path missing in request")
	}

	readOnly := req.GetReadonly()
	options := []string{"bind"}
	if readOnly {
		options = append(options, "ro")
	}

	deviceID := ""
	if req.GetPublishContext() != nil {
		deviceID = req.GetPublishContext()[deviceIDContextKey]
	}

	// Considering kubelet ensures the stage and publish operations
	// are serialized, we don't need any extra locking in NodePublishVolume.

	if req.GetVolumeCapability().GetMount() != nil { //nolint:nestif
		source := req.GetStagingTargetPath()

		notMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				if err := ns.mounter.MakeDir(targetPath); err != nil {
					return nil, status.Errorf(codes.Internal, "Could not create dir %q: %v", targetPath, err)
				}
			} else {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		if !notMnt {
			logger.Info("NodePublishVolume: volume is already mounted",
				"source", source,
				"targetPath", targetPath,
			)

			return &csi.NodePublishVolumeResponse{}, nil
		}

		fsType := req.GetVolumeCapability().GetMount().GetFsType()

		mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

		logger.Info("NodePublishVolume: mounting source",
			"source", source,
			"targetPath", targetPath,
			"fsType", fsType,
			"deviceID", deviceID,
			"readOnly", readOnly,
			"volumeID", volumeID,
			"mountFlags", mountFlags,
		)

		if err := ns.mounter.Mount(source, targetPath, fsType, options); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to mount %s at %s: %s", source, targetPath, err.Error())
		}
	}

	if req.GetVolumeCapability().GetBlock() != nil { //nolint:nestif
		volumeID := req.GetVolumeId()

		devicePath, err := ns.mounter.GetDevicePath(ctx, volumeID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Cannot find device path for volume %s: %s", volumeID, err.Error())
		}

		globalMountPath := filepath.Dir(targetPath)
		exists, err := ns.mounter.ExistsPath(globalMountPath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not check if path exists %q: %v", globalMountPath, err)
		}
		if !exists {
			if err = ns.mounter.MakeDir(globalMountPath); err != nil {
				return nil, status.Errorf(codes.Internal, "Could not create dir %q: %v", globalMountPath, err)
			}
		}

		err = ns.mounter.MakeFile(targetPath)
		if err != nil {
			if removeErr := os.Remove(targetPath); removeErr != nil {
				return nil, status.Errorf(codes.Internal, "Could not remove mount target %q: %v", targetPath, removeErr)
			}

			return nil, status.Errorf(codes.Internal, "Could not create file %q: %v", targetPath, err)
		}

		logger.Info("NodePublishVolume: mounting device",
			"devicePath", devicePath,
			"targetPath", targetPath,
			"deviceID", deviceID,
			"readOnly", readOnly,
			"volumeID", volumeID,
		)

		if err := ns.mounter.Mount(devicePath, targetPath, "", options); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to mount %s at %s: %s", devicePath, targetPath, err.Error())
		}
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("NodeUnpublishVolume: called", "args", *req)

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	// Considering that kubelet ensures the stage and publish operations
	// are serialized, we don't need any extra locking in NodeUnpublishVolume.

	if _, err := ns.connector.GetVolumeByID(ctx, volumeID); errors.Is(err, cloud.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "Volume %v not found", volumeID)
	} else if err != nil {
		// Error with CloudStack
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}

	logger.Info("NodeUnpublishVolume: unmounting volume",
		"targetPath", targetPath,
		"volumeID", volumeID,
	)

	err := ns.mounter.CleanupMountPoint(targetPath, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount target %q: %v", targetPath, err)
	}

	logger.Info("NodeUnpublishVolume: unmounting successful",
		"targetPath", targetPath,
		"volumeID", volumeID,
	)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("NodeGetInfo: called", "args", *req)

	if ns.nodeName == "" {
		return nil, status.Error(codes.Internal, "Missing node name")
	}

	vm, err := ns.connector.GetNodeInfo(ctx, ns.nodeName)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if vm.ID == "" {
		return nil, status.Error(codes.Internal, "Node with no ID")
	}
	if vm.ZoneID == "" {
		return nil, status.Error(codes.Internal, "Node zone ID not found")
	}

	topology := Topology{ZoneID: vm.ZoneID}

	return &csi.NodeGetInfoResponse{
		NodeId:             vm.ID,
		AccessibleTopology: topology.ToCSI(),
	}, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("NodeExpandVolume: called", "args", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	// Get volume path
	// This should work for Kubernetes >= 1.26, see https://github.com/kubernetes/kubernetes/issues/115343
	volumePath := req.GetStagingTargetPath()
	if volumePath == "" {
		// Except that it doesn't work in the sanity test, so we need a fallback to volumePath.
		volumePath = req.GetVolumePath()
	}
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap != nil {
		switch volCap.GetAccessType().(type) { //nolint:gocritic
		case *csi.VolumeCapability_Block:
			logger.Info("Filesystem expansion is skipped for block volumes")

			return &csi.NodeExpandVolumeResponse{}, nil
		}
	}

	if acquired := ns.volumeLocks.TryAcquire(volumeID); !acquired {
		logger.Error(errors.New(util.ErrVolumeOperationAlreadyExistsVolumeID), "failed to acquire volume lock", "volumeID", volumeID)

		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer ns.volumeLocks.Release(volumeID)

	_, err := ns.connector.GetVolumeByID(ctx, volumeID)
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("Volume with ID %s not found", volumeID))
		}

		return nil, status.Error(codes.Internal, fmt.Sprintf("NodeExpandVolume failed with error %v", err))
	}

	devicePath, err := ns.mounter.GetDevicePath(ctx, volumeID)
	if devicePath == "" {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to find Device path for volume %s: %v", volumeID, err))
	}

	logger.Info("Expanding volume",
		"devicePath", devicePath,
		"volumeID", volumeID,
		"volumePath", volumePath,
	)

	r := ns.mounter.NewResizeFs(mount.New())
	if _, err := r.Resize(devicePath, volumePath); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not resize volume %q:  %v", volumeID, err)
	}

	return &csi.NodeExpandVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	resp := &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
		},
	}

	return resp, nil
}
