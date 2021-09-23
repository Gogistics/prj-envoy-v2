package utilhandlers

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
)

const (
	grpcKeepaliveTime         = 30 * time.Second
	grpcKeepaliveTimeout      = 15 * time.Second
	grpcMaxConnectionAge      = 120 * time.Second
	grpcMaxConnectionAgeGrace = 30 * time.Second
	grpcKeepaliveMinTime      = 30 * time.Second
	grpcMaxConcurrentStreams  = 10000
)

var (
	port    = 20000
	crtFile = "atai-dynamic-config.com.crt"
	keyFile = "atai-dynamic-config.com.key"
)

func registerServer(grpcServer *grpc.Server, server serverv3.Server) {
	// register services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, server)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, server)
}

// RunServer starts an xDS server at the given port.
func RunServer(port uint) {
	/* init callbacks
	- signal: channel for sending info. back to server handler
	*/
	signal := make(chan struct{})
	cb := &Callbacks{
		Signal:   signal,
		Fetches:  0,
		Requests: 0,
		Debug:    true,
	}

	snapshot, cache := GenerateSnapshot()
	if err := snapshot.Consistent(); err != nil {
		log.Fatalf("snapshot inconsistency: %+v\n%+v", snapshot, err)
		os.Exit(1)
	}
	log.Printf("will serve snapshot %+v", snapshot)

	ctx := context.Background()
	// Run xDS server
	// ref: https://pkg.go.dev/github.com/envoyproxy/go-control-plane@v0.9.9/pkg/server/v3#NewServer
	srv := serverv3.NewServer(ctx, cache, cb)

	// set certs
	creds, err := credentials.NewServerTLSFromFile(crtFile, keyFile)
	if err != nil {
		log.Fatalf("Failed to generate credentials %v", err)
	}
	// construct grpc options
	var grpcServerOptions []grpc.ServerOption
	grpcServerOptions = append(grpcServerOptions,
		grpc.Creds(creds),
		grpc.NumStreamWorkers(50),
		grpc.MaxHeaderListSize(10240),
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:                  grpcKeepaliveTime,
			Timeout:               grpcKeepaliveTimeout,
			MaxConnectionAge:      grpcMaxConnectionAge,
			MaxConnectionAgeGrace: grpcMaxConnectionAgeGrace,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
	)
	grpcServer := grpc.NewServer(grpcServerOptions...)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	registerServer(grpcServer, srv)
	log.Printf("Resource management server listening on %d\n", port)
	if err = grpcServer.Serve(lis); err != nil {
		log.Println(err)
	}
}
