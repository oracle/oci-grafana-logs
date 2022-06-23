// Copyright © 2019, 2022 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/common/auth"
	"github.com/oracle/oci-go-sdk/identity"
	"github.com/oracle/oci-go-sdk/loggingsearch"
	"github.com/pkg/errors"
)

const MaxPagesToFetch = 20

// Constants for the log search result field names processed by the plugin
const LogSearchResultsField_LogContent = "logContent"
const LogSearchResultsField_Data = "data"
const LogSearchResultsField_Oracle = "oracle"
const LogSearchResultsField_Subject = "subject"
const LogSearchResultsField_Time = "time"

// Constants for the log query data response field namess
const LogSearchResponseField_timestamp = "timestamp"

const MaxLogMetricsDataPoints = 10
const DefaultLogMetricsDataPoints = 5
const MinLogMetricsDataPoints = 2

var cacheRefreshTime = time.Minute // how often to refresh our compartmentID cache

//OCIDatasource - pulls in data from telemtry/various oci apis
type OCIDatasource struct {
	loggingSearchClient loggingsearch.LogSearchClient
	identityClient      identity.IdentityClient
	config              common.ConfigurationProvider
	cmptid              string
	logger              log.Logger
	nameToOCID          map[string]string
	timeCacheUpdated    time.Time
}

//NewOCIDatasource - constructor
func NewOCIDatasource(_ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &OCIDatasource{
		logger:     log.DefaultLogger,
		nameToOCID: make(map[string]string),
	}, nil
}

// GrafanaOCIRequest - regions Query Request comning in from the front end
type GrafanaOCIRequest struct {
	GrafanaCommonRequest
}

//GrafanaSearchRequest incoming request body for compartment search requests
type GrafanaSearchRequest struct {
	GrafanaCommonRequest
}

// GrafanaSearchLogsRequest Incoming request for a search logs query
// NOTE: The PanelId field is not critical but allows to differentiate plugin
// activity across multiple data panels within log messages
type GrafanaSearchLogsRequest struct {
	GrafanaCommonRequest
	SearchQuery   string
	MaxDataPoints int32
	PanelId       string
}

// GrafanaCommonRequest - captures the common parts of the search and metricsRequests
type GrafanaCommonRequest struct {
	Compartment string
	Environment string
	QueryType   string
	Region      string
	TenancyOCID string `json:"tenancyOCID"`
}

// Enumeration to represent the value type of a data field to be included in a data frame
type FieldValueType int

const (
	ValueType_Undefined FieldValueType = iota
	ValueType_Float64
	ValueType_Int
	ValueType_Time
	ValueType_String
)

// Represents the elements required to create a data field which is to be included in
// a data frame
type DataFieldElements struct {
	Name   string
	Type   FieldValueType
	Labels map[string]string
	Values interface{}
}

// Query - Determine what kind of query we're making
func (o *OCIDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	var ts GrafanaSearchLogsRequest

	query := req.Queries[0]
	if err := json.Unmarshal(query.JSON, &ts); err != nil {
		return &backend.QueryDataResponse{}, err
	}

	queryType := ts.QueryType
	if o.config == nil {
		configProvider, err := getConfigProvider(ts.Environment)
		if err != nil {
			return nil, errors.Wrap(err, "broken environment")
		}
		identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
		if err != nil {
			o.logger.Error("error with client")
			panic(err)
		}
		loggingSearchClient, err := loggingsearch.NewLogSearchClientWithConfigurationProvider(configProvider)
		if err != nil {
			o.logger.Error("error with client")
			panic(err)
		}
		o.identityClient = identityClient
		o.config = configProvider
		o.loggingSearchClient = loggingSearchClient
		if ts.Compartment != "" {
			o.cmptid = ts.Compartment
		}
	}

	switch queryType {
	case "compartments":
		return o.compartmentsResponse(ctx, req)
	case "regions":
		return o.regionsResponse(ctx, req)
	case "searchLogs":
		return o.searchLogsResponse(ctx, req)
	default:
		return o.testResponse(ctx, req)
	}
}

func (o *OCIDatasource) testResponse(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	return &backend.QueryDataResponse{}, nil
	// var ts GrafanaCommonRequest

	// query := req.Queries[0]
	// if err := json.Unmarshal(query.JSON, &ts); err != nil {
	// 	return &backend.QueryDataResponse{}, err
	// }

	// //o.logger.Error("ts.Com is " + ts.Compartment)
	// listMetrics := monitoring.ListMetricsRequest{
	// 	CompartmentId: common.String(ts.Compartment),
	// }
	// reg := common.StringToRegion(ts.Region)
	// o.metricsClient.SetRegion(string(reg))
	// res, err := o.metricsClient.ListMetrics(ctx, listMetrics)
	// if err != nil {
	// 	return &backend.QueryDataResponse{}, err
	// }
	// status := res.RawResponse.StatusCode
	// if status >= 200 && status < 300 {
	// 	return &backend.QueryDataResponse{}, nil
	// }
	// return nil, errors.Wrap(err, fmt.Sprintf("list metrircs failed %s %d", spew.Sdump(res), status))
}

