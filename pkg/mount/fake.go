package mount

import (
	"context"
	"os"

	"k8s.io/mount-utils"
	exec "k8s.io/utils/exec/testing"
)

type fakeMounter struct {
	mount.SafeFormatAndMount
}

// NewFake creates a fake implementation of the
// mount.Interface, to be used in tests.
func NewFake() Interface {
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
	return "/dev/sdb", nil
}

func (m *fakeMounter) GetDeviceName(mountPath string) (string, int, error) {
	return mount.GetDeviceNameFromMount(m, mountPath)
}

func (*fakeMounter) PathExists(_ string) (bool, error) {
	return true, nil
}

func (*fakeMounter) MakeDir(pathname string) error {
	err := os.MkdirAll(pathname, os.FileMode(0o755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	return nil
}

func (*fakeMounter) MakeFile(_ string) error {
	return nil
}

func (m *fakeMounter) GetStatistics(_ string) (volumeStatistics, error) {
	return volumeStatistics{}, nil
}

func (m *fakeMounter) IsBlockDevice(_ string) (bool, error) {
	return true, nil
}

func (m *fakeMounter) IsCorruptedMnt(_ error) bool {
	return false
}

func (m *fakeMounter) NeedResize(_ string, _ string) (bool, error) {
	return true, nil
}

func (m *fakeMounter) Resize(_ string, _ string) (bool, error) {
	return true, nil
}

func (m *fakeMounter) Unpublish(_ string) error {
	return nil
}

func (m *fakeMounter) Unstage(_ string) error {
	return nil
}
