package cloud

import (
	"context"

	"k8s.io/klog/v2"
)

func (c *client) ListZonesID(ctx context.Context) ([]string, error) {
	logger := klog.FromContext(ctx)
	result := make([]string, 0)
	p := c.Zone.NewListZonesParams()
	p.SetAvailable(true)
	logger.V(2).Info("CloudStack API call", "command", "ListZones", "params", map[string]string{
		"available": "true",
	})
	r, err := c.Zone.ListZones(p)
	if err != nil {
		return result, err
	}
	for _, zone := range r.Zones {
		result = append(result, zone.Id)
	}

	return result, nil
}