func getConfigProvider(environment string) (common.ConfigurationProvider, error) {
	switch environment {
	case "local":
		return common.DefaultConfigProvider(), nil
	case "OCI Instance":
		return auth.InstancePrincipalConfigurationProvider()
	default:
		return nil, errors.New("unknown environment type")
	}
}

func (o *OCIDatasource) compartmentsResponse(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	var ts GrafanaSearchRequest

	query := req.Queries[0]
	if err := json.Unmarshal(query.JSON, &ts); err != nil {
		return &backend.QueryDataResponse{}, err
	}

	if o.timeCacheUpdated.IsZero() || time.Now().Sub(o.timeCacheUpdated) > cacheRefreshTime {
		m, err := o.getCompartments(ctx, ts.Region, ts.TenancyOCID)
		if err != nil {
			o.logger.Error("Unable to refresh cache")
			return nil, err
		}
		o.nameToOCID = m
	}

	frame := data.NewFrame(query.RefID,
		data.NewField("name", nil, []string{}),
		data.NewField("compartmentID", nil, []string{}),
	)
	for name, id := range o.nameToOCID {
		frame.AppendRow(name, id)
	}

	return &backend.QueryDataResponse{
		Responses: map[string]backend.DataResponse{
			query.RefID: {
				Frames: data.Frames{frame},
			},
		},
	}, nil
}

func (o *OCIDatasource) getCompartments(ctx context.Context, region string, rootCompartment string) (map[string]string, error) {
	m := make(map[string]string)

	tenancyOcid := rootCompartment

	req := identity.GetTenancyRequest{TenancyId: common.String(tenancyOcid)}
	// Send the request using the service client
	resp, err := o.identityClient.GetTenancy(context.Background(), req)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("This is what we were trying to get %s", " : fetching tenancy name"))
	}

	mapFromIdToName := make(map[string]string)
	mapFromIdToName[tenancyOcid] = *resp.Name //tenancy name

	mapFromIdToParentCmptId := make(map[string]string)
	mapFromIdToParentCmptId[tenancyOcid] = "" //since root cmpt does not have a parent

	var page *string
	reg := common.StringToRegion(region)
	o.identityClient.SetRegion(string(reg))
	for {
		res, err := o.identityClient.ListCompartments(ctx,
			identity.ListCompartmentsRequest{
				CompartmentId:          &rootCompartment,
				Page:                   page,
				AccessLevel:            identity.ListCompartmentsAccessLevelAny,
				CompartmentIdInSubtree: common.Bool(true),
			})
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("this is what we were trying to get %s", rootCompartment))
		}
		for _, compartment := range res.Items {
			if compartment.LifecycleState == identity.CompartmentLifecycleStateActive {
				mapFromIdToName[*(compartment.Id)] = *(compartment.Name)
				mapFromIdToParentCmptId[*(compartment.Id)] = *(compartment.CompartmentId)
			}
		}
		if res.OpcNextPage == nil {
			break
		}
		page = res.OpcNextPage
	}

	mapFromIdToFullCmptName := make(map[string]string)
	mapFromIdToFullCmptName[tenancyOcid] = mapFromIdToName[tenancyOcid] + "(tenancy, shown as '/')"

	for len(mapFromIdToFullCmptName) < len(mapFromIdToName) {
		for cmptId, cmptParentCmptId := range mapFromIdToParentCmptId {
			_, isCmptNameResolvedFullyAlready := mapFromIdToFullCmptName[cmptId]
			if !isCmptNameResolvedFullyAlready {
				if cmptParentCmptId == tenancyOcid {
					// If tenancy/rootCmpt my parent
					// cmpt name itself is fully qualified, just prepend '/' for tenancy aka rootCmpt
					mapFromIdToFullCmptName[cmptId] = "/" + mapFromIdToName[cmptId]
				} else {
					fullNameOfParentCmpt, isMyParentNameResolvedFully := mapFromIdToFullCmptName[cmptParentCmptId]
					if isMyParentNameResolvedFully {
						mapFromIdToFullCmptName[cmptId] = fullNameOfParentCmpt + "/" + mapFromIdToName[cmptId]
					}
				}
			}
		}
	}

	for cmptId, fullyQualifiedCmptName := range mapFromIdToFullCmptName {
		m[fullyQualifiedCmptName] = cmptId
	}
	return m, nil
}

