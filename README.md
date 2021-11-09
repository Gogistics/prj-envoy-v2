# Dynamic configuration xDS
An xDS management server is a mechanism that supports dynamic bootstrap configuration and its APIs are defined as proto3 Protocol Buffers in the api tree. In oreder to build our own xDS service, certain basic knowledge are required. First we need to know how many xDS types are supported in v3 and the variants of the xDS Transport protocol.

The v3 xDS types supported by Envoy are:
* envoy.config.listener.v3.Listener
* envoy.config.route.v3.RouteConfiguration
* envoy.config.route.v3.ScopedRouteConfiguration
* envoy.config.route.v3.VirtualHost
* envoy.config.cluster.v3.Cluster
* envoy.config.endpoint.v3.ClusterLoadAssignment
* envoy.extensions.transport_sockets.tls.v3.Secret
* envoy.service.runtime.v3.Runtime

Four variants of the xDS transport protocol are:
1. State of the World (Basic xDS): SotW, separate gRPC stream for each resource type
2. Incremental xDS: incremental, separate gRPC stream for each resource type
3. Aggregated Discovery Service (ADS): SotW, aggregate stream for all resource types
4. Incremental ADS: incremental, aggregate stream for all resource types

Ref:
- [xDS REST and gRPC protocol — envoy 1.20.0-dev-6f2726 documentation](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [Integrating the Envoy gRPC API into a Dynamic Service Discovery Platform](https://youtu.be/tTaFcZVqbbY)
- [Eventual consistency](https://en.wikipedia.org/wiki/Eventual_consistency)
- [IPC - Synchronous Communication (part 1/3) : Remote Procedure Invocation pattern](https://youtu.be/y4c5t85av7o)


## Context
Dynamically update Envoy by the control plane and what we are going to test are as follows:
* Bring up the front Envoy without API clusters configuration. Test the API services and will get error.
* Send new configuration to the control plane, and it will forward the configuration to the Envoy. Test the API service 1. and will receive the response.
* Send new configuration to the control plane, and it will forward the configuration to the Envoy. Test the API service 2. and will receive the response.
* Stop the control plane and all services should still work fine.

## Steps of implementing dynamic configuration
1. Draw the [diagram of the container topology](https://drive.google.com/file/d/1ejx5Ap5PRk9eLVTmWkykfbR9FIDrGaOU/view?usp=sharing)
2. Create the folders for resource management
3. Install required tools such as Docker and Bazel

## Development steps
Step 1. Init networks and generate certs
Step 2. Start Envoy front proxy and check the Envoy dynamic active clusters
Step 3. Build xDS server and run the server
Step 4. Build API service
Step 5. Check the Envoy dynamic active clusters and test the API endpoints

1. Init networks and generate certs
```sh
# init network
$ ./utils/scripts/init_networks.sh

# generate certs
$ cd utils/

# create a cert authority
$ openssl genrsa -out certs/ca.key 4096
# Generating RSA private key, 4096 bit long modulus
# .....++
# ...............................................................................++
# e is 65537 (0x10001)

$ openssl req -x509 -new -nodes -key certs/ca.key -sha256 -days 1024 -out certs/ca.crt
# You are about to be asked to enter information that will be incorporated
# into your certificate request.
# What you are about to enter is what is called a Distinguished Name or a DN.
# There are quite a few fields but you can leave some blank
# For some fields there will be a default value,
# If you enter '.', the field will be left blank.
# -----
# Country Name (2 letter code) []:TW    
# State or Province Name (full name) []:
# Locality Name (eg, city) []:Kaohsiung
# Organization Name (eg, company) []:Gogistics
# Organizational Unit Name (eg, section) []:DevOps
# Common Name (eg, fully qualified host name) []:atai-dynamic-config.com
# Email Address []:gogistics@gogistics-tw.com
# Generating RSA private key, 2048 bit long modulus
# ...............................................................+++
# ..............................................+++
# e is 65537 (0x10001)
# \create a cert authority

# create a domain key
$ openssl genrsa -out certs/atai-dynamic-config.com.key 2048
# Generating RSA private key, 2048 bit long modulus
# ...............................................................+++
# ..............................................+++
# e is 65537 (0x10001)

# generate signing requests for proxy and app
$ openssl req -new -sha256 \
     -key certs/atai-dynamic-config.com.key \
     -subj "/C=US/ST=CA/O=GOGISTICS, Inc./CN=atai-dynamic-config.com" \
     -out certs/atai-dynamic-config.com.csr
# \generate signing requests for proxy and app

# generate certificates for proxy and app
$ openssl x509 -req \
     -in certs/atai-dynamic-config.com.csr \
     -CA certs/ca.crt \
     -CAkey certs/ca.key \
     -CAcreateserial \
     -extfile <(printf "subjectAltName=DNS:atai-dynamic-config.com") \
     -out certs/atai-dynamic-config.com.crt \
     -days 500 \
     -sha256
# \generate certificates for proxy and app
```

2. Start Envoy front proxy and check the Envoy dynamic active clusters
```sh
# run the gazelle target specified in the BUILD file
$ bazel run //:gazelle

# update repos deps
$ bazel run //:gazelle -- update-repos -from_file=go.mod -to_macro=deps.bzl%go_dependencies

# build Docker image
$ bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //envoys:front-proxy-v0.0.0
$ bazel run --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //envoys:front-proxy-v0.0.0

$ docker run -d \
      --name atai_front_proxy \
      -p 80:80 -p 443:443 -p 8001:8001 \
      --network atai_apis_network \
      --ip "173.11.0.10" \
      --log-opt mode=non-blocking \
      --log-opt max-buffer-size=5m \
      --log-opt max-size=100m \
      --log-opt max-file=5 \
      alantai/prj-envoy-v2/envoys:front-proxy-v0.0.0

$ docker network connect atai_control_mechanism atai_front_proxy

# if jq is available
$ curl -s http://0.0.0.0:8001/config_dump  | jq '.configs[1].static_clusters'
# Or
$ curl -s http://0.0.0.0:8001/config_dump | grep -A 35 -B 1 static_cluster
#    "@type": "type.googleapis.com/envoy.admin.v3.ClustersConfigDump",
#    "static_clusters": [
#     {
#      "cluster": {
#       "@type": "type.googleapis.com/envoy.api.v2.Cluster",
#       "name": "xds_cluster",
#       "type": "STRICT_DNS",
#       "load_assignment": {
#        "cluster_name": "xds_cluster",
#        "endpoints": [
#         {
#          "lb_endpoints": [
#           {
#            "endpoint": {
#             "address": {
#              "socket_address": {
#               "address": "173.10.0.22",
#               "port_value": 443
#              }
#             }
#            }
#           }
#          ]
#         }
#        ]
#       },
#       "typed_extension_protocol_options": {
#        "envoy.extensions.upstreams.http.v3.HttpProtocolOptions": {
#         "@type": "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
#         "explicit_http_config": {
#          "http2_protocol_options": {}
#         }
#        }
#       }
#      },
#      "last_updated": "2021-09-12T02:08:02.592Z"
#     }
$ curl -s http://0.0.0.0:8001/config_dump | grep -A 10 -B 1 dynamic_active_clusters
```

3. Build xDS server and run the server
All source code of control plane servers are under **/control-mechanism/control-plane**

```sh
# test gRPC and control plane servers
$  docker run --name atai-go-dev \
     --network atai_control_mechanism \
     --ip "173.10.0.22" \
     -p 20000:20000 \
     -v $(pwd):/prj \
     -w /prj \
     -it \
     --rm \
     golang:latest bash

# write Bazel build to create a Docker image of the control plane
# build Docker image of the control plane by Bazel
# Note: currently this step needs workarounds because of the issues of com_github_envoyproxy_protoc_gen_validate and com_github_census_instrumentation_opencensus_proto
# Solutions:
# - https://github.com/bazelbuild/bazel-gazelle/issues/988#issuecomment-908973994
# - https://github.com/census-instrumentation/opencensus-proto/issues/200#issuecomment-622610454
# Once the workarounds have been applied, run the commands below:
$ bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //control-mechanism/control-plane:control-plane-v0.0.0
$ bazel run --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //control-mechanism/control-plane:control-plane-v0.0.0

# bring up a container running control plane
$ docker run -itd \
     --name atai_grpc_control_plane \
     --network atai_control_mechanism \
     --ip "173.10.0.22" \
     --log-opt mode=non-blocking \
     --log-opt max-buffer-size=5m \
     --log-opt max-size=100m \
     --log-opt max-file=5 \
     alantai/prj-envoy-v2/control-mechanism/control-plane:control-plane-v0.0.0

# check the logs of the generated container
$ docker logs atai_grpc_control_plane
# 2021/09/20 17:15:07 will serve snapshot {Resources:[{Version:1 Items:map[]} {Version:1 Items:map[api_service_v1:{Resource:name:"api_service_v1" type:LOGICAL_DNS connect_timeout:{seconds:5} lb_policy:LEAST_REQUEST load_assignment:{cluster_name:"api_service_v1" endpoints:{lb_endpoints:{endpoint:{address:{socket_address:{address:"173.11.0.21" port_value:443}}}} lb_endpoints:{endpoint:{address:{socket_address:{address:"173.11.0.22" port_value:443}}}}}} dns_lookup_family:V4_ONLY transport_socket:{name:"envoy.transport_sockets.tls" typed_config:{[type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext]:{}}} Ttl:<nil>}]} {Version:1 Items:map[]} {Version:1 Items:map[https_listener:{Resource:name:"https_listener" address:{socket_address:{address:"0.0.0.0" port_value:443}} filter_chains:{filters:{name:"envoy.filters.network.http_connection_manager" typed_config:{[type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager]:{stat_prefix:"ingress_http" route_config:{name:"service_route" virtual_hosts:{name:"api_servers" domains:"*" routes:{match:{prefix:"/api/v1"} route:{cluster:"api_service_v1"}}}} http_filters:{name:"envoy.filters.http.router"}}}} transport_socket:{name:"envoy.transport_sockets.tls" typed_config:{[type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext]:{common_tls_context:{tls_certificates:{certificate_chain:{inline_bytes:"-----BEGIN CERTIFICATE-----\nMIIEkzCCAnugAwIBAgIJAPDmgHLPbKVZMA0GCSqGSIb3DQEBCwUAMIGTMQswCQYD\nVQQGEwJUVzESMBAGA1UEBwwJS2FvaHNpdW5nMRIwEAYDVQQKDAlHb2dpc3RpY3Mx\nDzANBgNVBAsMBkRldk9wczEgMB4GA1UEAwwXYXRhaS1keW5hbWljLWNvbmZpZy5j\nb20xKTAnBgkqhkiG9w0BCQEWGmdvZ2lzdGljc0Bnb2dpc3RpY3MtdHcuY29tMB4X\nDTIxMDkxMjAxMzMxM1oXDTIzMDEyNTAxMzMxM1owVjELMAkGA1UEBhMCVVMxCzAJ\nBgNVBAgMAkNBMRgwFgYDVQQKDA9HT0dJU1RJQ1MsIEluYy4xIDAeBgNVBAMMF2F0\nYWktZHluYW1pYy1jb25maWcuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB\nCgKCAQEAnnZL4aBrrryzx5DhDeXs8fZOu2C08q+mUEjYvJMmar0XyU4+9EtHlD4x\nWYRGOeA9lML7b7/NVbyQzdHcrrBrScgpdBCjK8AmRYng42mpi/cNNwEzJMo3fpNf\n+oiLJ1ykxwsjCGjstJmklwuy1Df0D6ql8gX9oMShbrNz+agoRFduB6XV6GA+kpEu\nSuJULQP20RblEISffx2X8LabaW3vO7Io2k20gTWgYAxTkAtpIBcHgcImJG4PI+FK\nwyZLDOkajDYWF7BiAHhYxNYN+PabaSmT7o+DXtOSd13Y0bpW3fcMCtaCdL0X14oH\nq4YHHUdVxPnfKkK0NSUKLAHi3LHmHQIDAQABoyYwJDAiBgNVHREEGzAZghdhdGFp\nLWR5bmFtaWMtY29uZmlnLmNvbTANBgkqhkiG9w0BAQsFAAOCAgEARXREWrZUYC+n\ndaMxgNPQ0O1eGKJbUyagpcGhDuJ5S4ekOTvk521mUnQ+lgkoV1xxcRAQv/LR1pql\nNy0R/2qMnRnabT++KGVh8ldCwqZjL8gW3syYyaCs1hill0mHOqSLz7Vs3q86S44J\nnhSdUmhPtHJjRNV2y+g19HicC7d244wx67GPL+h4hAXbB6TOg4860AVTNWMC9txW\nw2DoXSdmyFXkXw3nvj6LbgBpKf38pb7KCx8d0R/FVJ9M3j5cJwUAB+x55NCGO/iu\nARPG33MGYmSZGfECkPUalSBNDrkTxqHfOx1/LunyP2P1OSZeUZuPhDrwDKOmfob9\nay/boaal2kutF6HLmj+aE50LdTioVtXxI+E5KMPlb7QZ85FbfzM83hm2rnZVM0eX\nC4u3PYJyTsflrBc9PltsT/bDimXqShi1VrxlBQwFw2RcVX/dgiI491bur0OgE3Ho\ng0gYFqFgaz9bnuMqV+y6KJVrh3qB50AT3E2IU9PIX0HXMyVqp+XZ5D7UU5E9/nfe\nzn0ITDtyiern1gAlCExkn4OKT85yN7W9Je6bfCzRCN4sGZjnpn3vS1oxOf1NZUH4\nBqJnXtZxeFdoFxw/uhBuuk+GElNnVH9HDjY2PFCYBnsj7ba9HSnNOPUPkcqvR2PA\nCkHaiCDu9l3CdAyrWewXQeS/VZLSSh4=\n-----END CERTIFICATE-----\n"} private_key:{inline_bytes:"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAnnZL4aBrrryzx5DhDeXs8fZOu2C08q+mUEjYvJMmar0XyU4+\n9EtHlD4xWYRGOeA9lML7b7/NVbyQzdHcrrBrScgpdBCjK8AmRYng42mpi/cNNwEz\nJMo3fpNf+oiLJ1ykxwsjCGjstJmklwuy1Df0D6ql8gX9oMShbrNz+agoRFduB6XV\n6GA+kpEuSuJULQP20RblEISffx2X8LabaW3vO7Io2k20gTWgYAxTkAtpIBcHgcIm\nJG4PI+FKwyZLDOkajDYWF7BiAHhYxNYN+PabaSmT7o+DXtOSd13Y0bpW3fcMCtaC\ndL0X14oHq4YHHUdVxPnfKkK0NSUKLAHi3LHmHQIDAQABAoIBAHViZGvLjnluyC65\noD3PaWsEbuZXiTON8sHedM+cogTH9urkz7XgXjHusFgDqJIPDw84MVJi3xT4Dryp\nDbVKcu/BGxQjjvxF5xP0Q2ezSimo5V0twlkqg1l8isjohUyvUFEyas08DLzsZASQ\nYfTbTiyc2TkkPvHtNzjuLqdubgXRI8cVUXPgI81Ef6U4ml92/arp6CfXCaXGsYuW\ns46+iTuA+KyEtA6vUgGVCQqP3+0jHvD1DxyBqELDU9anTXf4oUhB4W6TPWq4F5v4\no7oPKe3bz3LlCx8gnP2Khd6T7j0h8GLFGmpgrKerEI3VATWd1HOVEsq617DOZjiN\nPIGlsnECgYEA0bryxE00FYdeGBP9MpgysAz2W9vcNAGveo0OzL1GG32taBJ40U/u\n27rl7hsb6ptOu/uQhDIugNj6Oj0+JhRTJiOKNSNNoZ4CDtwnKvJunC4Xtw86uA63\nxD1+IbcDS5AvbP0JJB+3w7bEPVZMiX69P5F1rR3McDYBFadnS/NQ7uMCgYEAwWvY\n7VIZfsGgksG5iPn9u8cE3hKj0FmiYrq+yBWAb9u/VivNi1MBtSMpa3hjzm7UgdZI\nPlFp59QBldFA/EmR62BoVOBzI/O9S4L5Qrqq0iEosvfmw9L9OIVjQFj947/ufKCO\nYURHNKf6KXSaHuEBzAhPlAZF83I10122TKv85v8CgYEAlqRiNU+CxqfplP/ekNWz\nKrLUzVwZSZ2gTjU9WR/mWF6oDCWgdC+m0FrpRmJgZd3R6sIhpmJo9pFjAiv1FOLq\nam2CmvJVk21r6wKEe5uQiUuuKwWcVpHzute0XkEW89KHzg/d3f2OP9xqDeiLpwLK\nqfsv+/14V2zi0IvibTJCgqMCgYBXyw78sX43BcZPtrTzUp10BSLVddp7MKQ/cgo0\noWXZ4AGaKGm0qqmkwWAEkvGierXkdRH3j1alzpolmYSIvxAHqYvRssswb2rlgn6H\nZlkw5bImgdVx3yvm4sypIXukS7MBSJM33RkA8pnfBTkLeRAqvz73rl1D4fxCg0/C\nv3IcmwKBgHo/RzeB+jkNWEJ3RqePEo1VoEwHdYP29bwv4D83XsLtxgg6ILoag6Eb\ntE/8bBgYoJjfcAbDnRd0xaxlIju8qiFaqBwjMIGz2OyxZxiUi7AlEgSI/vMuT2yy\ndt6kFaMwT5PYM1Lc5Es4S1DBqiOSU/gZ+Q9SPXs3z4OTRXNK+6/f\n-----END RSA PRIVATE KEY-----\n"}} validation_context:{trusted_ca:{inline_bytes:"ca-certificates.crt"}} alpn_protocols:"h2,http/1.1"}}}}} Ttl:<nil>}]} {Version:1 Items:map[server_cert:{Resource:name:"server_cert" tls_certificate:{certificate_chain:{inline_bytes:"-----BEGIN CERTIFICATE-----\nMIIEkzCCAnugAwIBAgIJAPDmgHLPbKVZMA0GCSqGSIb3DQEBCwUAMIGTMQswCQYD\nVQQGEwJUVzESMBAGA1UEBwwJS2FvaHNpdW5nMRIwEAYDVQQKDAlHb2dpc3RpY3Mx\nDzANBgNVBAsMBkRldk9wczEgMB4GA1UEAwwXYXRhaS1keW5hbWljLWNvbmZpZy5j\nb20xKTAnBgkqhkiG9w0BCQEWGmdvZ2lzdGljc0Bnb2dpc3RpY3MtdHcuY29tMB4X\nDTIxMDkxMjAxMzMxM1oXDTIzMDEyNTAxMzMxM1owVjELMAkGA1UEBhMCVVMxCzAJ\nBgNVBAgMAkNBMRgwFgYDVQQKDA9HT0dJU1RJQ1MsIEluYy4xIDAeBgNVBAMMF2F0\nYWktZHluYW1pYy1jb25maWcuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB\nCgKCAQEAnnZL4aBrrryzx5DhDeXs8fZOu2C08q+mUEjYvJMmar0XyU4+9EtHlD4x\nWYRGOeA9lML7b7/NVbyQzdHcrrBrScgpdBCjK8AmRYng42mpi/cNNwEzJMo3fpNf\n+oiLJ1ykxwsjCGjstJmklwuy1Df0D6ql8gX9oMShbrNz+agoRFduB6XV6GA+kpEu\nSuJULQP20RblEISffx2X8LabaW3vO7Io2k20gTWgYAxTkAtpIBcHgcImJG4PI+FK\nwyZLDOkajDYWF7BiAHhYxNYN+PabaSmT7o+DXtOSd13Y0bpW3fcMCtaCdL0X14oH\nq4YHHUdVxPnfKkK0NSUKLAHi3LHmHQIDAQABoyYwJDAiBgNVHREEGzAZghdhdGFp\nLWR5bmFtaWMtY29uZmlnLmNvbTANBgkqhkiG9w0BAQsFAAOCAgEARXREWrZUYC+n\ndaMxgNPQ0O1eGKJbUyagpcGhDuJ5S4ekOTvk521mUnQ+lgkoV1xxcRAQv/LR1pql\nNy0R/2qMnRnabT++KGVh8ldCwqZjL8gW3syYyaCs1hill0mHOqSLz7Vs3q86S44J\nnhSdUmhPtHJjRNV2y+g19HicC7d244wx67GPL+h4hAXbB6TOg4860AVTNWMC9txW\nw2DoXSdmyFXkXw3nvj6LbgBpKf38pb7KCx8d0R/FVJ9M3j5cJwUAB+x55NCGO/iu\nARPG33MGYmSZGfECkPUalSBNDrkTxqHfOx1/LunyP2P1OSZeUZuPhDrwDKOmfob9\nay/boaal2kutF6HLmj+aE50LdTioVtXxI+E5KMPlb7QZ85FbfzM83hm2rnZVM0eX\nC4u3PYJyTsflrBc9PltsT/bDimXqShi1VrxlBQwFw2RcVX/dgiI491bur0OgE3Ho\ng0gYFqFgaz9bnuMqV+y6KJVrh3qB50AT3E2IU9PIX0HXMyVqp+XZ5D7UU5E9/nfe\nzn0ITDtyiern1gAlCExkn4OKT85yN7W9Je6bfCzRCN4sGZjnpn3vS1oxOf1NZUH4\nBqJnXtZxeFdoFxw/uhBuuk+GElNnVH9HDjY2PFCYBnsj7ba9HSnNOPUPkcqvR2PA\nCkHaiCDu9l3CdAyrWewXQeS/VZLSSh4=\n-----END CERTIFICATE-----\n"} private_key:{inline_bytes:"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAnnZL4aBrrryzx5DhDeXs8fZOu2C08q+mUEjYvJMmar0XyU4+\n9EtHlD4xWYRGOeA9lML7b7/NVbyQzdHcrrBrScgpdBCjK8AmRYng42mpi/cNNwEz\nJMo3fpNf+oiLJ1ykxwsjCGjstJmklwuy1Df0D6ql8gX9oMShbrNz+agoRFduB6XV\n6GA+kpEuSuJULQP20RblEISffx2X8LabaW3vO7Io2k20gTWgYAxTkAtpIBcHgcIm\nJG4PI+FKwyZLDOkajDYWF7BiAHhYxNYN+PabaSmT7o+DXtOSd13Y0bpW3fcMCtaC\ndL0X14oHq4YHHUdVxPnfKkK0NSUKLAHi3LHmHQIDAQABAoIBAHViZGvLjnluyC65\noD3PaWsEbuZXiTON8sHedM+cogTH9urkz7XgXjHusFgDqJIPDw84MVJi3xT4Dryp\nDbVKcu/BGxQjjvxF5xP0Q2ezSimo5V0twlkqg1l8isjohUyvUFEyas08DLzsZASQ\nYfTbTiyc2TkkPvHtNzjuLqdubgXRI8cVUXPgI81Ef6U4ml92/arp6CfXCaXGsYuW\ns46+iTuA+KyEtA6vUgGVCQqP3+0jHvD1DxyBqELDU9anTXf4oUhB4W6TPWq4F5v4\no7oPKe3bz3LlCx8gnP2Khd6T7j0h8GLFGmpgrKerEI3VATWd1HOVEsq617DOZjiN\nPIGlsnECgYEA0bryxE00FYdeGBP9MpgysAz2W9vcNAGveo0OzL1GG32taBJ40U/u\n27rl7hsb6ptOu/uQhDIugNj6Oj0+JhRTJiOKNSNNoZ4CDtwnKvJunC4Xtw86uA63\nxD1+IbcDS5AvbP0JJB+3w7bEPVZMiX69P5F1rR3McDYBFadnS/NQ7uMCgYEAwWvY\n7VIZfsGgksG5iPn9u8cE3hKj0FmiYrq+yBWAb9u/VivNi1MBtSMpa3hjzm7UgdZI\nPlFp59QBldFA/EmR62BoVOBzI/O9S4L5Qrqq0iEosvfmw9L9OIVjQFj947/ufKCO\nYURHNKf6KXSaHuEBzAhPlAZF83I10122TKv85v8CgYEAlqRiNU+CxqfplP/ekNWz\nKrLUzVwZSZ2gTjU9WR/mWF6oDCWgdC+m0FrpRmJgZd3R6sIhpmJo9pFjAiv1FOLq\nam2CmvJVk21r6wKEe5uQiUuuKwWcVpHzute0XkEW89KHzg/d3f2OP9xqDeiLpwLK\nqfsv+/14V2zi0IvibTJCgqMCgYBXyw78sX43BcZPtrTzUp10BSLVddp7MKQ/cgo0\noWXZ4AGaKGm0qqmkwWAEkvGierXkdRH3j1alzpolmYSIvxAHqYvRssswb2rlgn6H\nZlkw5bImgdVx3yvm4sypIXukS7MBSJM33RkA8pnfBTkLeRAqvz73rl1D4fxCg0/C\nv3IcmwKBgHo/RzeB+jkNWEJ3RqePEo1VoEwHdYP29bwv4D83XsLtxgg6ILoag6Eb\ntE/8bBgYoJjfcAbDnRd0xaxlIju8qiFaqBwjMIGz2OyxZxiUi7AlEgSI/vMuT2yy\ndt6kFaMwT5PYM1Lc5Es4S1DBqiOSU/gZ+Q9SPXs3z4OTRXNK+6/f\n-----END RSA PRIVATE KEY-----\n"}} Ttl:<nil>}]} {Version:1 Items:map[]} {Version:1 Items:map[]}] VersionMap:map[]}
# 2021/09/20 17:15:07 Resource management server listening on 20000

# in order to have atai_grpc_control_plane communicate with the frontend Envoy in atai_apis_network, connect atai_grpc_control_plane to atai_apis_network
$ docker network connect atai_apis_network atai_grpc_control_plane

# once the testing has been completed, remove the container
$ docker rm -f atai_grpc_control_plane
```

4. Build API service
Let's start writing API services.

4-1. build the web app in Golang; source codes are under */services/api/v1* and */services/api/v2*
4-2. test the web app by running it inside a container
```sh
$ docker run -it \
     --name atai_api_v1_dev \
     --network atai_apis_network \
     --ip "173.11.0.21" \
     -v $(pwd):/app \
     -w /app \
     --rm \
     golang:1.14.0-alpine sh
$ go run main.go -dev

# open the other terminal and access the container by the following command
$ docker exec -it atai_api_v1_dev sh
# install cURL by the following command
$ apk add --update --no-cache curl

# test the API endpoint
$  curl -k https://0.0.0.0/api/v1
# {"host":"a4efa98bfb18","wd":"/app/services/api-v1"}

# build Docker images by Bazel
$ bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 \
    //services/api-v1:api-v1.0.0.0
$ bazel run --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 \
    //services/api-v1:api-v1.0.0.0

# test the Docker image
$ docker run -d \
    -p 8443:443 \
    --name atai_service_api_v1 \
    --network atai_apis_network \
    --ip "173.11.0.21" \
    --log-opt mode=non-blocking \
    --log-opt max-buffer-size=5m \
    --log-opt max-size=100m \
    --log-opt max-file=5 \
    alantai/prj-envoy-v2/services/api-v1:api-v1.0.0.0
$ curl -k https://0.0.0.0:8443/api/v1
# {"host":"0a481c4ef2e0","wd":"/"}

# once the testing has been completed, remove the container
$ docker rm -f atai_dynamic_control_service_api_v1

# once api-v2 app has been tested, run the following commands to build Docker images of api-v2
$ bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 \
    //services/api-v2:api-v2.0.0.0
$ bazel run --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 \
    //services/api-v2:api-v2.0.0.0
```

5. Check the Envoy dynamic active clusters and test the API endpoints

5-1. Build Docker images of Envoy proxies (WIP)

5-2. Bring up all components required for testing Envoy xDS
```sh
# bring up Envoy
$ docker run -d \
      --name atai_front_proxy \
      -p 80:80 -p 443:443 -p 8001:8001 \
      --network atai_apis_network \
      --ip "173.11.0.10" \
      --log-opt mode=non-blocking \
      --log-opt max-buffer-size=5m \
      --log-opt max-size=100m \
      --log-opt max-file=5 \
      alantai/prj-envoy-v2/envoys:front-proxy-v0.0.0 && \
      docker network connect atai_control_mechanism atai_front_proxy

# bring up API V1
$ docker run -d \
    --name atai_service_api_v1_1 \
    --network atai_apis_network \
    --ip "173.11.0.21" \
    --log-opt mode=non-blocking \
    --log-opt max-buffer-size=5m \
    --log-opt max-size=100m \
    --log-opt max-file=5 \
    alantai/prj-envoy-v2/services/api-v1:api-v1.0.0.0

$ docker run -d \
    --name atai_service_api_v1_2 \
    --network atai_apis_network \
    --ip "173.11.0.22" \
    --log-opt mode=non-blocking \
    --log-opt max-buffer-size=5m \
    --log-opt max-size=100m \
    --log-opt max-file=5 \
    alantai/prj-envoy-v2/services/api-v1:api-v1.0.0.0

# bring up control plane
$ docker run -itd \
    --name atai_grpc_control_plane \
    --network atai_control_mechanism \
    --ip "173.10.0.22" \
    --log-opt mode=non-blocking \
    --log-opt max-buffer-size=5m \
    --log-opt max-size=100m \
    --log-opt max-file=5 \
    alantai/prj-envoy-v2/control-mechanism/control-plane:control-plane-v0.0.0 && \
    docker network connect atai_apis_network atai_grpc_control_plane

# once all containers are running successfully, run the following command to test the API service
$ curl -k -vvv https://atai-dynamic-config.com/api/v1
# *   Trying 0.0.0.0...
# * TCP_NODELAY set
# * Connected to atai-dynamic-config.com (127.0.0.1) port 443 (#0)
# * ALPN, offering h2
# * ALPN, offering http/1.1
# * successfully set certificate verify locations:
# *   CAfile: /etc/ssl/cert.pem
#   CApath: none
# * TLSv1.2 (OUT), TLS handshake, Client hello (1):
# * TLSv1.2 (IN), TLS handshake, Server hello (2):
# * TLSv1.2 (IN), TLS handshake, Certificate (11):
# * TLSv1.2 (IN), TLS handshake, Server key exchange (12):
# * TLSv1.2 (IN), TLS handshake, Request CERT (13):
# * TLSv1.2 (IN), TLS handshake, Server finished (14):
# * TLSv1.2 (OUT), TLS handshake, Certificate (11):
# * TLSv1.2 (OUT), TLS handshake, Client key exchange (16):
# * TLSv1.2 (OUT), TLS change cipher, Change cipher spec (1):
# * TLSv1.2 (OUT), TLS handshake, Finished (20):
# * TLSv1.2 (IN), TLS change cipher, Change cipher spec (1):
# * TLSv1.2 (IN), TLS handshake, Finished (20):
# * SSL connection using TLSv1.2 / ECDHE-RSA-CHACHA20-POLY1305
# * ALPN, server accepted to use h2
# * Server certificate:
# *  subject: C=US; ST=CA; O=GOGISTICS, Inc.; CN=atai-dynamic-config.com
# *  start date: Sep 12 01:33:13 2021 GMT
# *  expire date: Jan 25 01:33:13 2023 GMT
# *  issuer: C=TW; L=Kaohsiung; O=Gogistics; OU=DevOps; CN=atai-dynamic-config.com; emailAddress=gogistics@gogistics-tw.com
# *  SSL certificate verify result: unable to get local issuer certificate (20), continuing anyway.
# * Using HTTP2, server supports multi-use
# * Connection state changed (HTTP/2 confirmed)
# * Copying HTTP/2 data in stream buffer to connection buffer after upgrade: len=0
# * Using Stream ID: 1 (easy handle 0x7fdc4a80d600)
# > GET /api/v1 HTTP/2
# > Host: atai-dynamic-config.com
# > User-Agent: curl/7.64.1
# > Accept: */*
# > 
# * Connection state changed (MAX_CONCURRENT_STREAMS == 2147483647)!
# < HTTP/2 200 
# < content-type: application/json; charset=utf-8
# < date: Thu, 23 Sep 2021 01:58:34 GMT
# < content-length: 32
# < x-envoy-upstream-service-time: 1
# < server: envoy
# < 
# * Connection #0 to host atai-dynamic-config.com left intact
# {"host":"dac57fdd19f7","wd":"/"}* Closing connection 0
```

Notes: you might encounter some issues of having Envoy talk to xDS.
* gRPC config stream closed: 13 or gRPC config stream closed: 0 in proxy logs, every 30 minutes. This error message is expected, as the connection to Pilot is intentionally closed every 30 minutes.
* gRPC config stream closed: 14 in proxy logs. If this occurs repeatedly it may indicate problems connecting to Pilot. However, a single occurance of this is typical when Envoy is starting or restarting.


## References:
- [On the state of Envoy Proxy control planes](https://mattklein123.dev/2020/03/15/on-the-state-of-envoy-proxy-control-planes/)
- [Build Your Own Envoy Control Plane - Steve Sloka, VMware](https://youtu.be/qAuq4cKEG_E)
- [Hoot: Envoy xDS Dynamic Configuration and Control Plane Interactions](https://youtu.be/S5Fm1Yhomc4)
- [Evolution of Envoy as a Dynamic Redis Proxy - Nicolas Flacco, Henry Yang & Mitch Sulaski](https://youtu.be/SWVGENzonHE)
- [Dynamic configuration control plane](https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/dynamic-configuration-control-plane)
- [Control plane in Golang](https://github.com/envoyproxy/go-control-plane/tree/4d5454027eee333e007a8d6409efd9ed39134fa7/internal/example)
- [Quick start with dynamic resources](https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/configuration-dynamic-control-plane#start-quick-start-dynamic-dynamic-resources)
- [Envoy dynamic control](https://github.com/salrashid123/envoy_control)
- [Common issues of dynamic control plane](https://github.com/istio/istio/wiki/Troubleshooting-Istio#common-issues)