package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/logging"
	"github.com/oracle/oci-go-sdk/v65/loggingsearch"
	"github.com/oracle/oci-grafana-logs/pkg/plugin/constants"
	"github.com/oracle/oci-grafana-logs/pkg/plugin/models"
	"github.com/pkg/errors"
)

type LogSearchQueryType int

type FieldValueType int

type DataFieldElements struct {
	Name   string
	Type   FieldValueType
	Labels map[string]string
	Values interface{}
}

type LogTimeSeriesResult struct {
	TimestampMs    int64
	mMetricResults []*map[string]interface{}
}

const (
	QueryType_Undefined LogSearchQueryType = iota
	QueryType_LogRecords
	QueryType_LogMetrics_NoInterval
	QueryType_LogMetrics_TimeSeries
)

const numMaxResults = (constants.MaxPagesToFetch * constants.LimitPerPage) + 1

// TestConnectivity Check the OCI data source test request in Grafana's Datasource configuration UI.
func (o *OCIDatasource) TestConnectivity(ctx context.Context) error {
	backend.Logger.Debug("client", "TestConnectivity", "testing the OCI connectivity")

	var reg common.Region
	//var errAllComp error

	tenv := o.settings.Environment
	tmode := o.settings.TenancyMode
	if len(o.tenancyAccess) == 0 {
		return fmt.Errorf("TestConnectivity failed: cannot read o.tenancyAccess")
	}
	for key, _ := range o.tenancyAccess {
		if tmode == "multitenancy" && tenv == "OCI Instance" {
			return errors.New("Multitenancy mode using instance principals is not implemented yet.")
		}

		tenancyocid, tenancyErr := o.tenancyAccess[key].config.TenancyOCID()
		if tenancyErr != nil {
			return errors.Wrap(tenancyErr, "error fetching TenancyOCID")
		}

		regio, regErr := o.tenancyAccess[key].config.Region()
		if regErr != nil {
			return errors.Wrap(regErr, "error fetching Region")
		}
		reg = common.StringToRegion(regio)
		//perfect till above
		o.tenancyAccess[key].loggingSearchClient.SetRegion(string(reg))
		backend.Logger.Debug("TestConnectivity", "Config Key", key, "Testing Tenancy OCID", tenancyocid)
		if tenv == "local" {
			queri := `search "` + tenancyocid + `" | sort by datetime desc`
			t := time.Now()
			t2 := t.Add(-time.Minute * 30)
			start, _ := time.Parse(time.RFC3339, t2.Format(time.RFC3339))
			end, _ := time.Parse(time.RFC3339, t.Format(time.RFC3339))
			request := loggingsearch.SearchLogsRequest{SearchLogsDetails: loggingsearch.SearchLogsDetails{SearchQuery: common.String(queri),
				TimeStart:         &common.SDKTime{Time: start},
				TimeEnd:           &common.SDKTime{Time: end},
				IsReturnFieldInfo: common.Bool(false)},
				Limit: common.Int(10)}
			res, err := o.tenancyAccess[key].loggingSearchClient.SearchLogs(ctx, request)
			if err != nil {
				backend.Logger.Error("TestConnectivity", "Config Key", key, "SKIPPED", err)
				return fmt.Errorf("ListLogGroupsRequest failed in each Compartments in profile %v", err)
			}
			status := res.RawResponse.StatusCode
			if status >= 200 && status < 300 {
				backend.Logger.Debug("TestConnectivity", "Config Key", key, "OK", status)
				break
			} else {
				o.logger.Debug(key, "SKIPPED", status)
				return errors.Wrap(err, fmt.Sprintf("ListLogGroupsRequest failed: %s", key))
			}
		} else {
			request := logging.ListLogGroupsRequest{Limit: common.Int(69),
				CompartmentId:            common.String(tenancyocid),
				IsCompartmentIdInSubtree: common.Bool(true)}
			res, err := o.tenancyAccess[key].loggingManagementClient.ListLogGroups(ctx, request)
			if err != nil {
				o.logger.Debug(key, "FAILED", err)
				return errors.Wrap(err, fmt.Sprintf("ListLogGroupsRequest failed:%s", key))
			}
			status := res.RawResponse.StatusCode
			if status >= 200 && status < 300 {
				backend.Logger.Debug("TestConnectivity", "Config Key", key, "OK", status)
				break
			} else {
				backend.Logger.Debug("TestConnectivity", "Config Key", key, "SKIPPED", status)
				return errors.Wrap(err, fmt.Sprintf("ListLogGroupsRequest failed in each Compartments in profile %s", key))
			}
		}

	}
	return nil
}