func (o *OCIDatasource) regionsResponse(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	resp := backend.NewQueryDataResponse()

	for _, query := range req.Queries {
		var ts GrafanaOCIRequest
		if err := json.Unmarshal(query.JSON, &ts); err != nil {
			return &backend.QueryDataResponse{}, err
		}
		res, err := o.identityClient.ListRegions(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error fetching regions")
		}

		frame := data.NewFrame(query.RefID, data.NewField("text", nil, []string{}))

		for _, item := range res.Items {
			frame.AppendRow(*(item.Name))
		}

		respD := resp.Responses[query.RefID]
		respD.Frames = append(respD.Frames, frame)
		resp.Responses[query.RefID] = respD
	}
	return resp, nil
}

/*
 * Method creates a new entry in the provided data field definitions map if such an
 * entry does not already exist. If a new entry is created then it is initialized
 * using the information provided in the function parameters.
 *
 * @param dataFieldDefns - The map of data field definitions keyed off of the field
 *		name plus any distinguishing label values.
 * @param totalSamples - The number of possible data values the field can have
 * @param uniqueFieldKey - The unique identifier or key for a field which may include
 *		the field name plus any labels and associated values
 * @param fieldName - The name for the field in string format
 * @param fieldType - The data type for the field
 */
func (o *OCIDatasource) getCreateDataFieldElemsForField(dataFieldDefns map[string]*DataFieldElements,
	totalSamples int, uniqueFieldKey string, fieldName string, fieldType FieldValueType) *DataFieldElements {
	var dataFieldDefn *DataFieldElements
	var ok bool

	if dataFieldDefn, ok = dataFieldDefns[uniqueFieldKey]; !ok {
		o.logger.Debug("Did NOT find existing data field definition", "uniqueKey", uniqueFieldKey)
		// Since the specified unique key does not exist in the provided map,
		// create & populate a new DataFieldElements object and add it to the map
		// Map for the Labels element is always created and if a field has no associated labels then
		// it will be unused but this does not cause any issues when the data is presented by Grafana
		dataFieldDefn = &DataFieldElements{
			Name:   fieldName,
			Type:   fieldType,
			Labels: make(map[string]string),
			Values: nil,
		}

		/*
		 * Note that Values arrays are preallocated arrays with totalSamples entries where each entry is nil.
		 * Only intervals where a corresponding field/label combination has a value will the Values array
		 * entry have a value. This is important for situations where some field/label combinations don't
		 * have any value or data in a particular interval.
		 */
		if fieldType == ValueType_Time {
			dataFieldDefn.Values = make([]*time.Time, totalSamples)
		} else if fieldType == ValueType_Float64 {
			dataFieldDefn.Values = make([]*float64, totalSamples)
		} else if fieldType == ValueType_Int {
			dataFieldDefn.Values = make([]*int, totalSamples)
		} else { // Treat all other data types as a string (including string fields)
			dataFieldDefn.Values = make([]*string, totalSamples)
		}
		dataFieldDefns[uniqueFieldKey] = dataFieldDefn
	}

	return dataFieldDefn
}

// Function will output the list of current data field definitions in the provided map.
// NOTE:  This function should only be used when debugging plugin operation and should not be
// called in a production version of the plugin.
func (o *OCIDatasource) outputFieldDefinitions(dataFieldDefns map[string]*DataFieldElements) {
	o.logger.Debug("Outputting data field definitions")
	o.logger.Debug("# of data field definitions", "num", len(dataFieldDefns))
	for uniqueKey, dataFieldDefn := range dataFieldDefns {
		o.logger.Debug("Unique key", "uniqueKey", uniqueKey)
		o.logger.Debug("Field name", "fieldName", dataFieldDefn.Name)
		o.logger.Debug("Field type", "fieldType", dataFieldDefn.Type)
		o.logger.Debug("# of labels", "count", len(dataFieldDefn.Labels))
		if dataFieldDefn.Type == ValueType_Time {
			timeValues := dataFieldDefn.Values.([]*time.Time)
			o.logger.Debug("# of values", "count", len(timeValues))
		} else if dataFieldDefn.Type == ValueType_Float64 {
			floatValues := dataFieldDefn.Values.([]*float64)
			o.logger.Debug("# of values", "count", len(floatValues))
		} else if dataFieldDefn.Type == ValueType_Int {
			intValues := dataFieldDefn.Values.([]*int)
			o.logger.Debug("# of values", "count", len(intValues))
		} else if dataFieldDefn.Type == ValueType_String {
			stringValues := dataFieldDefn.Values.([]*string)
			o.logger.Debug("# of values", "count", len(stringValues))
		}
	}
}

/*
 * Method determines whether the specified OCI Logging service query will return
 * numeric data (assuming it succeeds). This determination is made by checking
 * whether any of the Logging query functions, e.g. sum() or avg(), that cause
 * a query to return numeric data are used within the specified query string.
 * Method returns true if the query will return numeric data and false otherwise.
 *
 * @param loggingSearchQuery - Logging search query string
 */
