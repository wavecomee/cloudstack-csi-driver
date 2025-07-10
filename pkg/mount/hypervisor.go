package mount

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// HypervisorType represents the detected hypervisor type
type HypervisorType string

const (
	HypervisorKVM  HypervisorType = "kvm"
	HypervisorXen  HypervisorType = "xen"
	HypervisorAuto HypervisorType = "auto"
)

// hypervisorCache caches the detected hypervisor type to avoid repeated detection
type hypervisorCache struct {
	hypervisorType HypervisorType
	detectedAt     time.Time
	mu             sync.RWMutex
}

var globalHypervisorCache = &hypervisorCache{}

// Get returns the cached hypervisor type if recent, otherwise returns auto
func (c *hypervisorCache) Get() HypervisorType {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return cached value if recent (5 minutes)
	if time.Since(c.detectedAt) < 5*time.Minute {
		return c.hypervisorType
	}

	return HypervisorAuto
}

// Set updates the cached hypervisor type
func (c *hypervisorCache) Set(hypervisor HypervisorType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hypervisorType = hypervisor
	c.detectedAt = time.Now()
}

// DetectHypervisor automatically detects the hypervisor type
func DetectHypervisor() HypervisorType {
	// Check cache first
	if cached := globalHypervisorCache.Get(); cached != HypervisorAuto {
		return cached
	}

	// Try system files detection first (most reliable)
	if hypervisor := detectFromSystemFiles(); hypervisor != HypervisorAuto {
		globalHypervisorCache.Set(hypervisor)
		return hypervisor
	}

	// Try device path detection as fallback
	if hypervisor := detectFromDevicePaths(); hypervisor != HypervisorAuto {
		globalHypervisorCache.Set(hypervisor)
		return hypervisor
	}

	// Default to KVM (safest fallback)
	globalHypervisorCache.Set(HypervisorKVM)
	return HypervisorKVM
}

// detectFromSystemFiles detects hypervisor by checking system files
func detectFromSystemFiles() HypervisorType {
	// Check for Xen
	if data, err := os.ReadFile("/sys/hypervisor/type"); err == nil {
		if strings.TrimSpace(string(data)) == "xen" {
			return HypervisorXen
		}
	}

	// Check for Xen in /proc/xen/
	if _, err := os.Stat("/proc/xen"); err == nil {
		return HypervisorXen
	}

	// Check for KVM
	if _, err := os.Stat("/sys/class/kvm"); err == nil {
		return HypervisorKVM
	}

	// Check /proc/cpuinfo for hypervisor hints
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		cpuInfo := strings.ToLower(string(data))
		if strings.Contains(cpuInfo, "xen") {
			return HypervisorXen
		}
		if strings.Contains(cpuInfo, "kvm") || strings.Contains(cpuInfo, "qemu") {
			return HypervisorKVM
		}
	}

	return HypervisorAuto
}

// detectFromDevicePaths detects hypervisor by checking device patterns
func detectFromDevicePaths() HypervisorType {
	// Check for Xen devices
	if entries, err := os.ReadDir("/dev"); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "xvd") {
				return HypervisorXen
			}
		}
	}

	// Check for KVM devices
	if entries, err := os.ReadDir("/dev/disk/by-id"); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "virtio-") ||
				strings.HasPrefix(entry.Name(), "scsi-0QEMU_QEMU_HARDDISK_") {
				return HypervisorKVM
			}
		}
	}

	return HypervisorAuto
}

// xenDeviceIDToPath converts a CloudStack device ID to Xen device path
// Uses the formula: device_id=$(( $(printf "%d" "'$letter") - $(printf "%d" "'a") ))
// Xen devices are named /dev/xvd* where * is a letter (a-z)
func xenDeviceIDToPath(deviceID string) (string, error) {
	// Parse device ID to integer
	id, err := strconv.Atoi(deviceID)
	if err != nil {
		return "", fmt.Errorf("invalid device ID %s: %w", deviceID, err)
	}

	// Convert to letter (0=a, 1=b, 2=c, etc.)
	if id < 0 || id > 25 {
		return "", fmt.Errorf("device ID %d out of range (0-25)", id)
	}

	// Convert to letter: 0=a, 1=b, 2=c, etc.
	letter := rune('a' + id)
	devicePath := fmt.Sprintf("/dev/xvd%c", letter)

	return devicePath, nil
}

// getXenDevicePaths returns possible Xen device paths for a given volume ID
func getXenDevicePaths(volumeID string) []string {
	// For Xen, we need to try different approaches since the device ID
	// comes from CloudStack's volume attachment response

	// First, try the volume ID as device ID (common case)
	if devicePath, err := xenDeviceIDToPath(volumeID); err == nil {
		return []string{devicePath}
	}

	// If that fails, try common Xen device patterns
	// This is a fallback for cases where device ID is not directly available
	var paths []string
	for i := 0; i < 26; i++ { // a-z
		letter := rune('a' + i)
		paths = append(paths, fmt.Sprintf("/dev/xvd%c", letter))
	}

	return paths
}

// logHypervisorDetection logs the hypervisor detection result
func logHypervisorDetection(ctx context.Context, hypervisor HypervisorType, method string) {
	logger := klog.FromContext(ctx)
	logger.Info("Hypervisor detected",
		"hypervisor", hypervisor,
		"method", method,
		"confidence", getConfidenceLevel(method))
}

// getConfidenceLevel returns confidence level for detection method
func getConfidenceLevel(method string) string {
	switch method {
	case "system_files":
		return "high"
	case "device_paths":
		return "medium"
	default:
		return "low"
	}
}
