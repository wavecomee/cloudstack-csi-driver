package driver

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/wavecomee/cloudstack-csi-driver/pkg/cloud"
)

var (
	FakeCapacityGiB    = 1
	FakeVolName        = "CSIVolumeName"
	FakeVolID          = "CSIVolumeID"
	FakeAvailability   = "nova"
	FakeDiskOfferingID = "9743fd77-0f5d-4ef9-b2f8-f194235c769c"
	FakeVol            = cloud.Volume{
		ID:     FakeVolID,
		Name:   FakeVolName,
		Size:   int64(FakeCapacityGiB),
		ZoneID: FakeAvailability,
	}
)

func TestDetermineSize(t *testing.T) {
	cases := []struct {
		name          string
		capacityRange *csi.CapacityRange
		expectedSize  int64
		expectError   bool
	}{
		{"no range", nil, 1, false},
		{"only limit", &csi.CapacityRange{LimitBytes: 100 * 1024 * 1024 * 1024}, 1, false},
		{"only limit (too small)", &csi.CapacityRange{LimitBytes: 1024 * 1024}, 0, true},
		{"only required", &csi.CapacityRange{RequiredBytes: 50 * 1024 * 1024 * 1024}, 50, false},
		{"required and limit", &csi.CapacityRange{RequiredBytes: 25 * 1024 * 1024 * 1024, LimitBytes: 100 * 1024 * 1024 * 1024}, 25, false},
		{"required = limit", &csi.CapacityRange{RequiredBytes: 30 * 1024 * 1024 * 1024, LimitBytes: 30 * 1024 * 1024 * 1024}, 30, false},
		{"required = limit (not GB int)", &csi.CapacityRange{RequiredBytes: 3_000_000_000, LimitBytes: 3_000_000_000}, 0, true},
		{"no int GB int possible", &csi.CapacityRange{RequiredBytes: 4_000_000_000, LimitBytes: 1_000_001_000}, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &csi.CreateVolumeRequest{
				CapacityRange: c.capacityRange,
			}
			size, err := determineSize(req)
			if err != nil && !c.expectError {
				t.Errorf("Unexepcted error: %v", err.Error())
			}
			if err == nil && c.expectError {
				t.Error("Expected an error")
			}
			if size != c.expectedSize {
				t.Errorf("Expected size %v, got %v", c.expectedSize, size)
			}
		})
	}
}

func TestCreateVolume(t *testing.T) {
	ctx := context.Background()
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()
	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), FakeDiskOfferingID, FakeAvailability, FakeVolName, gomock.Any()).Return(FakeVolID, nil)
	mockCloud.EXPECT().GetVolumeByName(gomock.Eq(ctx), FakeVolName).Return(nil, cloud.ErrNotFound)
	fakeCs := NewControllerService(mockCloud)
	// mock CloudStack
	// CreateVolume(ctx context.Context, diskOfferingID, zoneID, name string, sizeInGB int64) (string, error)
	// csmock.On("CreateVolume", FakeCtx, FakeDiskOfferingID, FakeAvailability, FakeVolName, mock.AnythingOfType("int64")).Return(FakeVolID, nil)
	// csmock.On("GetVolumeByName", FakeCtx, FakeVolName).Return(nil, cloud.ErrNotFound)
	// Init assert
	assert := assert.New(t)
	// Fake request
	fakeReq := &csi.CreateVolumeRequest{
		Name: FakeVolName,
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		Parameters: map[string]string{
			DiskOfferingKey: FakeDiskOfferingID,
		},
		AccessibilityRequirements: &csi.TopologyRequirement{
			Requisite: []*csi.Topology{
				{
					Segments: map[string]string{"topology.csi.cloudstack.apache.org/zone": FakeAvailability},
				},
			},
		},
	}
	// Invoke CreateVolume
	actualRes, err := fakeCs.CreateVolume(ctx, fakeReq)
	if err != nil {
		t.Errorf("failed to CreateVolume: %v", err)
	}
	// Assert
	assert.NotNil(actualRes.GetVolume())
	assert.NotNil(actualRes.GetVolume().GetCapacityBytes())
	assert.NotEmpty(actualRes.GetVolume().GetVolumeId(), "Volume Id is empty")
	assert.NotNil(actualRes.GetVolume().GetAccessibleTopology())
	assert.Equal(FakeAvailability, actualRes.GetVolume().GetAccessibleTopology()[0].GetSegments()[ZoneKey])
}