func (o *OCIDatasource) queryReturnsNumericData(loggingSearchQuery string) bool {
	bNumericResult := false

	// Determine if the specified logging query utilizes any of the mathematical query functions, see
	// https://docs.oracle.com/en-us/iaas/Content/Logging/Reference/query_language_specification.htm
	// for the full query language specification

	reAvg, _ := regexp.Compile(`avg\s*\(.+\)`)
	reSum, _ := regexp.Compile(`sum\s*\(.+\)`)
	reMin, _ := regexp.Compile(`min\s*\(.+\)`)
	reMax, _ := regexp.Compile(`max\s*\(.+\)`)
	/*
	 * There are many valid ways to use the count aggregate operator within a logging
	 * search query including:
	 *   search "<compartment>[/<log group>[/<log>]]" | count
	 *   search "<compartment>[/<log group>[/<log>]]" | summarize count()
	 *   search "<compartment>[/<log group>[/<log>]]" | summarize count() by (<field1>)
	 *   search "<compartment>[/<log group>[/<log>]]" | summarize count(<field1>) by (<field2>)
	 * The next regex object attempts to cover the first above case while the 2nd
	 * regex object attempts to cover the remainder of the above cases
	 */
	reCountWithoutParens, _ := regexp.Compile(`\|\s*count\s*$`)
	reCountWithParens, _ := regexp.Compile(`\s*count\s*\(.*\)`)

	// Check if the logging search query includes any of the mathematical query
	// functions represented in the regular expression objects
	if reAvg.Match([]byte(loggingSearchQuery)) == true ||
		reSum.Match([]byte(loggingSearchQuery)) == true ||
		reCountWithParens.Match([]byte(loggingSearchQuery)) == true ||
		reCountWithoutParens.Match([]byte(loggingSearchQuery)) == true ||
		reMin.Match([]byte(loggingSearchQuery)) == true ||
		reMax.Match([]byte(loggingSearchQuery)) == true {
		bNumericResult = true
	} else {
		bNumericResult = false
	}

	return bNumericResult

}

/*
 * Data source class method that performs a logging search query and processes the
 * returned log data. This method ONLY processes results from a logging search query
 * that returns log records (as opposed to a logging search query that returns log
 * related metrics). The data returned by this method is held in the provided data
 * field definitions map.
 *
 * @param ctx - Additional context for the execution of the query
 * @param query - Object representing the characteristics of the query
 * @param searchLogsReq - Object containing the attributes of the search logs request from
 *			the plugin frontend
 * @param fromMs - The time (in milliseconds) that identifies the start of the query time range
 * @param toMs - The time (in milliseconds) that identifies the end of the query time range
 * @param mFieldDefns - A map of data field definitions where each element references an object
 *			that defines the characteristics of a given data field
 */
