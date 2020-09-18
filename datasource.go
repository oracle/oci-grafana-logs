// Copyright © 2019 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/grafana/grafana_plugin_model/go/datasource"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/common/auth"
	"github.com/oracle/oci-go-sdk/identity"
	"github.com/oracle/oci-go-sdk/loggingsearch"
	"github.com/oracle/oci-go-sdk/monitoring"
	"github.com/pkg/errors"
)

//how often to refresh our compartmentID cache
var cacheRefreshTime = time.Minute

//OCIDatasource - pulls in data from telemtry/various oci apis
type OCIDatasource struct {
	plugin.NetRPCUnsupportedPlugin
	metricsClient       monitoring.MonitoringClient
	loggingSearchClient loggingsearch.LogSearchClient
	identityClient      identity.IdentityClient
	config              common.ConfigurationProvider
	logger              hclog.Logger
	nameToOCID          map[string]string
	timeCacheUpdated    time.Time
}

//NewOCIDatasource - constructor
func NewOCIDatasource(pluginLogger hclog.Logger) (*OCIDatasource, error) {
	m := make(map[string]string)

	return &OCIDatasource{
		logger:     pluginLogger,
		nameToOCID: m,
	}, nil
}

// GrafanaOCIRequest - Query Request comning in from the front end
type GrafanaOCIRequest struct {
	GrafanaCommonRequest
	Query         string
	Resolution    string
	Namespace     string
	ResourceGroup string
}

//GrafanaSearchRequest incoming request body for search requests
type GrafanaSearchRequest struct {
	GrafanaCommonRequest
	Metric        string `json:"metric,omitempty"`
	Namespace     string
	ResourceGroup string
}

type GrafanaSearchLogsRequest struct {
	GrafanaCommonRequest
	Metric        string `json:"metric,omitempty"`
	Namespace     string
	ResourceGroup string
	SearchQuery   string
}

type GrafanaCompartmentRequest struct {
	GrafanaCommonRequest
}

// GrafanaCommonRequest - captures the common parts of the search and metricsRequests
type GrafanaCommonRequest struct {
	Compartment string
	Environment string
	QueryType   string
	Region      string
	TenancyOCID string `json:"tenancyOCID"`
	SearchQuery string
}

// Query - Determine what kind of query we're making
func (o *OCIDatasource) Query(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	var ts GrafanaSearchLogsRequest
	json.Unmarshal([]byte(tsdbReq.Queries[0].ModelJson), &ts)

	queryType := ts.QueryType
	if o.config == nil {
		configProvider, err := getConfigProvider(ts.Environment)
		if err != nil {
			return nil, errors.Wrap(err, "broken environment")
		}
		metricsClient, err := monitoring.NewMonitoringClientWithConfigurationProvider(configProvider)
		if err != nil {
			return nil, errors.New(fmt.Sprint("error with client", spew.Sdump(configProvider), err.Error()))
		}
		identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
		if err != nil {
			log.Printf("error with client")
			panic(err)
		}

		loggingSearchClient, err := loggingsearch.NewLogSearchClientWithConfigurationProvider(configProvider)
		if err != nil {
			log.Printf("error with client")
			panic(err)
		}

		o.identityClient = identityClient
		o.metricsClient = metricsClient
		o.config = configProvider
		o.loggingSearchClient = loggingSearchClient
	}

	switch queryType {
	case "compartments":
		return o.compartmentsResponse(ctx, tsdbReq)
	case "regions":
		return o.regionsResponse(ctx, tsdbReq)
	case "searchLogs":
		return o.searchLogsResponse(ctx, tsdbReq)
	case "test":
		return o.testResponse(ctx, tsdbReq)
	default:
		return o.searchLogsResponse(ctx, tsdbReq)
	}
}

