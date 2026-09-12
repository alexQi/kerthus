// Package micro wires explicit go-micro transports and service discovery.
package micro

import (
	"go-micro.dev/v6/client"
	cg "go-micro.dev/v6/client/grpc"
	"go-micro.dev/v6/registry"
	"go-micro.dev/v6/registry/consul"
	"go-micro.dev/v6/selector"
	"go-micro.dev/v6/server"
	sg "go-micro.dev/v6/server/grpc"
	"kerthus/internal/platform/config"
	"time"
)

const ServiceName = "kerthus.saas"

func Registry(c config.Config) registry.Registry {
	return consul.NewConsulRegistry(registry.Addrs(c.ConsulAddress))
}
func Client(c config.Config) (client.Client, error) {
	reg := Registry(c)
	tls, e := c.RPCClientTLS()
	if e != nil {
		return nil, e
	}
	opts := []client.Option{client.Registry(reg), client.Selector(selector.NewSelector(selector.Registry(reg))), client.RequestTimeout(10 * time.Second), client.Retries(0)}
	if tls != nil {
		opts = append(opts, cg.AuthTLS(tls))
	}
	return cg.NewClient(opts...), nil
}
func Server(c config.Config) (server.Server, error) {
	tls, e := c.RPCServerTLS()
	if e != nil {
		return nil, e
	}
	opts := []server.Option{server.Name(ServiceName), server.Version("1.0.0"), server.Address(c.RPCAddress), server.Registry(Registry(c)), server.RegisterTTL(30 * time.Second), server.RegisterInterval(10 * time.Second), sg.MaxMsgSize(12 << 20), sg.GracefulStopTimeout(10 * time.Second)}
	if tls != nil {
		opts = append(opts, sg.AuthTLS(tls))
	}
	return sg.NewServer(opts...), nil
}
