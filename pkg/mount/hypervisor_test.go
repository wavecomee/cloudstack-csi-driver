package mount

import (
	"testing"
)

func TestXenDeviceIDToPath(t *testing.T) {
	tests := []struct {
		name      string
		deviceID  string
		expected  string
		expectErr bool
	}{
		{
			name:      "device ID 0",
			deviceID:  "0",
			expected:  "/dev/xvda",
			expectErr: false,
		},
		{
			name:      "device ID 1",
			deviceID:  "1",
			expected:  "/dev/xvdb",
			expectErr: false,
		},
		{
			name:      "device ID 25",
			deviceID:  "25",
			expected:  "/dev/xvdz",
			expectErr: false,
		},
		{
			name:      "device ID -1",
			deviceID:  "-1",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "device ID 26",
			deviceID:  "26",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "invalid device ID",
			deviceID:  "abc",
			expected:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := xenDeviceIDToPath(tt.deviceID)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestGetXenDevicePaths(t *testing.T) {
	// Test with a valid device ID
	paths := getXenDevicePaths("1")
	if len(paths) == 0 {
		t.Error("expected at least one device path")
	}
	if paths[0] != "/dev/xvdb" {
		t.Errorf("expected /dev/xvdb, got %s", paths[0])
	}

	// Test with an invalid device ID (should return all possible paths)
	paths = getXenDevicePaths("invalid")
	if len(paths) != 26 {
		t.Errorf("expected 26 paths, got %d", len(paths))
	}
}

func TestHypervisorCache(t *testing.T) {
	cache := &hypervisorCache{}

	// Test initial state
	if cache.Get() != HypervisorAuto {
		t.Errorf("expected HypervisorAuto, got %s", cache.Get())
	}

	// Test setting and getting
	cache.Set(HypervisorXen)
	if cache.Get() != HypervisorXen {
		t.Errorf("expected HypervisorXen, got %s", cache.Get())
	}

	// Test setting another value
	cache.Set(HypervisorKVM)
	if cache.Get() != HypervisorKVM {
		t.Errorf("expected HypervisorKVM, got %s", cache.Get())
	}
}

func TestDetectFromSystemFiles(t *testing.T) {
	// This test is limited by the fact that we can't easily mock
	// system file access in a unit test, but we can test the logic
	// that would be used in the real detection
	// In a real environment, this would check:
	// - /sys/hypervisor/type for "xen"
	// - /proc/xen/ directory existence
	// - /sys/class/kvm/ directory existence
	// - /proc/cpuinfo for hypervisor hints

	t.Skip("System file detection test requires actual system access")
}

func TestGetConfidenceLevel(t *testing.T) {
	tests := []struct {
		method     string
		confidence string
	}{
		{"system_files", "high"},
		{"device_paths", "medium"},
		{"unknown", "low"},
	}

	for _, tt := range tests {
		result := getConfidenceLevel(tt.method)
		if result != tt.confidence {
			t.Errorf("method %s: expected %s, got %s", tt.method, tt.confidence, result)
		}
	}
}