/*
Function generates an array  containing OCI tenancy list in the following format:
<Label/TenancyOCID>
*/
func (o *OCIDatasource) GetTenancies(ctx context.Context) []models.OCIResource {
	backend.Logger.Debug("client", "GetTenancies", "fetching the tenancies")

	tenancyList := []models.OCIResource{}
	for key, _ := range o.tenancyAccess {
		// frame.AppendRow(*(common.String(key)))

		tenancyList = append(tenancyList, models.OCIResource{
			Name: *(common.String(key)),
			OCID: *(common.String(key)),
		})
	}

	return tenancyList
}

// GetSubscribedRegions Returns the subscribed regions by the mentioned tenancy
// API Operation: ListRegionSubscriptions
// Permission Required: TENANCY_INSPECT
// Links:
// https://docs.oracle.com/en-us/iaas/Content/Identity/Reference/iampolicyreference.htm
// https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingregions.htm
// https://docs.oracle.com/en-us/iaas/api/#/en/identity/20160918/RegionSubscription/ListRegionSubscriptions
func (o *OCIDatasource) GetSubscribedRegions(ctx context.Context, tenancyOCID string) []string {
	backend.Logger.Debug("client", "GetSubscribedRegions", "fetching the subscribed region for tenancy: "+tenancyOCID)

	var subscribedRegions []string
	takey := o.GetTenancyAccessKey(tenancyOCID)
	tenancymode := o.settings.TenancyMode
	var tenancyocid string
	var tenancyErr error

	if len(takey) == 0 {
		backend.Logger.Error("client", "GetSubscribedRegions", "invalid takey")
		return nil
	}
	if tenancymode == "multitenancy" {
		if len(takey) <= 0 || takey == NoTenancy {
			o.logger.Error("Unable to get Multi-tenancy OCID")
			return nil
		}
		res := strings.Split(takey, "/")
		tenancyocid = res[1]
	} else {
		tenancyocid, tenancyErr = o.tenancyAccess[takey].config.TenancyOCID()
		if tenancyErr != nil {
			return nil
		}
	}
	backend.Logger.Debug("client", "GetSubscribedRegionstakey", "fetching the subscribed region for tenancy OCID: "+*common.String(tenancyocid))

	req := identity.ListRegionSubscriptionsRequest{TenancyId: common.String(tenancyocid)}

	resp, err := o.tenancyAccess[takey].identityClient.ListRegionSubscriptions(ctx, req)
	if err != nil {
		backend.Logger.Error("client", "error in GetSubscribedRegions", err)
		return nil
	}

	if resp.RawResponse.StatusCode != 200 {
		backend.Logger.Error("client", "GetSubscribedRegions", "Could not fetch subscribed regions. Please check IAM policy.")
		return subscribedRegions
	}

	for _, item := range resp.Items {
		if item.Status == identity.RegionSubscriptionStatusReady {
			backend.Logger.Error("client", "GetSubscribedRegionstakey", "fetching the subscribed region for regioname: "+*item.RegionName)
			subscribedRegions = append(subscribedRegions, *item.RegionName)
		}
	}

	if len(subscribedRegions) > 1 {
		subscribedRegions = append(subscribedRegions, constants.ALL_REGION)
	}
	/* Sort regions list */
	sort.Strings(subscribedRegions)
	return subscribedRegions
}

