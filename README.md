# Dynamic configuration xDS (WIP)

## Steps of running dynamic configuration
1. Draw diagram of the container topology
2. Create folders

## Context
Dynamically update Envoy by the control plane and what we are going to test are as follows:
* Bring up the front Envoy without API clusters configuration. Test the API services and will get error.
* Send new configuration to the control plane, and it will forward the configuration to the Envoy. Test the API service 1. and will receive the response.
* Send new configuration to the control plane, and it will forward the configuration to the Envoy. Test the API service 2. and will receive the response.
* Stop the control plane and all services should still work fine.

## ...
Step 1. init networks
Step 2. start Envoy front proxy
Step 3. check the config.


```sh
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
```

# build API services (WIP)
```sh
```

## References:
- https://github.com/salrashid123/envoy_control