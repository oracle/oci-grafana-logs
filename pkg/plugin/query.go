// Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package plugin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/oracle/oci-grafana-logs/pkg/plugin/models"
)

// query processes a data query for the OCIDatasource, executing the necessary operations based on the query type and time range.
// It identifies the query type (Log Metrics or Log Records) and invokes the corresponding method to process the data.
//
// Parameters:
// - ctx (context.Context): The context for the query execution, typically used for cancellation or deadlines.
// - pCtx (backend.PluginContext): The plugin context, providing access to the plugin environment and configuration.
// - query (backend.DataQuery): The data query to be executed, which contains the query text and time range.
//
// Returns:
// - map[string]*DataFieldElements: A map containing the processed data field elements, which will be included in the query response.
// - backend.DataResponse: A response struct containing any errors encountered during query processing.
//
// Function Behavior:
// - The function begins by unmarshalling the query's JSON into a QueryModel object.
// - It identifies the query type (Log Metrics Time Series, Log Metrics No Interval, or Log Records) based on the query text.
// - Depending on the query type, it calls the appropriate method to process the log data (e.g., `processLogMetricTimeSeries`, `processLogMetrics`, or `processLogRecords`).
// - If an error occurs during processing, it is returned in the response. The function ensures proper handling of different query types to return the correct data format for the client.
func (ocidx *OCIDatasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) (map[string]*DataFieldElements, backend.DataResponse) {
	backend.Logger.Debug("plugin.query", "query", "query initiated for "+query.RefID)
	// Creating the Data response for query
	response := backend.DataResponse{}
	//response := backend.NewQueryDataResponse()
	// Unmarshal the json into oci queryModel
	qm := &models.QueryModel{}
	response.Error = json.Unmarshal(query.JSON, &qm)
	if response.Error != nil {
		return nil, response
	}

	takey := ocidx.GetTenancyAccessKey(qm.TenancyOCID)

	logQueryType := ocidx.identifyQueryType(qm.QueryText)

	var processErr error
	fromMs := query.TimeRange.From.UnixNano() / int64(time.Millisecond)
	toMs := query.TimeRange.To.UnixNano() / int64(time.Millisecond)
	var mFieldData = make(map[string]*DataFieldElements)

	if logQueryType == QueryType_LogMetrics_TimeSeries {
		ocidx.logger.Debug("Logging query WILL return numeric data over intervals", "refId", query.RefID)
		// Call method that parses log metric results and produces the required field definitions
		mFieldData, processErr = ocidx.processLogMetricTimeSeries(ctx, query, qm, fromMs, toMs, mFieldData, takey)
	} else if logQueryType == QueryType_LogMetrics_NoInterval {
		ocidx.logger.Debug("Logging query will NOT return numeric data over entire time range", "refId", query.RefID)
		// Call method that parses log metric results and produces the required field definitions
		mFieldData, processErr = ocidx.processLogMetrics(ctx, query, qm, fromMs, toMs, mFieldData, takey)

	} else { // QueryType_LogRecords
		ocidx.logger.Debug("Logging query will return log records for the specified time interval", "refId", query.RefID)
		// Call method that parses log record results and produces the required field definitions
		mFieldData, processErr = ocidx.processLogRecords(ctx, query, qm, fromMs, toMs, mFieldData, takey)
	}
	if processErr != nil {
		response.Error = processErr
		return nil, response
	}

	return mFieldData, response
}
