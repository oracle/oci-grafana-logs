/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
 */

package plugin

import (
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	jsoniter "github.com/json-iterator/go"

	"github.com/oracle/oci-grafana-logs/pkg/plugin/models"
)

type rootRequest struct {
	Tenancy string `json:"tenancy"`
}

type queryRequest struct {
	Tenancy   string `json:"tenancy"`
	Region    string `json:"region"`
	Query     string `json:"getquery"`
	Field     string `json:"field"`
	TimeStart int64  `json:"timeStart"`
	TimeEnd   int64  `json:"timeEnd"`
}

func (ocidx *OCIDatasource) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/tenancies", ocidx.GetTenanciesHandler)
	mux.HandleFunc("/regions", ocidx.GetRegionsHandler)
	mux.HandleFunc("/getquery", ocidx.GetQueryHandler)
}

func (ocidx *OCIDatasource) GetTenanciesHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		respondWithError(rw, http.StatusMethodNotAllowed, "Invalid method", nil)
		return
	}

	// ts := ocidx.clients.GetTenancies(req.Context())
	ts := ocidx.GetTenancies(req.Context())
	backend.Logger.Debug("plugin.resource_handler", "GetTenanciesHandler", ts)
	writeResponse(rw, ts)
}

func (ocidx *OCIDatasource) GetRegionsHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		respondWithError(rw, http.StatusMethodNotAllowed, "Invalid method", nil)
		return
	}

	var rr rootRequest
	if err := jsoniter.NewDecoder(req.Body).Decode(&rr); err != nil {
		backend.Logger.Error("plugin.resource_handler", "GetRegionsHandler", err)
		respondWithError(rw, http.StatusBadRequest, "Failed to read request body", err)
		return
	}
	regions := ocidx.GetSubscribedRegions(req.Context(), rr.Tenancy)
	if regions == nil {
		backend.Logger.Error("plugin.resource_handler", "GetSubscribedRegions", "Could not read regions")
		respondWithError(rw, http.StatusBadRequest, "Could not read regions", nil)
		return
	}
	backend.Logger.Debug("plugin.resource_handler", "GetRegionsHandler", regions)
	writeResponse(rw, regions)
}

func (ocidx *OCIDatasource) GetQueryHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		respondWithError(rw, http.StatusMethodNotAllowed, "Invalid method", nil)
		return
	}

	var rr queryRequest
	if err := jsoniter.NewDecoder(req.Body).Decode(&rr); err != nil {
		backend.Logger.Error("plugin.resource_handler", "GetQueryHandler", err)
		respondWithError(rw, http.StatusBadRequest, "Failed to read request body", err)
		return
	}

	resp, err := ocidx.getLogs(req.Context(), rr.Tenancy, rr.Query, rr.Field, rr.TimeStart, rr.TimeEnd)
	if err != nil {
		backend.Logger.Error("plugin.resource_handler", "GetQueryHandler", err)
		respondWithError(rw, http.StatusBadRequest, "Could not run query", err)
		return
	}

	if resp == nil {
		backend.Logger.Error("plugin.resource_handler", "GetQueryHandler", "Query Result is empty")
		respondWithError(rw, http.StatusBadRequest, "Query Result is empty", nil)
		return
	}
	backend.Logger.Debug("plugin.resource_handler", "GetQueryHandler", resp)
	writeResponse(rw, resp)
}

func writeResponse(rw http.ResponseWriter, resp interface{}) {
	resultJson, err := jsoniter.Marshal(resp)
	if err != nil {
		backend.Logger.Error("plugin.resource_handler", "writeResponse", "could not parse response:"+err.Error())
		respondWithError(rw, http.StatusInternalServerError, "Failed to convert result", err)
	}

	sendResponse(rw, http.StatusOK, resultJson)
}

func respondWithError(rw http.ResponseWriter, statusCode int, message string, err error) {
	httpError := &models.HttpError{
		Message:    message,
		StatusCode: statusCode,
	}
	if err != nil {
		httpError.Error = err.Error()
	}

	response, err := jsoniter.Marshal(httpError)
	if err != nil {
		backend.Logger.Error("plugin.resource_handler", "respondWithError", "could not parse response:"+err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	sendResponse(rw, statusCode, response)
}

func sendResponse(rw http.ResponseWriter, statusCode int, response []byte) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(statusCode)

	_, err := rw.Write(response)
	if err != nil {
		backend.Logger.Error("plugin.resource_handler", "sendResponse", "could not write to response: "+err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
	}
}