func (o *OCIDatasource) processLogRecords(ctx context.Context, searchLogsReq GrafanaSearchLogsRequest,
	query backend.DataQuery, fromMs int64, toMs int64, mFieldDefns map[string]*DataFieldElements) error {

	var queryRefId string = query.RefID
	var queryPanelId string = searchLogsReq.PanelId

	// Implicit assumption that the request contains this field, must be set by the plugin frontend
	searchQuery := searchLogsReq.SearchQuery

	// Populate a SearchLogsDetails structure to provide with the logging search API call
	req1 := loggingsearch.SearchLogsDetails{}

	// hardcoded for now
	req1.IsReturnFieldInfo = common.Bool(false)

	// Convert the current to/from time values into the format required for the Logging service search
	// API call
	start := time.Unix(fromMs/1000, (fromMs%1000)*1000000).UTC()
	end := time.Unix(toMs/1000, (toMs%1000)*1000000).UTC()
	start = start.Truncate(time.Millisecond)
	end = end.Truncate(time.Millisecond)

	// Set the current query time range start and end times for the current interval
	req1.TimeStart = &common.SDKTime{start}
	req1.TimeEnd = &common.SDKTime{end}
	// Directly use the query provided by the user
	req1.SearchQuery = common.String(searchQuery)
	o.logger.Debug("Processing log records search query", "panelId", queryPanelId, "refId", queryRefId,
		"query", searchQuery, "from", query.TimeRange.From, "to", query.TimeRange.To)

	// Construct the Logging service SearchLogs request structure
	request := loggingsearch.SearchLogsRequest{
		SearchLogsDetails: req1,
		Limit:             common.Int(500),
	}
	reg := common.StringToRegion(searchLogsReq.Region)
	o.loggingSearchClient.SetRegion(string(reg))
	// Perform the logs search operation
	res, err := o.loggingSearchClient.SearchLogs(ctx, request)

	if err != nil {
		o.logger.Debug(fmt.Sprintf("Log search operation FAILED, queryPanelId = %s, refId = %s, err = %s",
			queryPanelId, queryRefId, err))
		return errors.Wrap(err, "error fetching logs")
	}
	o.logger.Debug("Log search operation SUCCEEDED", "panelId", queryPanelId, "refId", queryRefId)

	// Determine how many rows were returned in the search results
	resultCount := *res.SearchResponse.Summary.ResultCount

	if resultCount > 0 {

		// Loop through each row of the results and add data values for each of encountered fields
		for rowCount, logSearchResult := range res.SearchResponse.Results {
			var fieldDefn *DataFieldElements
			searchResultData, ok := (*logSearchResult.Data).(map[string]interface{})
			if ok == true {
				if logContent, ok := searchResultData[LogSearchResultsField_LogContent]; ok {
					mLogContent, ok := logContent.(map[string]interface{})
					if ok == true {
						for key, value := range mLogContent {

							// Only three special case fields within a log record: 1) time, 2) data, and 3) oracle
							// Treat all other logContent fields as strings
							if key == LogSearchResultsField_Time {
								fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, resultCount,
									LogSearchResponseField_timestamp, LogSearchResponseField_timestamp,
									ValueType_Time)
								timestamp, errStr := time.Parse(time.RFC3339, value.(string))
								if errStr != nil {
									o.logger.Debug("Error parsing timestamp string", "panelId", queryPanelId,
										"refId", queryRefId, LogSearchResponseField_timestamp,
										mLogContent[LogSearchResultsField_Time],
										"error", errStr)
								}
								fieldDefn.Values.([]*time.Time)[rowCount] = &timestamp
							} else if key == LogSearchResultsField_Data || key == LogSearchResultsField_Oracle {
								var logData string = ""
								fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, resultCount,
									key, key, ValueType_String)

								logJSON, marerr := json.Marshal(value)
								if marerr == nil {
									logData = string(logJSON)
								} else {
									o.logger.Debug("Error marshalling log record data string, log data variable type",
										"panelId", queryPanelId, "refId", queryRefId, "type", fmt.Sprintf("%T", value))
									logData = "UNKNOWN"
								}
								fieldDefn.Values.([]*string)[rowCount] = &logData

								// Skip the subject field since it seems to always be an empty string
								// For all other keys treat them generically as string type
							} else if key != LogSearchResultsField_Subject {
								var stringFieldValue string
								fieldDefn = nil

								if stringFieldValue, ok = value.(string); ok {
									// If the field value is non-zero length string then proceed to get/create the data
									// field definition. But if the field value is a zero length string then skip
									// creating the data field definition, this is to avoid creating a data field for a
									// log record field that is always empty.
									if len(stringFieldValue) > 0 {
										fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, resultCount,
											key, key, ValueType_String)
									}
								} else {
									o.logger.Debug("Error parsing string field value", "panelId", queryPanelId,
										"refId", queryRefId, "key", key, "value", value)
									fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, resultCount,
										key, key, ValueType_String)
									stringFieldValue = "UNKNOWN"
								}
								if fieldDefn != nil {
									fieldDefn.Values.([]*string)[rowCount] = &stringFieldValue
								}
							} // endif key name
						} // for each field key in the logContent field

					} else {
						o.logger.Debug("Unable to get logContent map", "panelId", queryPanelId,
							"refId", queryRefId, "row", rowCount)
					}
				} else {
					o.logger.Debug("Encountered log record without a logContent element",
						"panelId", queryPanelId, "refId", queryRefId, "row", rowCount)
				}
			} else {
				o.logger.Debug("Encountered row without a log record", "panelId", queryPanelId,
					"refId", queryRefId, "row", rowCount)
			}
		}
	} else {
		o.logger.Warn("Logging search query returned no results", "panelId", queryPanelId,
			"refId", queryRefId)
	}

	return nil
}

/*
 * Data source class method that performs a logging search query and processes the
 * returned log data. This method ONLY processes results from a logging search query
 * that returns log metrics (as opposed to a logging search query that returns log
 * records). The data returned by this method is held in the provided field
 * definitions map.
 *
 * @param ctx - Additional context for the execution of the query
 * @param query - Object representing the characteristics of the query
 * @param searchLogsReq - Object containing the attributes of the search logs request from
 *			the plugin frontend
 * @param fromMs - The time (in milliseconds) that identifies the start of the query time range
 * @param toMs - The time (in milliseconds) that identifies the end of the query time range
 * @param mFieldDefns - A map of data field definitions where each element references an object
 *			that defines the characteristics of a given field
 */
