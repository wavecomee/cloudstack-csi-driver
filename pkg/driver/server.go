package driver

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

func (cs *cloudstackDriver) serve(ctx context.Context, ids csi.IdentityServer, ctrls csi.ControllerServer, ns csi.NodeServer) error {
	logger := klog.FromContext(ctx)
	proto, addr, err := parseEndpoint(cs.endpoint)
	if err != nil {
		return err
	}

	if proto == "unix" {
		if !strings.HasPrefix(addr, "/") {
			addr = "/" + addr
		}
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s, error: %s", addr, err.Error())
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Log every request and payloads (request + response)
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			resp, err := handler(klog.NewContext(ctx, logger), req)
			if err != nil {
				logger.Error(err, "GRPC method failed", "method", info.FullMethod)
			}

			return resp, err
		}),
	}
	grpcServer := grpc.NewServer(opts...)

	if ids != nil {
		csi.RegisterIdentityServer(grpcServer, ids)
	}
	if ctrls != nil {
		csi.RegisterControllerServer(grpcServer, ctrls)
	}
	if ns != nil {
		csi.RegisterNodeServer(grpcServer, ns)
	}

	logger.Info("Listening for connections", "address", listener.Addr())

	return grpcServer.Serve(listener)
}

func parseEndpoint(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") || strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}

	return "", "", fmt.Errorf("invalid endpoint: %v", ep)
}
