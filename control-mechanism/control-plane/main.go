package main

import (
	"flag"

	"github.com/Gogistics/prj-envoy-v2/control-mechanism/control-plane/utilhandlers"
)

var (
	port   uint
	nodeID string
)

func init() {

	// The port that this xDS server listens on
	flag.UintVar(&port, "port", 20000, "xDS management server port")

	// Tell Envoy to use this Node ID
	flag.StringVar(&nodeID, "nodeID", "atai-test-id-123", "Node ID")
}

// TODO: try to build custom Callbacks that holds callback properties
/* ref:
- https://github.com/envoyproxy/go-control-plane/blob/main/pkg/test/v3/callbacks.go
- https://github.com/envoyproxy/go-control-plane/blob/v0.9.9/pkg/server/v3/server.go#L106
*/
func main() {
	flag.Parse()
	utilhandlers.RunServer(port)
}