func (o *OCIDatasource) testResponse(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	var ts GrafanaCommonRequest
	json.Unmarshal([]byte(tsdbReq.Queries[0].ModelJson), &ts)

	listMetrics := monitoring.ListMetricsRequest{
		CompartmentId: common.String(ts.TenancyOCID),
	}
	reg := common.StringToRegion(ts.Region)
	o.metricsClient.SetRegion(string(reg))
	res, err := o.metricsClient.ListMetrics(ctx, listMetrics)
	status := res.RawResponse.StatusCode
	if status >= 200 && status < 300 {
		return &datasource.DatasourceResponse{}, nil
	}
	return nil, errors.Wrap(err, fmt.Sprintf("list metrircs failed %s %d", spew.Sdump(res), status))
}

func (o *OCIDatasource) dimensionResponse(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	table := datasource.Table{
		Columns: []*datasource.TableColumn{
			&datasource.TableColumn{Name: "text"},
		},
		Rows: make([]*datasource.TableRow, 0),
	}

	for _, query := range tsdbReq.Queries {
		var ts GrafanaSearchRequest
		json.Unmarshal([]byte(query.ModelJson), &ts)
		reqDetails := monitoring.ListMetricsDetails{}
		reqDetails.Namespace = common.String(ts.Namespace)
		if ts.ResourceGroup != "NoResourceGroup" {
			reqDetails.ResourceGroup = common.String(ts.ResourceGroup)
		}
		reqDetails.Name = common.String(ts.Metric)
		items, err := o.searchHelper(ctx, ts.Region, ts.Compartment, reqDetails)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprint("list metrircs failed", spew.Sdump(reqDetails)))
		}
		rows := make([]*datasource.TableRow, 0)
		for _, item := range items {
			for dimension, value := range item.Dimensions {
				rows = append(rows, &datasource.TableRow{
					Values: []*datasource.RowValue{
						&datasource.RowValue{
							Kind:        datasource.RowValue_TYPE_STRING,
							StringValue: fmt.Sprintf("%s=%s", dimension, value),
						},
					},
				})
			}
		}
		table.Rows = rows
	}
	return &datasource.DatasourceResponse{
		Results: []*datasource.QueryResult{
			&datasource.QueryResult{
				RefId:  "dimensions",
				Tables: []*datasource.Table{&table},
			},
		},
	}, nil
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

const MAX_PAGES_TO_FETCH = 20

func (o *OCIDatasource) searchHelper(ctx context.Context, region, compartment string, metricDetails monitoring.ListMetricsDetails) ([]monitoring.Metric, error) {
	var items []monitoring.Metric
	var page *string

	pageNumber := 0
	for {
		reg := common.StringToRegion(region)
		o.metricsClient.SetRegion(string(reg))
		res, err := o.metricsClient.ListMetrics(ctx, monitoring.ListMetricsRequest{
			CompartmentId:      common.String(compartment),
			ListMetricsDetails: metricDetails,
			Page:               page,
		})

		if err != nil {
			return nil, errors.Wrap(err, "list metrircs failed")
		}
		items = append(items, res.Items...)
		// Only 0 - n-1  pages are to be fetched, as indexing starts from 0 (for page number
		if res.OpcNextPage == nil || pageNumber >= MAX_PAGES_TO_FETCH {
			break
		}

		page = res.OpcNextPage
		pageNumber++
	}
	return items, nil
}

func (o *OCIDatasource) compartmentsResponse(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	table := datasource.Table{
		Columns: []*datasource.TableColumn{
			&datasource.TableColumn{Name: "text"},
			&datasource.TableColumn{Name: "text"},
		},
	}
	now := time.Now()
	var ts GrafanaSearchRequest
	json.Unmarshal([]byte(tsdbReq.Queries[0].ModelJson), &ts)
	if o.timeCacheUpdated.IsZero() || now.Sub(o.timeCacheUpdated) > cacheRefreshTime {

		m, err := o.getCompartments(ctx, ts.Region, ts.TenancyOCID)
		if err != nil {
			o.logger.Error("Unable to refresh cache")
			return nil, err
		}
		o.nameToOCID = m
	}

	rows := make([]*datasource.TableRow, 0, len(o.nameToOCID))
	for name, id := range o.nameToOCID {
		val := &datasource.RowValue{
			Kind:        datasource.RowValue_TYPE_STRING,
			StringValue: name,
		}
		id := &datasource.RowValue{
			Kind:        datasource.RowValue_TYPE_STRING,
			StringValue: id,
		}

		rows = append(rows, &datasource.TableRow{
			Values: []*datasource.RowValue{
				val,
				id,
			},
		})
	}
	table.Rows = rows
	return &datasource.DatasourceResponse{
		Results: []*datasource.QueryResult{
			&datasource.QueryResult{
				RefId:  "compartments",
				Tables: []*datasource.Table{&table},
			},
		},
	}, nil
}