func (o *OCIDatasource) processLogMetrics(ctx context.Context, searchLogsReq GrafanaSearchLogsRequest,
	query backend.DataQuery, fromMs int64, toMs int64, mFieldDefns map[string]*DataFieldElements) error {

	var numDataPoints int32
	var intervalMs float64
	var queryRefId string = query.RefID
	var queryPanelId string = searchLogsReq.PanelId

	// Implicit assumption that the request contains this field, must be set by the plugin frontend
	searchQuery := searchLogsReq.SearchQuery

	o.logger.Debug("Processing log metrics search query", "panelId", queryPanelId, "refId", queryRefId,
		"query", searchQuery, "from", query.TimeRange.From.UTC(), "to", query.TimeRange.To.UTC())

	// Check the max data points value set within the query options element of the data panel to use that
	// as guidance for the number of data points to be returned. However the default value provided for the
	// max data points by Grafana is typically very high (800-1000) which is going to lead to way too many
	// logging search queries and thus a poor user experience. So the provided max data points value will be
	// used if it is less than or equal to  our max log metrics data points limit or if there is no value
	// then use our defined default log metrics data points value. Otherwise use the value set by the user
	// in the data panel
	if searchLogsReq.MaxDataPoints >= MaxLogMetricsDataPoints {
		numDataPoints = MaxLogMetricsDataPoints
	} else if searchLogsReq.MaxDataPoints <= 0 {
		numDataPoints = DefaultLogMetricsDataPoints
	} else if searchLogsReq.MaxDataPoints < MinLogMetricsDataPoints {
		numDataPoints = MinLogMetricsDataPoints
	} else {
		numDataPoints = searchLogsReq.MaxDataPoints
	}

	// Compute the query interval using the number of data points (reduced by one to account for the data
	// sample at the start of the period). Store the interval as a floating point number to handle cases
	// where the computed interval is not an integer number of milliseconds
	intervalMs = float64(toMs-fromMs) / float64(numDataPoints-1)

	o.logger.Debug("Derived query interval", "panelId", queryPanelId, "refId", queryRefId,
		"numDataPoints", numDataPoints, "intervalInMs", intervalMs)

	// Populate a SearchLogsDetails structure to provide with the logging search API call
	req1 := loggingsearch.SearchLogsDetails{}

	// hardcoded for now
	req1.IsReturnFieldInfo = common.Bool(false)

	// To fill the data panel from the start of the specified period to the end there needs to be
	// an initial data point at the start of the period. To be able get this initial data sample
	// we will actually move back the start time by one interval to generate this initial data
	// sample. This is also why the initial from timestamp (in milliseconds) is "backed up" one interval
	currFromMs := fromMs - int64(intervalMs) + 1
	currToMs := fromMs

	// For the number of required data points loop through the logic to run the query for a sub-interval
	// of the specified query time range. Process each search query's results and combine all of the results
	// into a set of data field definitions and set of values per data field. This information will be used
	// to construct the data frame to be passed to the front end as the response to the incoming query.
	for intervalCnt := 0; intervalCnt < int(numDataPoints); intervalCnt++ {
		// Compute the from/to time for the current interval (in milliseconds) if this is not the
		// initial interval
		if intervalCnt > 0 {
			// Set the from time for the current interval to one millisecond greater than the prior period
			// to ensure that we cover all milliseconds within the original query interval
			currFromMs = currToMs + 1

			currToMs = fromMs + int64(float64(intervalMs)*float64(intervalCnt))

			// If this is the last interval then set the 'to' time to value provided with the query. This
			// ensures that if there are any partial milliseconds not accounted for in the interval
			// start & end times to this point they are added to the last interval. In this way the final
			// interval will end on the 'to' time specified in the query.
			if (intervalCnt + 1) == int(numDataPoints) {
				currToMs = toMs
			}
		}

		// Convert the current to/from time values into the format required for the Logging service search
		// API call
		start := time.Unix(currFromMs/1000, (currFromMs%1000)*1000000).UTC()
		end := time.Unix(currToMs/1000, (currToMs%1000)*1000000).UTC()
		start = start.Truncate(time.Millisecond)
		end = end.Truncate(time.Millisecond)

		o.logger.Debug("Intermediate logging query time range", "panelId", queryPanelId, "refId", queryRefId,
			"interval", intervalCnt, "from", start, "to", end)

		// Set the current query time range start and end times for the current interval
		req1.TimeStart = &common.SDKTime{start}
		req1.TimeEnd = &common.SDKTime{end}
		// Directly use the query provided by the user
		req1.SearchQuery = common.String(searchQuery)

		// Construct the Logging service SearchLogs request structure
		request := loggingsearch.SearchLogsRequest{
			SearchLogsDetails: req1,
			Limit:             common.Int(500),
		}
		reg := common.StringToRegion(searchLogsReq.Region)
		o.loggingSearchClient.SetRegion(string(reg))

		// Perform the logs search operation
		res, err := o.loggingSearchClient.SearchLogs(ctx, request)

		if err != nil {
			o.logger.Debug(fmt.Sprintf("Log search operation FAILED, panelId = %s, refId = %s, err = %s",
				queryPanelId, queryRefId, err))
			return errors.Wrap(err, "error fetching logs")
		}
		o.logger.Debug("Log search operation SUCCEEDED", "panelId", queryPanelId, "refId", queryRefId,
			"interval", intervalCnt)

		// Determine how many rows were returned in the search results
		resultCount := *res.SearchResponse.Summary.ResultCount

		if resultCount > 0 {

			searchResultData, ok := (*res.SearchResponse.Results[0].Data).(map[string]interface{})
			if ok == true {

				if _, ok := searchResultData[LogSearchResultsField_LogContent]; !ok {

					// Prepare regular expression filters once for processing all results, using
					// a raw string to simplify escaping
					reFunc, _ := regexp.Compile(`^([a-zA-Z]+)\((.+)\)`)

					labelFieldKey := ""
					numericFieldKey := ""
					numericFieldType := ValueType_Undefined
					metricFieldName := ""

					var fieldDefn *DataFieldElements
					var labelValue string
					var convOk bool

					// There needs to be a timestamp field but there is none returned in the
					// logging query results, so create the timestamp field if it doesn't already
					// exist and use the end time range for the current query interval as the
					// timestamp value
					fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, int(numDataPoints),
						LogSearchResponseField_timestamp, LogSearchResponseField_timestamp,
						ValueType_Time)

					// This needs to be the 'To' time for the current interval in time.Time format
					currToTime := time.UnixMilli(currToMs).UTC()
					fieldDefn.Values.([]*time.Time)[intervalCnt] = &currToTime

					for rowCount, logSearchResult := range res.SearchResponse.Results {
						searchResultData, ok := (*logSearchResult.Data).(map[string]interface{})
						if ok == true {
							// If this is the first row then inspect the values of the elements to
							// speed up the processing of the remaining rows
							if rowCount == 0 {
								// Loop through the keys for the entries in the results data item
								// to determine what kind of fields we have in the results
								for key, value := range searchResultData {
									// Check whether the key contains one of the aggregation functions
									if key == "count" {
										metricFieldName = "count"
										numericFieldKey = key
										// In the JSON content for the log record the count appears as an
										// integer but when converted becomes a float value
										numericFieldType = ValueType_Float64
									} else if reFunc.Match([]byte(key)) == true {
										metricFieldName = key
										numericFieldKey = key
										if _, ok := value.(float64); ok {
											numericFieldType = ValueType_Float64
										} else if _, ok := value.(int); ok {
											numericFieldType = ValueType_Int
										} else {
											o.logger.Error("Unable to determine numeric data type for field value",
												"panelId", queryPanelId, "refId", queryRefId, "value", value)
											numericFieldType = ValueType_Undefined
										}
									} else {
										labelFieldKey = key
									}
								}
							} // end if first row

							if numericFieldType == ValueType_Float64 {

								metricFieldCombKey := metricFieldName
								if labelFieldKey != "" {
									// On rare occasions the identified label field will have no value so need
									// to protect against that case by checking the conversion result
									if labelValue, convOk = searchResultData[labelFieldKey].(string); !convOk {
										labelValue = "null"
									}
									metricFieldCombKey = metricFieldName + "_" + labelValue
								}
								// Get or create the data field elements structure for this field
								fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, int(numDataPoints),
									metricFieldCombKey, metricFieldName, ValueType_Float64)

								if floatValue, ok := searchResultData[numericFieldKey].(float64); ok {
									fieldDefn.Values.([]*float64)[intervalCnt] = &floatValue
								} else {
									o.logger.Error("Unable to extract float field value",
										"panelId", queryPanelId, "refId", queryRefId, "field", numericFieldKey)
								}
								if labelFieldKey != "" {
									fieldDefn.Labels[labelFieldKey] = labelValue
								}

							} else if numericFieldType == ValueType_Int {

								metricFieldCombKey := metricFieldName
								if labelFieldKey != "" {
									// On rare occasions the identified label field will have no value so need
									// to protect against that case by checking the conversion result
									if labelValue, convOk = searchResultData[labelFieldKey].(string); !convOk {
										labelValue = "null"
									}
									metricFieldCombKey = metricFieldName + "_" + labelValue
								}
								// Get or create the data field elements structure for this field
								fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, int(numDataPoints),
									metricFieldCombKey, metricFieldName, ValueType_Int)

								if intValue, ok := searchResultData[numericFieldKey].(int); ok {
									fieldDefn.Values.([]*int)[intervalCnt] = &intValue
								} else {
									o.logger.Error("Unable to extract int value for ",
										"panelId", queryPanelId, "refId", queryRefId, "field", numericFieldKey)
								}

								if labelFieldKey != "" {
									fieldDefn.Labels[labelFieldKey] = labelValue
								}

							} else {
								o.logger.Debug("Encountered unexpected field value type for numeric results logging query",
									"panelId", queryPanelId, "refId", queryRefId)
							}

						} else {
							o.logger.Error("Unable to map result data elements",
								"panelId", queryPanelId, "refId", queryRefId, "row", rowCount)
						}
					}
				} else {
					o.logger.Debug("Log search results should NOT contain log records",
						"panelId", queryPanelId, "refId", queryRefId)
				}
			} else {
				o.logger.Debug("Unable to assert search result data is a string map",
					"panelId", queryPanelId, "refId", queryRefId)
			}
		} else { // result count is <= 0
			o.logger.Debug("No results returned by query", "panelId", queryPanelId,
				"refId", queryRefId, "resultCount", *res.SearchResponse.Summary.ResultCount)
		}

	} // end for the required number of data intervals

	return nil
}

