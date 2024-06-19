package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

const (
	cloudInitInstanceFilePath = "/run/cloud-init/instance-data.json"
	cloudStackCloudName       = "cloudstack"
)

func (c *client) metadataInstanceID(ctx context.Context) string {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Attempting to retrieve metadata from envvar NODE_ID")

	// Try a NODE_ID environment variable
	if envNodeID := os.Getenv("NODE_ID"); envNodeID != "" {
		logger.V(4).Info("Found CloudStack VM ID from envvar NODE_ID", "nodeID", envNodeID)

		return envNodeID
	}

	// Try cloud-init
	logger.V(4).Info("Environment variable NODE_ID not found, trying with cloud-init")
	if _, err := os.Stat(cloudInitInstanceFilePath); err == nil {
		logger.V(4).Info("File " + cloudInitInstanceFilePath + " exists")
		ciData, err := c.readCloudInit(ctx, cloudInitInstanceFilePath)
		if err != nil {
			logger.Error(err, "Cannot read cloud-init instance data")
		} else if ciData.V1.InstanceID != "" {
			logger.V(4).Info("Found CloudStack VM ID from cloud-init", "nodeID", ciData.V1.InstanceID)

			return ciData.V1.InstanceID
		}
		slog.Error("cloud-init instance ID is not provided")
	} else if os.IsNotExist(err) {
		logger.V(4).Info("File " + cloudInitInstanceFilePath + " does not exist")
	} else {
		logger.Error(err, "Cannot read file "+cloudInitInstanceFilePath)
	}

	logger.V(4).Info("CloudStack VM ID not found in meta-data")

	return ""
}

type cloudInitInstanceData struct {
	V1 cloudInitV1 `json:"v1"`
}

type cloudInitV1 struct {
	CloudName  string `json:"cloud_name"`
	InstanceID string `json:"instance_id"`
	Zone       string `json:"availability_zone"`
}

func (c *client) readCloudInit(ctx context.Context, instanceFilePath string) (*cloudInitInstanceData, error) {
	logger := klog.FromContext(ctx)

	b, err := os.ReadFile(instanceFilePath)
	if err != nil {
		logger.Error(err, "Cannot read file "+instanceFilePath)

		return nil, err
	}

	var data cloudInitInstanceData
	if err := json.Unmarshal(b, &data); err != nil {
		logger.Error(err, "Cannot parse JSON file "+instanceFilePath)

		return nil, err
	}

	if strings.ToLower(data.V1.CloudName) != cloudStackCloudName {
		err := fmt.Errorf("cloud name from cloud-init is %s, only %s is supported", data.V1.CloudName, cloudStackCloudName)
		logger.Error(err, "Unsupported cloud name detected")

		return nil, err
	}

	return &data, nil
}
