package controller

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/gorilla/handlers"
	"encoding/json"
	"strconv"
	"os"
	"github.com/nextmetaphor/tcp-proxy-pool/log"
)

const (
	logCannotEncodeConnectionPool = "Cannot JSON encode connection pool"
	logContainerPoolIsNil = "Container pool is nil"
	urlMonitor = "/monitor"
)

func (ctx *Context) StartMonitor() {
	r := mux.NewRouter()
	server := &http.Server{
		Addr:    "localhost:" + strconv.Itoa(8080),
		Handler: handlers.LoggingHandler(os.Stdout, r),
	}

	r.HandleFunc(urlMonitor, ctx.handleMonitorRequest).Methods(http.MethodGet)

	ctx.Logger.Error(server.ListenAndServe())

}

func (ctx *Context) handleMonitorRequest(writer http.ResponseWriter, request *http.Request) {
	if ctx.ContainerPool != nil {
		if err := json.NewEncoder(writer).Encode(ctx.ContainerPool); err != nil {
			log.LogError(logCannotEncodeConnectionPool, err, ctx.Logger)
			writer.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		ctx.Logger.Error(logContainerPoolIsNil)
	}
}