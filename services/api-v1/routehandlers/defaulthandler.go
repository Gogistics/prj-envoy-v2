package routehandlers

import (
	"encoding/json"
	"net/http"
	"os"
)

type defaultHandler struct{}

var (
	// Default is ...
	Default defaultHandler
)

func (def *defaultHandler) Hello(respWriter http.ResponseWriter, req *http.Request) {
	//
	hostname, errOfHost := os.Hostname()
	if errOfHost != nil {
		panic(errOfHost)
	}
	wd, errOfWD := os.Getwd()
	if errOfWD != nil {
		panic(errOfWD)
	}

	osInfo := map[string]string{
		"host": hostname,
		"wd":   wd,
	}

	jOSInfo, err := json.Marshal(osInfo)
	if err != nil {
		http.Error(respWriter, err.Error(), http.StatusInternalServerError)
	} else {
		respWriter.Header().Set("Content-type", "application/json; charset=utf-8")
		respWriter.Write(jOSInfo)
	}
}
