package utilhandlers

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_api_v3_auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
)

const (
	ClusterName  = "example_proxy_cluster"
	RouteName    = "local_route"
	ListenerName = "listener_0"
	ListenerPort = 10000
	UpstreamHost = "www.envoyproxy.io"
	UpstreamPort = 80
)

var (
	newSnapCache cachev3.SnapshotCache
	version      int32
)

// TODO: update makeCluster
func makeCluster(clusterName string) *cluster.Cluster {
	return &cluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment:       makeEndpoint(clusterName),
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
	}
}

// TODO: update makeEndpoint
func makeEndpoint(clusterName string) *endpoint.ClusterLoadAssignment {
	return &endpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: []*endpoint.LbEndpoint{{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.SocketAddress_TCP,
									Address:  UpstreamHost,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: UpstreamPort,
									},
								},
							},
						},
					},
				},
			}},
		}},
	}
}

// TODO: update makeRoute
func makeRoute(routeName string, clusterName string) *route.RouteConfiguration {
	return &route.RouteConfiguration{
		Name: routeName,
		VirtualHosts: []*route.VirtualHost{{
			Name:    "local_service",
			Domains: []string{"*"},
			Routes: []*route.Route{{
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				Action: &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{
							Cluster: clusterName,
						},
						HostRewriteSpecifier: &route.RouteAction_HostRewriteLiteral{
							HostRewriteLiteral: UpstreamHost,
						},
					},
				},
			}},
		}},
	}
}

// TODO: update makeHTTPListener
func makeHTTPListener(listenerName string, route string) *listener.Listener {
	// HTTP filter configuration
	httpConnManager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource:    makeConfigSource(),
				RouteConfigName: route,
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: wellknown.Router,
		}},
	}
	pbst, err := ptypes.MarshalAny(httpConnManager)
	if err != nil {
		panic(err)
	}

	return &listener.Listener{
		Name: listenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: ListenerPort,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}
}

// TODO: update makeConfigSource
func makeConfigSource() *core.ConfigSource {
	source := &core.ConfigSource{}
	source.ResourceApiVersion = resource.DefaultAPIVersion
	source.ConfigSourceSpecifier = &core.ConfigSource_ApiConfigSource{
		ApiConfigSource: &core.ApiConfigSource{
			TransportApiVersion:       resource.DefaultAPIVersion,
			ApiType:                   core.ApiConfigSource_GRPC,
			SetNodeOnFirstMessageOnly: true,
			GrpcServices: []*core.GrpcService{{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: "xds_cluster"},
				},
			}},
		},
	}
	return source
}

