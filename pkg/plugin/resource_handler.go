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

// rootRequest defines the structure for requests that only require a tenancy OCID.
type rootRequest struct {
	Tenancy string `json:"tenancy"`
}

// queryRequest defines the structure for requests to execute a query on a specific tenancy.
type queryRequest struct {
	Tenancy   string `json:"tenancy"`   // The OCID of the tenancy
	Region    string `json:"region"`    // The region of the tenancy
	Query     string `json:"getquery"`  // The query to be executed
	Field     string `json:"field"`     // Specific field for the query
	TimeStart int64  `json:"timeStart"` // The start timestamp of the time range for the query (in milliseconds)
	TimeEnd   int64  `json:"timeEnd"`   // The end timestamp of the time range for the query (in milliseconds)
}

// registerRoutes registers the HTTP routes and their corresponding handler functions.
// Parameters:
//   - mux: *http.ServeMux - The multiplexer that routes HTTP requests to the appropriate handlers.
func (ocidx *OCIDatasource) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/tenancies", ocidx.GetTenanciesHandler)
	mux.HandleFunc("/regions", ocidx.GetRegionsHandler)
	mux.HandleFunc("/getquery", ocidx.GetQueryHandler)
}

// GetTenanciesHandler handles GET requests for retrieving a list of tenancies.
// Parameters:
//   - rw: http.ResponseWriter - The response writer to send the response to the client.
//   - req: *http.Request - The incoming HTTP request containing the details for the request.
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

// GetRegionsHandler handles POST requests for retrieving a list of regions for a specific tenancy.
// Parameters:
//   - rw: http.ResponseWriter - The response writer to send the response to the client.
//   - req: *http.Request - The incoming HTTP request containing the tenancy OCID in the body.
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
	// Fetch subscribed regions for the specified tenancy OCID
	regions := ocidx.GetSubscribedRegions(req.Context(), rr.Tenancy)
	if regions == nil {
		backend.Logger.Error("plugin.resource_handler", "GetSubscribedRegions", "Could not read regions")
		respondWithError(rw, http.StatusBadRequest, "Could not read regions", nil)
		return
	}
	backend.Logger.Debug("plugin.resource_handler", "GetRegionsHandler", regions)
	writeResponse(rw, regions)
}

// GetQueryHandler handles POST requests for querying logs based on the provided parameters.
// Parameters:
//   - rw: http.ResponseWriter - The response writer to send the response to the client.
//   - req: *http.Request - The incoming HTTP request containing the query details (Tenancy, Region, Query, Field, TimeStart, TimeEnd).
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

	// Execute the query and fetch results based on the parameters
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

// writeResponse writes a successful JSON response to the http.ResponseWriter.
//
// Parameters:
//   - rw: http.ResponseWriter to write the response.
//   - resp: interface{} representing the data to be written as JSON.
func writeResponse(rw http.ResponseWriter, resp interface{}) {
	resultJson, err := jsoniter.Marshal(resp)
	if err != nil {
		backend.Logger.Error("plugin.resource_handler", "writeResponse", "could not parse response:"+err.Error())
		respondWithError(rw, http.StatusInternalServerError, "Failed to convert result", err)
	}

	sendResponse(rw, http.StatusOK, resultJson)
}

// respondWithError writes an error JSON response to the http.ResponseWriter.
//
// Parameters:
//   - rw: http.ResponseWriter to write the response.
//   - statusCode: int representing the HTTP status code.
//   - message: string representing the error message.
//   - err: error representing the error object (optional).
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

// sendResponse writes the given JSON response to the http.ResponseWriter with the specified status code.
//
// Parameters:
//   - rw: http.ResponseWriter to write the response.
//   - statusCode: int representing the HTTP status code.
//   - response: []byte representing the JSON response.
func sendResponse(rw http.ResponseWriter, statusCode int, response []byte) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(statusCode)

	_, err := rw.Write(response)
	if err != nil {
		backend.Logger.Error("plugin.resource_handler", "sendResponse", "could not write to response: "+err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
	}
}
