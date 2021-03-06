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

// StartStatistics is called when the application is ready to start the statistics service
func (ctx *Context) StartStatistics() {
	r := mux.NewRouter()
	server := &http.Server{
		Addr:    "localhost:" + strconv.Itoa(8080),
		Handler: handlers.LoggingHandler(os.Stdout, r),
	}

	r.HandleFunc(urlMonitor, ctx.handleStatisticsRequest).Methods(http.MethodGet)

	ctx.Logger.Error(server.ListenAndServe())

}

func (ctx *Context) handleStatisticsRequest(writer http.ResponseWriter, request *http.Request) {
	if ctx.ContainerPool != nil {
		if err := json.NewEncoder(writer).Encode(ctx.ContainerPool); err != nil {
			log.Error(logCannotEncodeConnectionPool, err, ctx.Logger)
			writer.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		ctx.Logger.Error(logContainerPoolIsNil)
	}
}