// GenerateSnapshot is a function to generate config snapshot
// ref: https://pkg.go.dev/github.com/envoyproxy/go-control-plane@v0.9.9/pkg/cache/v3#NewSnapshot
func GenerateSnapshot() (cachev3.Snapshot, cachev3.SnapshotCache) {
	// new snapshot cache
	newSnapCache = cachev3.NewSnapshotCache(true, cachev3.IDHash{}, nil)

	// create cluster
	// Note: can pass node ID into generator by flag
	var nodeID string
	statusKeys := newSnapCache.GetStatusKeys()
	if len(statusKeys) > 0 {
		nodeID = statusKeys[0]
	} else {
		nodeHostname, err := os.Hostname()
		if err != nil {
			panic(err)
		}
		nodeID = nodeHostname
	}

	clusterName := "api_service_v1"

	// upstream tls context
	uctx := &envoy_api_v3_auth.UpstreamTlsContext{}
	tctx, err := ptypes.MarshalAny(uctx)
	if err != nil {
		log.Fatal(err)
	}
	newCluster := []types.Resource{
		&cluster.Cluster{
			Name:                 clusterName,
			ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
			ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
			DnsLookupFamily:      cluster.Cluster_V4_ONLY,
			LbPolicy:             cluster.Cluster_LEAST_REQUEST,
			LoadAssignment: &endpoint.ClusterLoadAssignment{
				ClusterName: clusterName,
				Endpoints: []*endpoint.LocalityLbEndpoints{{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Protocol: core.SocketAddress_TCP,
												Address:  "173.11.0.21",
												PortSpecifier: &core.SocketAddress_PortValue{
													PortValue: uint32(443),
												},
											},
										},
									},
								}},
						},
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Protocol: core.SocketAddress_TCP,
												Address:  "173.11.0.22",
												PortSpecifier: &core.SocketAddress_PortValue{
													PortValue: uint32(443),
												},
											},
										},
									},
								}},
						},
					},
				}},
			},
			TransportSocket: &core.TransportSocket{
				Name: "envoy.transport_sockets.tls",
				ConfigType: &core.TransportSocket_TypedConfig{
					TypedConfig: tctx,
				},
			},
		},
	}

	// create listener
	listenerName := "https_listener"
	targetPrefix := "/api/v1"
	virtualHostName := "api_servers"
	routeConfigName := "service_route"

	// route_config
	rte := &route.RouteConfiguration{
		Name: routeConfigName,
		VirtualHosts: []*route.VirtualHost{{
			Name:    virtualHostName,
			Domains: []string{"*"},
			Routes: []*route.Route{{
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{
						Prefix: targetPrefix,
					},
				},
				Action: &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{
							Cluster: clusterName,
						},
					},
				},
			}},
		}},
	}

	// filters
	// http_connection_manager
	httpConnManager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "ingress_http",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: rte,
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: wellknown.Router,
		}},
	}

	// SDS
	pbst, err := ptypes.MarshalAny(httpConnManager)
	if err != nil {
		log.Fatal(err)
	}
	crtSDS, err := ioutil.ReadFile("atai-dynamic-config.com.crt")
	if err != nil {
		log.Fatal(err)
	}
	keySDS, err := ioutil.ReadFile("atai-dynamic-config.com.key")
	if err != nil {
		log.Fatal(err)
	}

	// sdsTLS
	/* ref:
	- https://github.com/envoyproxy/go-control-plane/blob/aae09fc4f10139abdbd47dd9ef67d59490319690/pkg/test/resource/v3/secret.go#L118
	*/
	sdsTLS := &envoy_api_v3_auth.DownstreamTlsContext{
		CommonTlsContext: &envoy_api_v3_auth.CommonTlsContext{
			AlpnProtocols: []string{"h2,http/1.1"},
			TlsCertificates: []*envoy_api_v3_auth.TlsCertificate{{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(crtSDS)},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(keySDS)},
				},
			}},
			ValidationContextType: &envoy_api_v3_auth.CommonTlsContext_ValidationContext{
				ValidationContext: &envoy_api_v3_auth.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte("ca-certificates.crt")},
					},
				},
			},
		},
	}
	scfg, err := ptypes.MarshalAny(sdsTLS)
	if err != nil {
		log.Fatal(err)
	}
	listenerOfHTTPS := []types.Resource{
		&listener.Listener{
			Name: listenerName,
			Address: &core.Address{
				Address: &core.Address_SocketAddress{
					SocketAddress: &core.SocketAddress{
						Protocol: core.SocketAddress_TCP,
						Address:  "0.0.0.0",
						PortSpecifier: &core.SocketAddress_PortValue{
							PortValue: uint32(443),
						},
					},
				},
			},
			FilterChains: []*listener.FilterChain{{
				Filters: []*listener.Filter{{
					Name: wellknown.HTTPConnectionManager,
					ConfigType: &listener.Filter_TypedConfig{
						TypedConfig: pbst,
					},
				}},
				TransportSocket: &core.TransportSocket{
					Name: "envoy.transport_sockets.tls",
					ConfigType: &core.TransportSocket_TypedConfig{
						TypedConfig: scfg,
					},
				},
			}},
		}}

	secretConfig := []types.Resource{
		&envoy_api_v3_auth.Secret{
			Name: "server_cert",
			Type: &envoy_api_v3_auth.Secret_TlsCertificate{
				TlsCertificate: &envoy_api_v3_auth.TlsCertificate{
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(crtSDS)},
					},
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(keySDS)},
					},
				},
			},
		},
	}
	atomic.AddInt32(&version, 1)

	var snap cachev3.Snapshot
	snap = cachev3.NewSnapshot(fmt.Sprint(version), nil, newCluster, nil, listenerOfHTTPS, nil, secretConfig)

	if errCacheConsistancy := snap.Consistent(); errCacheConsistancy != nil {
		log.Fatalf("snapshot inconsistency: %+v\n%+v", snap, errCacheConsistancy)
		os.Exit(1)
	}
	// ref: https://pkg.go.dev/github.com/envoyproxy/go-control-plane@v0.9.9/pkg/cache/v3#SnapshotCache.SetSnapshot
	errSetSnapshot := newSnapCache.SetSnapshot(nodeID, snap)
	if errSetSnapshot != nil {
		log.Fatalf("Could not set snapshot %v", errSetSnapshot)
	}
	return snap, newSnapCache
}