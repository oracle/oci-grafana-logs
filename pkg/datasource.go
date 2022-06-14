// Copyright © 2022 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
type GrafanaSearchLogsRequest struct {
	GrafanaCommonRequest
	SearchQuery string
}

// GrafanaCommonRequest - captures the common parts of the search and metricsRequests
type GrafanaCommonRequest struct {
	Compartment string
	Environment string
	QueryType   string
	Region      string
	TenancyOCID string `json:"tenancyOCID"`
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
		if o.cmptid == cmptId {
			m[fullyQualifiedCmptName] = cmptId
		}
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

func (o *OCIDatasource) searchLogsResponse(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	resp := backend.NewQueryDataResponse()
	for _, query := range req.Queries {
		var ts GrafanaSearchLogsRequest
		if err := json.Unmarshal(query.JSON, &ts); err != nil {
			return &backend.QueryDataResponse{}, err
		}
		fromMs := query.TimeRange.From.UnixNano() / int64(time.Millisecond)
		toMs := query.TimeRange.To.UnixNano() / int64(time.Millisecond)
		start := time.Unix(fromMs/1000, (fromMs%1000)*1000000).UTC()
		end := time.Unix(toMs/1000, (toMs%1000)*1000000).UTC()

		start = start.Truncate(time.Millisecond)
		end = end.Truncate(time.Millisecond)
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
			frame := data.NewFrame(query.RefID, data.NewField("text", nil, []string{}))
			frame.AppendRow(string(nr))
			respD := resp.Responses[query.RefID]
			respD.Frames = append(respD.Frames, frame)
			resp.Responses[query.RefID] = respD
		}
	}
	return resp, nil
}