/*
 * Data source class method that processes a set of query requests received from the
 * plugin frontend and provides back a query response for each of the queries
 * referenced in the request. The data returned by this method is formatted as data
 * frames which can be directly rendered by Grafana without further manipulation by
 * the plugin's front end.
 *
 * The method has to handle (at least) two types of log query results:
 *   1. Log records that meet some filtering criteria specified via a logging search query
 *   2. Numeric data derived from log records over the specified time period which
 *      are then aggregated by a mathematical operation such as sum() or avg().
 */
func (o *OCIDatasource) searchLogsResponse(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	resp := backend.NewQueryDataResponse()
	for _, query := range req.Queries {
		var ts GrafanaSearchLogsRequest

		/*
		* This is a map containing an entry per data field that will be added to the data frame.
		* The map key is the field name (plus any distinguishing label values) and the value is an
		* array of pointers to the relevant characteristics for the field.
		 */
		var mFieldData = make(map[string]*DataFieldElements)

		// Unmarshal the request to determine whether the query will return log records or numeric data
		if err := json.Unmarshal(query.JSON, &ts); err != nil {
			return &backend.QueryDataResponse{}, err
		}

		// Convert the from and to time range values into milliseconds since January 1, 1970 which makes
		// them easier to use in forthcoming computations
		fromMs := query.TimeRange.From.UnixNano() / int64(time.Millisecond)
		toMs := query.TimeRange.To.UnixNano() / int64(time.Millisecond)

		// Determine whether the specified query will return numeric data (based on its use of numerical
		// logging query functions)
		bNumericResult := o.queryReturnsNumericData(ts.SearchQuery)

		var processErr error
		// Call the appropriate function to process the logging search results based on the expected
		// type of results (metrics or log records). The data extracted from the log search results
		// is held in the data field definitions map which is used below to construct the data frame
		// containing the data returned by the query in a format that Grafana can understand.
		if bNumericResult == true {
			o.logger.Debug("Logging query WILL return numeric data", "refId", query.RefID)
			// Call method that parses log metric results and produces the required field definitions
			processErr = o.processLogMetrics(ctx, ts, query, fromMs, toMs, mFieldData)
		} else {
			o.logger.Debug("Logging query will NOT return numeric data", "refId", query.RefID)
			// Call method that parses log record results and produces the required field definitions
			processErr = o.processLogRecords(ctx, ts, query, fromMs, toMs, mFieldData)
		}
		if processErr != nil {
			return nil, processErr
		}

		/*
		 * Create the data frame for the current logging search query using the accumulated
		 * field definitions derived from the query results
		 */

		var frame *data.Frame = nil
		// Create an array of data.Field pointers, one for each data field definition in the
		// field definition map
		dfFields := make([]*data.Field, len(mFieldData))
		// Get the query ID from the responses as that ID needs to be referenced in the data frame
		respD := resp.Responses[query.RefID]

		// Loop through each of the data field definitions and create a corresponding data.Field object
		// using the information in the data field definition to initialize the Field object
		fieldCnt := 0
		for _, fieldDataElems := range mFieldData {
			dfFields[fieldCnt] = data.NewField(fieldDataElems.Name, fieldDataElems.Labels, fieldDataElems.Values)
			fieldCnt += 1
		}
		// Create a new data Frame using the generated Fields while referencing the query ID
		frame = data.NewFrame(query.RefID, dfFields...)

		// Add the current frame to the list of frames for all of the provided queries
		respD.Frames = append(respD.Frames, frame)
		resp.Responses[query.RefID] = respD

	} // for each query included in the request

	return resp, nil
}
