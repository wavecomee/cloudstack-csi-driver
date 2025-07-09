package driver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	cloud "github.com/wavecomee/cloudstack-csi-driver/pkg/cloud/fake"
	"github.com/wavecomee/cloudstack-csi-driver/pkg/mount"
	"github.com/wavecomee/cloudstack-csi-driver/pkg/util"
)

const (
	//nolint:godox
	// TODO: Adjusted this to paths in /tmp until https://github.com/kubernetes/kubernetes/pull/128286
	//       is solved.
	sourceTest = "/tmp/source_test"
	targetTest = "/tmp/target_test"
)

func TestNodePublishVolumeIdempotentMount(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root")
	}
	logger := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.Verbosity(10), ktesting.BufferLogs(true)))
	ctx := klog.NewContext(context.Background(), logger)

	driver := &NodeService{
		connector:   cloud.New(),
		mounter:     mount.New(),
		volumeLocks: util.NewVolumeLocks(),
	}

	err := driver.mounter.MakeDir(sourceTest)
	require.NoError(t, err)
	err = driver.mounter.MakeDir(targetTest)
	require.NoError(t, err)

	volCapAccessMode := csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}
	volCapAccessType := csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}
	req := csi.NodePublishVolumeRequest{
		VolumeCapability:  &csi.VolumeCapability{AccessMode: &volCapAccessMode, AccessType: &volCapAccessType},
		VolumeId:          "vol_1",
		TargetPath:        targetTest,
		StagingTargetPath: sourceTest,
		Readonly:          true,
	}

	underlyingLogger, ok := logger.GetSink().(ktesting.Underlier)
	if !ok {
		t.Fatalf("should have had ktesting LogSink, got %T", logger.GetSink())
	}

	_, err = driver.NodePublishVolume(ctx, &req)
	require.NoError(t, err)
	_, err = driver.NodePublishVolume(ctx, &req)
	require.NoError(t, err)

	logEntries := underlyingLogger.GetBuffer().String()
	assert.Contains(t, logEntries, "Target path is already mounted")

	// ensure the target not be mounted twice
	targetAbs, err := filepath.Abs(targetTest)
	require.NoError(t, err)

	mountList, err := driver.mounter.List()
	require.NoError(t, err)
	mountPointNum := 0
	for _, mountPoint := range mountList {
		if mountPoint.Path == targetAbs {
			mountPointNum++
		}
	}
	assert.Equal(t, 1, mountPointNum)
	err = driver.mounter.Unmount(targetTest)
	require.NoError(t, err)
	_ = driver.mounter.Unmount(targetTest)
	err = os.RemoveAll(sourceTest)
	require.NoError(t, err)
	err = os.RemoveAll(targetTest)
	require.NoError(t, err)
}
