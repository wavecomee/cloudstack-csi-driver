package mount

import (
	"context"
	"os"

	"k8s.io/mount-utils"
	exec "k8s.io/utils/exec/testing"
)

const (
	giB = 1 << 30
)

type fakeMounter struct {
	mount.SafeFormatAndMount
}

// NewFake creates a fake implementation of the
// mount.Mounter, to be used in tests.
func NewFake() Mounter {
	return &fakeMounter{
		mount.SafeFormatAndMount{
			Interface: mount.NewFakeMounter([]mount.MountPoint{}),
			Exec:      &exec.FakeExec{DisableScripts: true},
		},
	}
}

func (m *fakeMounter) GetBlockSizeBytes(_ string) (int64, error) {
	return 1073741824, nil
}

func (m *fakeMounter) GetDevicePath(_ context.Context, _ string) (string, error) {
	// For fake testing, return a generic device path
	// In real testing, this could be made configurable based on hypervisor
	return "/dev/sdb", nil
}

func (m *fakeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	return mount.GetDeviceNameFromMount(m, mountPath)
}

func (*fakeMounter) PathExists(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func (*fakeMounter) MakeDir(path string) error {
	err := os.MkdirAll(path, os.FileMode(0o755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	return nil
}

func (*fakeMounter) MakeFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE, os.FileMode(0o644))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err = file.Close(); err != nil {
		return err
	}

	return nil
}

func (m *fakeMounter) GetStatistics(_ string) (VolumeStatistics, error) {
	return VolumeStatistics{
		AvailableBytes: 3 * giB,
		TotalBytes:     10 * giB,
		UsedBytes:      7 * giB,

		AvailableInodes: 3000,
		TotalInodes:     10000,
		UsedInodes:      7000,
	}, nil
}

func (m *fakeMounter) IsBlockDevice(_ string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) IsCorruptedMnt(_ error) bool {
	return false
}

func (m *fakeMounter) NeedResize(_ string, _ string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) Resize(_ string, _ string) (bool, error) {
	return true, nil
}

func (m *fakeMounter) Unpublish(path string) error {
	return m.Unstage(path)
}

func (m *fakeMounter) Unstage(path string) error {
	return mount.CleanupMountPoint(path, m, true)
}
