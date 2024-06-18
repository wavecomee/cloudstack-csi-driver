package mount

import (
	"context"
	"os"

	"k8s.io/mount-utils"
	utilsexec "k8s.io/utils/exec"
	exec "k8s.io/utils/exec/testing"
)

type fakeMounter struct {
	mount.SafeFormatAndMount
	utilsexec.Interface
}

// NewFake creates a fake implementation of the
// mount.Interface, to be used in tests.
func NewFake() Interface {
	return &fakeMounter{
		mount.SafeFormatAndMount{
			Interface: mount.NewFakeMounter([]mount.MountPoint{}),
			Exec:      &exec.FakeExec{DisableScripts: true},
		},
		utilsexec.New(),
	}
}

func (m *fakeMounter) CleanupMountPoint(path string, extensiveCheck bool) error {
	return mount.CleanupMountPoint(path, m, extensiveCheck)
}

func (m *fakeMounter) GetDevicePath(_ context.Context, _ string) (string, error) {
	return "/dev/sdb", nil
}

func (m *fakeMounter) GetDeviceName(mountPath string) (string, int, error) {
	return mount.GetDeviceNameFromMount(m, mountPath)
}

func (*fakeMounter) ExistsPath(_ string) (bool, error) {
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

func (*fakeMounter) NewResizeFs(_ utilsexec.Interface) *mount.ResizeFs {
	return mount.NewResizeFs(New())
}
