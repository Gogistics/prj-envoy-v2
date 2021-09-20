package main

import (
	"log"

	"github.com/Gogistics/prj-envoy-v2/services/api-v1/utilhandlers"
)

func main() {
	err := utilhandlers.AppServerHandler.InitAppServer()
	if err != nil {
		log.Fatal("Failed to start ListenAndServeTLS: ", err)
	}
}
