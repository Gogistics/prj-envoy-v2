package utilhandlers

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/Gogistics/prj-envoy-v2/services/api-v1/routehandlers"
	"github.com/gorilla/mux"
)

type appServerHandler struct {
	appRouter *mux.Router
	appMode   bool
	crtPath   string
	keyPath   string
}

var (
	// AppServerHandler is ...
	AppServerHandler = initAppServerHandler()
	dev              *bool
)

func setFlagsVals() {
	dev = flag.Bool("dev", false, "set app mode")
	flag.Parse()
}
func getFlagVal(fg string) interface{} {
	//
	switch fg {
	case "dev":
		return dev
	default:
		log.Println("Warning: flag does not exist!")
		return nil
	}
}

func getRouter() *mux.Router {
	newRouter := mux.NewRouter()

	// general REST APIs
	newRouter.HandleFunc("/api/v1", routehandlers.Default.Hello).Methods(http.MethodGet)
	newRouter.NotFoundHandler = newRouter.NewRoute().HandlerFunc(http.NotFound).GetHandler()
	return newRouter
}
func initAppServerHandler() appServerHandler {
	//
	setFlagsVals()
	appModeInterface := getFlagVal("dev")
	appMode := *appModeInterface.(*bool)
	var crtPath string
	var keyPath string

	/* Notes
	- for product development, use different certs
		dev: atai-dev-dynamic-config.com.crt and atai-dev-dynamic-config.com.key
		prd: atai-dynamic-config.com.crt and atai-dynamic-config.com.key
	*/
	if appMode {
		crtPath = "certs/atai-dynamic-config.com.crt"
		keyPath = "certs/atai-dynamic-config.com.key"
	} else {
		crtPath = "atai-dynamic-config.com.crt"
		keyPath = "atai-dynamic-config.com.key"
	}

	return appServerHandler{appRouter: getRouter(), appMode: appMode, crtPath: crtPath, keyPath: keyPath}
}

func (appSH *appServerHandler) InitAppServer() error {
	/* Notes
	Follow Gorilla README to set timeouts to avoid Slowloris attacks.

	ref:
	- https://github.com/gorilla/mux/blob/d07530f46e1eec4e40346e24af34dcc6750ad39f/README.md
	*/
	tlsCfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
	appServer := &http.Server{
		Addr:           ":443",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      tlsCfg,
		TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		Handler:        AppServerHandler.appRouter,
	}

	return appServer.ListenAndServeTLS(appSH.crtPath, appSH.keyPath)
}