func (o *OCIDatasource) getCompartments(ctx context.Context, region string, rootCompartment string) (map[string]string, error) {
	m := make(map[string]string)
	m["root compartment"] = rootCompartment
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
				m[*(compartment.Name)] = *(compartment.Id)
			}
		}
		if res.OpcNextPage == nil {
			break
		}
		page = res.OpcNextPage
	}
	return m, nil
}

type responseAndQuery struct {
	ociRes monitoring.SummarizeMetricsDataResponse
	query  *datasource.Query
	err    error
}


func (o *OCIDatasource) regionsResponse(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	table := datasource.Table{
		Columns: []*datasource.TableColumn{
			&datasource.TableColumn{Name: "text"},
		},
		Rows: make([]*datasource.TableRow, 0),
	}
	for _, query := range tsdbReq.Queries {
		var ts GrafanaOCIRequest
		json.Unmarshal([]byte(query.ModelJson), &ts)
		res, err := o.identityClient.ListRegions(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error fetching regions")
		}
		rows := make([]*datasource.TableRow, 0, len(res.Items))
		for _, item := range res.Items {
			rows = append(rows, &datasource.TableRow{
				Values: []*datasource.RowValue{
					&datasource.RowValue{
						Kind:        datasource.RowValue_TYPE_STRING,
						StringValue: *(item.Name),
					},
				},
			})
		}
		table.Rows = rows
	}
	return &datasource.DatasourceResponse{
		Results: []*datasource.QueryResult{
			&datasource.QueryResult{
				RefId:  "regions",
				Tables: []*datasource.Table{&table},
			},
		},
	}, nil
}


func (o *OCIDatasource) searchLogsResponse(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	table := datasource.Table{
		Columns: []*datasource.TableColumn{
			{Name: "text"},
		},
		Rows: make([]*datasource.TableRow, 0),
	}

	rows := make([]*datasource.TableRow, 0, 2)

	for _, query := range tsdbReq.Queries {

		var ts GrafanaSearchLogsRequest
		json.Unmarshal([]byte(query.ModelJson), &ts)
		start := time.Unix(tsdbReq.TimeRange.FromEpochMs/1000, (tsdbReq.TimeRange.FromEpochMs%1000)*1000000).UTC()
		end := time.Unix(tsdbReq.TimeRange.ToEpochMs/1000, (tsdbReq.TimeRange.ToEpochMs%1000)*1000000).UTC()
		searchQuery := ts.SearchQuery

		req1 := loggingsearch.SearchLogsDetails{}

		// hardcoded for now
		req1.IsReturnFieldInfo = common.Bool(false)
		req1.TimeStart = &common.SDKTime{start}
		req1.TimeEnd = &common.SDKTime{end}
		req1.SearchQuery = common.String(searchQuery)

		request := loggingsearch.SearchLogsRequest{
			SearchLogsDetails: req1,
			Limit:             common.Int(500),
		}
		reg := common.StringToRegion(ts.Region)
		o.loggingSearchClient.SetRegion(string(reg))
		res, err := o.loggingSearchClient.SearchLogs(ctx, request)

		if err != nil {
			return nil, errors.Wrap(err, "error fetching logs")

		}

		nr, nrerr := json.Marshal(res.Results)

		if nrerr == nil {
			table.Rows = append(rows, &datasource.TableRow{
				Values: []*datasource.RowValue{
					{
						Kind:        datasource.RowValue_TYPE_STRING,
						StringValue: string(nr),
					},
				},
			})
		}

	}
	return &datasource.DatasourceResponse{
		Results: []*datasource.QueryResult{
			{
				RefId:  "searchResults",
				Tables: []*datasource.Table{&table},
			},
		},
	}, nil

}