func (o *OCIDatasource) identifyQueryType(loggingSearchQuery string) LogSearchQueryType {
	var queryType LogSearchQueryType = QueryType_Undefined

	// Determine if the specified logging query utilizes any of the mathematical query functions, see
	// https://docs.oracle.com/en-us/iaas/Content/Logging/Reference/query_language_specification.htm
	// for the full query language specification

	reAvg, _ := regexp.Compile(`avg\s*\(.+\)`)
	reSum, _ := regexp.Compile(`sum\s*\(.+\)`)
	reMin, _ := regexp.Compile(`min\s*\(.+\)`)
	reMax, _ := regexp.Compile(`max\s*\(.+\)`)

	// Regular expression to be used to determine if the query uses the rounddown() function
	reInterval, _ := regexp.Compile(`rounddown\s*\(.+\)`)

	/*
	 * There are many valid ways to use the count aggregate operator within a logging
	 * search query including:
	 *   search "<compartment>[/<log group>[/<log>]]" | count
	 *   search "<compartment>[/<log group>[/<log>]]" | summarize count()
	 *   search "<compartment>[/<log group>[/<log>]]" | summarize count() by <field1>
	 *   search "<compartment>[/<log group>[/<log>]]" | summarize count(<field1>) by <field2>
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

		// Finally check whether the query includes the rounddown() function since the
		// inclusion of this function in the query will cause the OCI Logging service
		// to return time series data in a single query response
		if reInterval.Match([]byte(loggingSearchQuery)) == true {
			queryType = QueryType_LogMetrics_TimeSeries
		} else {
			queryType = QueryType_LogMetrics_NoInterval
		}

	} else {
		queryType = QueryType_LogRecords
	}
	return queryType

}

func (o *OCIDatasource) processLogMetricTimeSeries(ctx context.Context,
	query backend.DataQuery, queryModel *models.QueryModel, fromMs int64, toMs int64, mFieldDefns map[string]*DataFieldElements, takey string) (map[string]*DataFieldElements, error) {

	var searchLogsReq models.GrafanaSearchLogsRequest
	var queryRefId string = query.RefID
	var queryPanelId string = searchLogsReq.PanelId
	var timestampFieldKey string
	// Implicit assumption that the request contains this field, must be set by the plugin frontend
	searchQuery := queryModel.QueryText
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
	req1.TimeStart = &common.SDKTime{Time: start}
	req1.TimeEnd = &common.SDKTime{Time: end}
	// Directly use the query provided by the user (where any template variable references
	// have already been replaced by the plugin frontend)
	req1.SearchQuery = common.String(searchQuery)
	// Construct the Logging service SearchLogs request structure
	request := loggingsearch.SearchLogsRequest{
		SearchLogsDetails: req1,
		Limit:             common.Int(constants.LimitPerPage),
	}
	reg := common.StringToRegion(queryModel.Region)
	// ensures it catch always the correct tenancy when computing dashboards with data coming from multiple tenancies
	if searchLogsReq.TenancyMode == "multitenancy" {
		takey = searchLogsReq.Tenancy
	}

	o.tenancyAccess[takey].loggingSearchClient.SetRegion(string(reg))

	// Perform the logs search operation
	res, err := o.tenancyAccess[takey].loggingSearchClient.SearchLogs(ctx, request)
	if err != nil {
		o.logger.Debug(fmt.Sprintf("Log search operation FAILED, panelId = %s, refId = %s, err = %s",
			queryPanelId, queryRefId, err))
		return nil, errors.Wrap(err, "error fetching logs")
	}

	// Determine how many rows were returned in the search results
	resultCount := *res.SearchResponse.Summary.ResultCount
	//*&res.SearchResponse.Results
	if resultCount > 0 {

		// Keep track of the labels to be applied to the field
		sLabelFields := make([]*models.LabelFieldMetadata, 0)

		numericFieldKey := ""
		numericFieldType := constants.ValueType_Undefined
		var timestampMs int64

		searchResultData, ok := (*res.SearchResponse.Results[0].Data).(map[string]interface{})
		if ok == true {
			if _, ok := searchResultData[constants.LogSearchResultsField_LogContent]; !ok {

				// Prepare regular expression filter once for processing all results, using
				// a raw string to simplify escaping
				reFunc, _ := regexp.Compile(`^(count|sum|avg|min|max)\s*\([^\)]*\)`)

				// If the user has defined an alias for the timestamp as part of their query, e.g.
				//   ... by rounddown(datetime, '<interval>') as interval
				// then we need to know what that alias is to know which corresponding field in the
				// log search results is the timestamp field. So check the query to see if it includes
				// an alias for the timestamp, if it does then save that alias otherwise use the
				// default timestamp name: 'datetime'
				timestampFieldKey = ""
				reTimestampAlias, _ := regexp.Compile(`rounddown\s*\([^\)]+\)\s+as\s+(?P<alias>[^,\s]+)`)
				if reTimestampAlias.Match([]byte(searchQuery)) == true {
					matches := reTimestampAlias.FindStringSubmatch(searchQuery)
					aliasIndex := reTimestampAlias.SubexpIndex("alias")
					timestampFieldKey = matches[aliasIndex]
				} else {
					timestampFieldKey = "datetime"
				}

				// If the metric generated by the search query is aliased in the logging search
				// query, e.g.
				//     ... | summarize count() as foo
				//     ... | summarize count(<field name>) as bar
				//     ... | summarize sum(<field name>) as field_sum
				// then we need to know what that alias is to know which corresponding field in the
				// log search results is the numeric metric field. So check the query to see if it
				// includes an alias for the query function result, if it does then save that alias
				// otherwise the existing logic for determining the numeric field name will apply
				reFuncResultAlias, _ := regexp.Compile(`(count|sum|avg|min|max)\s*\([^\)]*\)\s+as\s+(?P<alias>[^\s]+)`)
				if reFuncResultAlias.Match([]byte(searchQuery)) == true {
					matches := reFuncResultAlias.FindStringSubmatch(searchQuery)
					aliasIndex := reFuncResultAlias.SubexpIndex("alias")

					numericFieldKey = matches[aliasIndex]
					numericFieldType = constants.ValueType_Float64

					o.logger.Debug("Search query DID match query aggregation function alias regex", "alias", numericFieldKey)
				}

				mLogTimeSeriesResults := make(map[int64]*LogTimeSeriesResult)
				// Keep track of the unique timestamps encountered so the results timestamp
				// group map can be walked in sorted order later
				sTimestampKeys := make([]int64, 0)

				// Note that unless the user specifically sorts the results of the logging search
				// query on the date/timestamp field, e.g.
				//     ... | <aggregation operation> by rounddown(datetime, '5m') as interval | sort by interval
				// there is NO guarantee that the results returned by the OCI Logging service are
				// time ordered. While Grafana handles the out of order data situation sometimes
				// it doesn't do so consistently and when it doesn't work the resulting visualization is
				// unusable.
				//
				// One option would be to add a sort clause to the user's search query but this could be
				// fairly complicated given the extreme variability of logging search queries that a user
				// could provide (when you consider they might already have a sort clause in the query). In
				// addition the notion of modifying user input without their approval or understanding is
				// sub-optimal. So to work around this issue, the following logic walks the logging search
				// results one result at a time extracting the timestamp field for each result and building
				// a results timestamp group map where each entry contains a map of corresponding metric values
				// for that timestamp. The keys of the results timestamp group map are then sorted so the
				// metric data is placed in the data frame to be provided to Grafana in time sorted order.

				for rowCount, logSearchResult := range res.SearchResponse.Results {
					searchResultData, ok := (*logSearchResult.Data).(map[string]interface{})
					if ok == true {
						if timestampFloat, ok := searchResultData[timestampFieldKey].(float64); ok {
							timestampMs = int64(timestampFloat)

							// Check if a results timestamp group map entry does not exist for the current
							// timestamp in which case create a new map entry and save a pointer to the
							// log search results. Otherwise add the search result fields to the existing
							// timestamp group map entry
							if _, ok = mLogTimeSeriesResults[timestampMs]; !ok {
								var tempTimestampResults LogTimeSeriesResult
								tempTimestampResults.TimestampMs = timestampMs
								tempTimestampResults.mMetricResults = make([]*map[string]interface{}, 0)
								mLogTimeSeriesResults[timestampMs] = &tempTimestampResults

								sTimestampKeys = append(sTimestampKeys, timestampMs)
							}
							mLogTimeSeriesResults[timestampMs].mMetricResults =
								append(mLogTimeSeriesResults[timestampMs].mMetricResults, &searchResultData)
						} else {
							o.logger.Error("Unable to extract timestamp value from log row",
								"panelId", queryPanelId, "refId", queryRefId, "timestampFieldKey", timestampFieldKey,
								"rowCount", rowCount)
						}
					} else {
						o.logger.Error("Unable to map result data elements",
							"panelId", queryPanelId, "refId", queryRefId, "row", rowCount)
					}
				}
				// Now sort the list of timestamps so the map of results timestamp groups can be walked in
				// sorted time order
				sort.Slice(sTimestampKeys, func(i, j int) bool { return sTimestampKeys[i] < sTimestampKeys[j] })

				var fieldDefn *DataFieldElements
				var timestampResults *LogTimeSeriesResult
				var searchResultFields map[string]interface{}

				tgtNumRows := len(mLogTimeSeriesResults)
				// Now that we have the results sorted by time, populate the data field definition
				// structures to be used to construct the data frame that will be passed to the
				// plugin frontend (and ultimately Grafana)
				for rowCount, timestampMs := range sTimestampKeys {
					timestampResults = mLogTimeSeriesResults[timestampMs]

					if rowCount == 0 {
						// Loop through the keys for the first log results entry for the associated
						// timestamp to determine what kind of fields we have in the results
						for key, value := range *timestampResults.mMetricResults[0] {
							// Check whether the key contains one of the aggregation functions
							if key == "count" {
								numericFieldKey = key
								// In the JSON content for the log record the count appears as an
								// integer but when converted becomes a float value
								numericFieldType = constants.ValueType_Float64

								// If the numeric field key was not already identified from the search
								// query and the current key contains one of the known query mathematical
								// functions then this is the numeric field in the log search results
							} else if numericFieldKey == "" && reFunc.Match([]byte(key)) == true {
								numericFieldKey = key
								// The order of these checks is important since integer fields will likely
								// be convertible as floating point values
								if _, ok := value.(int); ok {
									numericFieldType = constants.ValueType_Int
								} else if _, ok := value.(float64); ok {
									numericFieldType = constants.ValueType_Float64
								} else {
									o.logger.Error("Unable to determine numeric data type for field value",
										"panelId", queryPanelId, "refId", queryRefId, "value", value)
									numericFieldType = constants.ValueType_Undefined
								}

								// If the current key is not for the timestamp or metric field then treat
								// it is a label field
							} else if key != timestampFieldKey && key != numericFieldKey {
								// Save the information about the label field
								labelFieldMetadata := models.LabelFieldMetadata{
									LabelName:  key,
									LabelValue: "",
								}
								sLabelFields = append(sLabelFields, &labelFieldMetadata)
							}
						}
					} // end if first row

					// There should always be a timestamp field so go ahead and process that
					// field first
					fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, tgtNumRows,
						timestampFieldKey, timestampFieldKey, FieldValueType(constants.ValueType_Time))
					// Convert the timestamp field value for the current results timestamp group into
					// a time.Time object and add that value to the timestamp field values
					timestamp := time.Unix(timestampMs/1000, (timestampMs%1000)*1000000).UTC()
					fieldDefn.Values.([]*time.Time)[rowCount] = &timestamp
					for _, searchResultFieldsPtr := range timestampResults.mMetricResults {
						searchResultFields = *searchResultFieldsPtr

						// Process the label fields for the log metric to generate a unique key for the
						// log metric. This logic is the same no matter the data type of the log metric
						// field
						metricFieldCombKey := numericFieldKey
						for _, labelFieldMetadata := range sLabelFields {
							var labelValueStr string
							// The label value when provided in the Field data structure is a string so just
							// output a string representation of the label field's value without worrying about
							// the actual type. However sometimes the label field may be null so handle that case
							// cleanly
							if searchResultFields[labelFieldMetadata.LabelName] != nil {
								labelValueStr = fmt.Sprintf("%v", searchResultFields[labelFieldMetadata.LabelName])
							} else {
								labelValueStr = "null"
							}
							labelFieldMetadata.LabelValue = labelValueStr
							metricFieldCombKey += "_" + labelValueStr
						}

						// Process the numeric field in the log search results
						if numericFieldType == constants.ValueType_Float64 {

							// Get or create the data field elements structure for this field
							//
							// NOTE: Passing an empty string for the field name for now until
							// the feature enhancement which allows the user to control the
							// visualization legend is implemented and it is determined whether
							// the field name is still applicable. Same comment applies to the
							// next call to this function
							fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, tgtNumRows,
								metricFieldCombKey, "", FieldValueType(constants.ValueType_Float64))
							if floatValue, ok := searchResultFields[numericFieldKey].(float64); ok {
								fieldDefn.Values.([]*float64)[rowCount] = &floatValue
							} else {
								o.logger.Error("Unable to extract float field value",
									"panelId", queryPanelId, "refId", queryRefId, "field", numericFieldKey)
							}

						} else if numericFieldType == constants.ValueType_Int {

							// Get or create the data field elements structure for this field
							fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, tgtNumRows,
								metricFieldCombKey, "", FieldValueType(constants.ValueType_Int))
							if intValue, ok := searchResultFields[numericFieldKey].(int); ok {
								fieldDefn.Values.([]*int)[rowCount] = &intValue
							} else {
								o.logger.Error("Unable to extract int value for ",
									"panelId", queryPanelId, "refId", queryRefId, "field", numericFieldKey)
							}

						} else {
							o.logger.Error("Encountered unexpected field value type for numeric results logging query",
								"panelId", queryPanelId, "refId", queryRefId)
						}
						// Populate the label values for this log metric
						for _, labelFieldMetadata := range sLabelFields {
							fieldDefn.Labels[labelFieldMetadata.LabelName] = labelFieldMetadata.LabelValue
							// Clear the label value field so the value for the label field doesn't get re-used
							// for the next result
							labelFieldMetadata.LabelValue = ""
						}
					}
					rowCount++
				}

			} else {
				o.logger.Error("Log search results should NOT contain log records",
					"panelId", queryPanelId, "refId", queryRefId)
			}
		} else {
			o.logger.Error("Unable to assert search result data is a string map",
				"panelId", queryPanelId, "refId", queryRefId)
		}
	} else { // result count is <= 0
		o.logger.Error("No results returned by query", "panelId", queryPanelId,
			"refId", queryRefId, "resultCount", *res.SearchResponse.Summary.ResultCount)
	}

	return mFieldDefns, nil
}

func (o *OCIDatasource) processLogMetrics(ctx context.Context,
	query backend.DataQuery, queryModel *models.QueryModel, fromMs int64, toMs int64, mFieldDefns map[string]*DataFieldElements, takey string) (map[string]*DataFieldElements, error) {

	var searchLogsReq models.GrafanaSearchLogsRequest
	var numDataPoints int32
	var intervalMs float64
	var queryRefId string = query.RefID
	var queryPanelId string = searchLogsReq.PanelId

	// Implicit assumption that the request contains this field, must be set by the plugin frontend
	searchQuery := queryModel.QueryText
	o.logger.Debug("Processing log metrics search query", "panelId", queryPanelId, "refId", queryRefId,
		"query", searchQuery, "from", query.TimeRange.From.UTC(), "to", query.TimeRange.To.UTC())

	// Check the max data points value set within the query options element of the data panel to use that
	// as guidance for the number of data points to be returned. However the default value provided for the
	// max data points by Grafana is typically very high (800-1000) which is going to lead to way too many
	// logging search queries and thus a poor user experience. So the provided max data points value will be
	// used if it is less than or equal to  our max log metrics data points limit or if there is no value
	// then use our defined default log metrics data points value. Otherwise use the value set by the user
	// in the data panel
	if searchLogsReq.MaxDataPoints >= constants.MaxLogMetricsDataPoints {
		numDataPoints = constants.MaxLogMetricsDataPoints
	} else if searchLogsReq.MaxDataPoints <= 0 {
		numDataPoints = constants.DefaultLogMetricsDataPoints
	} else if searchLogsReq.MaxDataPoints < constants.MinLogMetricsDataPoints {
		numDataPoints = constants.MinLogMetricsDataPoints
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

	// Keep track of the labels to be applied to the field
	sLabelFields := make([]*models.LabelFieldMetadata, 0)

	numericFieldKey := ""
	numericFieldType := constants.ValueType_Undefined

	// If the metric generated by the search query is aliased in the logging search
	// query, e.g.
	//     ... | summarize count() as foo
	//     ... | summarize count(<field name>) as bar
	//     ... | summarize sum(<field name>) as field_sum
	// then we need to know what that alias is to know which corresponding field in the
	// log search results is the numeric metric field. So check the query to see if it
	// includes an alias for the query function result, if it does then save that alias
	// otherwise the existing logic for determining the numeric field name will apply.
	reFuncResultAlias, _ := regexp.Compile(`(count|sum|avg|min|max)\s*\([^\)]*\)\s+as\s+(?P<alias>[^\s]+)`)
	if reFuncResultAlias.Match([]byte(searchQuery)) == true {
		matches := reFuncResultAlias.FindStringSubmatch(searchQuery)
		aliasIndex := reFuncResultAlias.SubexpIndex("alias")

		numericFieldKey = matches[aliasIndex]
		numericFieldType = constants.ValueType_Float64
		o.logger.Error("Search query DID match query aggregation function alias regex", "alias", numericFieldKey)
	}

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
			Limit:             common.Int(constants.LimitPerPage),
		}
		reg := common.StringToRegion(queryModel.Region)
		// ensures it catch always the correct tenancy when computing dashboards with data coming from multiple tenancies
		if searchLogsReq.TenancyMode == "multitenancy" {
			takey = searchLogsReq.Tenancy
		}

		o.tenancyAccess[takey].loggingSearchClient.SetRegion(string(reg))

		// Perform the logs search operation
		res, err := o.tenancyAccess[takey].loggingSearchClient.SearchLogs(ctx, request)
		if err != nil {
			o.logger.Debug(fmt.Sprintf("Log search operation FAILED, panelId = %s, refId = %s, err = %s",
				queryPanelId, queryRefId, err))
			return nil, errors.Wrap(err, "error fetching logs")
		}
		o.logger.Debug("Log search operation SUCCEEDED", "panelId", queryPanelId, "refId", queryRefId,
			"interval", intervalCnt)

		// Determine how many rows were returned in the search results
		resultCount := *res.SearchResponse.Summary.ResultCount

		if resultCount > 0 {

			searchResultData, ok := (*res.SearchResponse.Results[0].Data).(map[string]interface{})
			if ok == true {

				if _, ok := searchResultData[constants.LogSearchResultsField_LogContent]; !ok {

					// Prepare regular expression filter once for processing all results, using
					// a raw string to simplify escaping
					reFunc, _ := regexp.Compile(`^(count|sum|avg|min|max)\s*\([^\)]*\)`)

					var fieldDefn *DataFieldElements

					// There needs to be a timestamp field but there is none returned in the
					// logging query results, so create the timestamp field if it doesn't already
					// exist and use the end time range for the current query interval as the
					// timestamp value
					fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, int(numDataPoints),
						constants.LogSearchResponseField_timestamp, constants.LogSearchResponseField_timestamp,
						FieldValueType(constants.ValueType_Time))

					// This needs to be the 'To' time for the current interval in time.Time format
					currToTime := time.UnixMilli(currToMs).UTC()
					fieldDefn.Values.([]*time.Time)[intervalCnt] = &currToTime

					for rowCount, logSearchResult := range res.SearchResponse.Results {
						searchResultData, ok := (*logSearchResult.Data).(map[string]interface{})
						if ok == true {
							// If this is the first row for the first interval then inspect the
							// values of the elements to speed up the processing of the remaining rows
							// for all intervals. It is important to do this only for the first row of
							// all of the results otherwise the order of the label keys may be different
							// between the search results for different intervals
							if intervalCnt == 0 && rowCount == 0 {
								// Loop through the keys for the entries in the results data item
								// to determine what kind of fields we have in the results
								for key, value := range searchResultData {

									// Check whether the key contains one of the aggregation functions
									if key == "count" {
										numericFieldKey = key
										// In the JSON content for the log record the count appears as an
										// integer but when converted becomes a float value
										numericFieldType = constants.ValueType_Float64
									} else if numericFieldKey == "" && reFunc.Match([]byte(key)) == true {
										numericFieldKey = key
										// The order of these checks is important since integer fields will likely
										// be convertible as floating point values
										if _, ok := value.(int); ok {
											numericFieldType = constants.ValueType_Int
										} else if _, ok := value.(float64); ok {
											numericFieldType = constants.ValueType_Float64
										} else {
											o.logger.Error("Unable to determine numeric data type for field value",
												"panelId", queryPanelId, "refId", queryRefId, "value", value)
											numericFieldType = constants.ValueType_Undefined
										}
									} else if key != numericFieldKey {
										// Save the information about the label field
										labelFieldMetadata := models.LabelFieldMetadata{
											LabelName:  key,
											LabelValue: "",
										}
										sLabelFields = append(sLabelFields, &labelFieldMetadata)
									}
								}
							} // end if first row

							// Process the label fields for the log metric to generate a unique key for the
							// log metric. This logic is the same no matter the data type of the log metric
							// field
							metricFieldCombKey := numericFieldKey
							for _, labelFieldMetadata := range sLabelFields {
								var labelValueStr string
								// The label value when provided in the Field data structure is a string so just
								// output a string representation of the label field's value without worrying about
								// the actual type. However sometimes the label fiel may be null so handle that case
								// cleanly
								if searchResultData[labelFieldMetadata.LabelName] != nil {
									labelValueStr = fmt.Sprintf("%v", searchResultData[labelFieldMetadata.LabelName])
								} else {
									labelValueStr = "null"
								}
								labelFieldMetadata.LabelValue = labelValueStr
								metricFieldCombKey += "_" + labelValueStr
							}

							// Process the numeric field in the log search results
							if numericFieldType == constants.ValueType_Float64 {

								// Get or create the data field elements structure for this field
								//
								// NOTE: Passing an empty string for the field name for now until
								// the feature enhancement which allows the user to control the
								// visualization legend is implemented and it is determined whether
								// the field name is still applicable. Same comment applies to the
								// next call to this function
								fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, int(numDataPoints),
									metricFieldCombKey, "", FieldValueType(constants.ValueType_Float64))

								if floatValue, ok := searchResultData[numericFieldKey].(float64); ok {
									fieldDefn.Values.([]*float64)[intervalCnt] = &floatValue
								} else {
									o.logger.Error("Unable to extract float field value",
										"panelId", queryPanelId, "refId", queryRefId, "field", numericFieldKey)
								}

							} else if numericFieldType == constants.ValueType_Int {

								// Get or create the data field elements structure for this field
								fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, int(numDataPoints),
									metricFieldCombKey, "", FieldValueType(constants.ValueType_Int))

								if intValue, ok := searchResultData[numericFieldKey].(int); ok {
									fieldDefn.Values.([]*int)[intervalCnt] = &intValue
								} else {
									o.logger.Error("Unable to extract int value for ",
										"panelId", queryPanelId, "refId", queryRefId, "field", numericFieldKey)
								}

							} else {
								o.logger.Debug("Encountered unexpected field value type for numeric results logging query",
									"panelId", queryPanelId, "refId", queryRefId)
							}

							// Populate the label values for this log metric
							for _, labelFieldMetadata := range sLabelFields {
								fieldDefn.Labels[labelFieldMetadata.LabelName] = labelFieldMetadata.LabelValue
								// Clear the label value field so the value for the label field doesn't get re-used
								// for the next result
								labelFieldMetadata.LabelValue = ""
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

	return mFieldDefns, nil
}

func (o *OCIDatasource) processLogRecords(ctx context.Context,
	query backend.DataQuery, queryModel *models.QueryModel, fromMs int64, toMs int64, mFieldDefns map[string]*DataFieldElements, takey string) (map[string]*DataFieldElements, error) {

	var searchLogsReq models.GrafanaSearchLogsRequest
	var queryRefId string = query.RefID
	var queryPanelId string = searchLogsReq.PanelId
	var numpage = 1
	var indexCountPag = 0
	// Implicit assumption that the request contains this field, must be set by the plugin frontend
	searchQuery := queryModel.QueryText
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
		Limit:             common.Int(constants.LimitPerPage),
	}
	reg := common.StringToRegion(queryModel.Region)
	o.tenancyAccess[takey].loggingSearchClient.SetRegion(string(reg))
	// Perform the logs search operation
	for res, err := o.tenancyAccess[takey].loggingSearchClient.SearchLogs(ctx, request); ; res, err = o.tenancyAccess[takey].loggingSearchClient.SearchLogs(ctx, request) {
		if err != nil {
			o.logger.Debug(fmt.Sprintf("Log search operation FAILED, queryPanelId = %s, refId = %s, err = %s",
				queryPanelId, queryRefId, err))
			return nil, errors.Wrap(err, "error fetching logs")
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
					if logContent, ok := searchResultData[constants.LogSearchResultsField_LogContent]; ok {
						mLogContent, ok := logContent.(map[string]interface{})
						if ok == true {
							for key, value := range mLogContent {

								// Only three special case fields within a log record: 1) time, 2) data, and 3) oracle
								// Treat all other logContent fields as strings
								if key == constants.LogSearchResultsField_Time {
									fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, numMaxResults,
										constants.LogSearchResponseField_timestamp, constants.LogSearchResponseField_timestamp,
										FieldValueType(constants.ValueType_Time))
									timestamp, errStr := time.Parse(time.RFC3339, value.(string))
									if errStr != nil {
										o.logger.Debug("Error parsing timestamp string", "panelId", queryPanelId,
											"refId", queryRefId, constants.LogSearchResponseField_timestamp,
											mLogContent[constants.LogSearchResultsField_Time],
											"error", errStr)
									}
									fieldDefn.Values.([]*time.Time)[indexCountPag] = &timestamp
								} else if key == constants.LogSearchResultsField_Data || key == constants.LogSearchResultsField_Oracle {
									var logData string = ""
									fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, numMaxResults,
										key, key, FieldValueType(constants.ValueType_String))

									logJSON, marerr := json.Marshal(value)
									if marerr == nil {
										logData = string(logJSON)
									} else {
										o.logger.Debug("Error marshalling log record data string, log data variable type",
											"panelId", queryPanelId, "refId", queryRefId, "type", fmt.Sprintf("%T", value))
										logData = "UNKNOWN"
									}
									fieldDefn.Values.([]*string)[indexCountPag] = &logData

									// Skip the subject field since it seems to always be an empty string
									// For all other keys treat them generically as string type
								} else if key != constants.LogSearchResultsField_Subject {
									var stringFieldValue string
									fieldDefn = nil

									if stringFieldValue, ok = value.(string); ok {
										// If the field value is non-zero length string then proceed to get/create the data
										// field definition. But if the field value is a zero length string then skip
										// creating the data field definition, this is to avoid creating a data field for a
										// log record field that is always empty.
										if len(stringFieldValue) > 0 {
											fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, numMaxResults,
												key, key, FieldValueType(constants.ValueType_String))
										}
									} else {
										o.logger.Debug("Error parsing string field value", "panelId", queryPanelId,
											"refId", queryRefId, "key", key, "value", value)
										fieldDefn = o.getCreateDataFieldElemsForField(mFieldDefns, numMaxResults,
											key, key, FieldValueType(constants.ValueType_String))
										stringFieldValue = "UNKNOWN"
									}
									if fieldDefn != nil {
										fieldDefn.Values.([]*string)[indexCountPag] = &stringFieldValue
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
				indexCountPag++
			}

		} else {
			o.logger.Warn("Logging search query returned no results", "panelId", queryPanelId,
				"refId", queryRefId)
		}
		if res.OpcNextPage != nil && numpage < MaxPagesToFetch {
			// if there are more items in next page, fetch items from next page
			request.Page = res.OpcNextPage
			numpage++
		} else {
			o.logger.Debug("Reducing data field values", "resultsCount", indexCountPag)
			for _, dataFieldDefn := range mFieldDefns {
				if dataFieldDefn.Type == FieldValueType(constants.ValueType_Time) {
					timeValuesSlice, _ := dataFieldDefn.Values.([]*time.Time)
					dataFieldDefn.Values = timeValuesSlice[:indexCountPag]
				} else if dataFieldDefn.Type == FieldValueType(constants.ValueType_Float64) {
					floatValuesSlice, _ := dataFieldDefn.Values.([]*float64)
					dataFieldDefn.Values = floatValuesSlice[:indexCountPag]
				} else if dataFieldDefn.Type == FieldValueType(constants.ValueType_Int) {
					intValuesSlice, _ := dataFieldDefn.Values.([]*int)
					dataFieldDefn.Values = intValuesSlice[:indexCountPag]
				} else { // Treat all other data types as a string (including string fields)
					stringValuesSlice, _ := dataFieldDefn.Values.([]*string)
					dataFieldDefn.Values = stringValuesSlice[:indexCountPag]
				}
			}
			// no more result, break the loop
			break
		}
	}
	return mFieldDefns, nil
}
