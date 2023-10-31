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

	var takey string
	takey = ocidx.GetTenancyAccessKey(qm.TenancyOCID)

	logQueryType := ocidx.identifyQueryType(qm.QueryText)
	backend.Logger.Debug("plugin.query", "logQueryType", logQueryType)

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
		return nil, response
	}

	return mFieldData, response
}
