// Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package plugin

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/dgraph-io/ristretto"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/logging"
	"github.com/oracle/oci-go-sdk/v65/loggingsearch"
	"github.com/oracle/oci-go-sdk/v65/monitoring"

	"github.com/oracle/oci-grafana-logs/pkg/plugin/models"
)

const MaxPagesToFetch = 20
const SingleTenancyKey = "DEFAULT/"
const NoTenancy = "NoTenancy"

var EmptyString string = ""
var EmptyKeyPass *string = &EmptyString

var (
	cacheRefreshTime = time.Minute // how often to refresh our compartmentID cache
	re               = regexp.MustCompile(`(?m)\w+Name`)
)

type TenancyAccess struct {
	monitoringClient monitoring.MonitoringClient
	identityClient   identity.IdentityClient
	config           common.ConfigurationProvider
}
type logTenancyAccess struct {
	loggingSearchClient     loggingsearch.LogSearchClient
	loggingManagementClient logging.LoggingManagementClient
	identityClient          identity.IdentityClient
	config                  common.ConfigurationProvider
}

type OCIDatasource struct {
	tenancyAccess    map[string]*logTenancyAccess
	monTenancyAccess map[string]*TenancyAccess
	logger           log.Logger
	nameToOCID       map[string]string
	timeCacheUpdated time.Time
	backend.CallResourceHandler
	// clients  *client.OCIClients
	settings *models.OCIDatasourceSettings
	cache    *ristretto.Cache
}

type OCIConfigFile struct {
	tenancyocid map[string]string
	region      map[string]string
	user        map[string]string
	fingerprint map[string]string
	privkey     map[string]string
	privkeypass map[string]*string
	logger      log.Logger
}

type OCISecuredSettings struct {
	Profile_0     string `json:"profile0,omitempty"`
	Tenancy_0     string `json:"tenancy0,omitempty"`
	Region_0      string `json:"region0,omitempty"`
	User_0        string `json:"user0,omitempty"`
	Privkey_0     string `json:"privkey0,omitempty"`
	Fingerprint_0 string `json:"fingerprint0,omitempty"`

	Profile_1     string `json:"profile1,omitempty"`
	Tenancy_1     string `json:"tenancy1,omitempty"`
	Region_1      string `json:"region1,omitempty"`
	User_1        string `json:"user1,omitempty"`
	Fingerprint_1 string `json:"fingerprint1,omitempty"`
	Privkey_1     string `json:"privkey1,omitempty"`

	Profile_2     string `json:"profile2,omitempty"`
	Tenancy_2     string `json:"tenancy2,omitempty"`
	Region_2      string `json:"region2,omitempty"`
	User_2        string `json:"user2,omitempty"`
	Fingerprint_2 string `json:"fingerprint2,omitempty"`
	Privkey_2     string `json:"privkey2,omitempty"`

	Profile_3     string `json:"profile3,omitempty"`
	Tenancy_3     string `json:"tenancy3,omitempty"`
	Region_3      string `json:"region3,omitempty"`
	User_3        string `json:"user3,omitempty"`
	Fingerprint_3 string `json:"fingerprint3,omitempty"`
	Privkey_3     string `json:"privkey3,omitempty"`

	Profile_4     string `json:"profile4,omitempty"`
	Tenancy_4     string `json:"tenancy4,omitempty"`
	Region_4      string `json:"region4,omitempty"`
	User_4        string `json:"user4,omitempty"`
	Fingerprint_4 string `json:"fingerprint4,omitempty"`
	Privkey_4     string `json:"privkey4,omitempty"`

	Profile_5     string `json:"profile5,omitempty"`
	Tenancy_5     string `json:"tenancy5,omitempty"`
	Region_5      string `json:"region5,omitempty"`
	User_5        string `json:"user5,omitempty"`
	Fingerprint_5 string `json:"fingerprint5,omitempty"`
	Privkey_5     string `json:"privkey5,omitempty"`
}

// NewOCIConfigFile - constructor
func NewOCIConfigFile() *OCIConfigFile {
	return &OCIConfigFile{
		tenancyocid: make(map[string]string),
		region:      make(map[string]string),
		user:        make(map[string]string),
		fingerprint: make(map[string]string),
		privkey:     make(map[string]string),
		privkeypass: make(map[string]*string),
		logger:      log.DefaultLogger,
	}
}

// NewOCIDatasourceConstructor - constructor
func NewOCIDatasourceConstructor() *OCIDatasource {
	return &OCIDatasource{
		tenancyAccess:    make(map[string]*logTenancyAccess),
		monTenancyAccess: make(map[string]*TenancyAccess),
		logger:           log.DefaultLogger,
		nameToOCID:       make(map[string]string),
	}
}

func NewOCIDatasource(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	backend.Logger.Debug("plugin", "NewOCIDatasource", settings.ID)
	backend.Logger.Error("plugin", "NewOCIDatasource", "In NewOCIDatasource")
	o := NewOCIDatasourceConstructor()
	dsSettings := &models.OCIDatasourceSettings{}

	if err := dsSettings.Load(settings); err != nil {
		backend.Logger.Error("plugin", "NewOCIDatasource", "failed to load oci datasource settings: "+err.Error())
		return nil, err
	}
	o.settings = dsSettings

	backend.Logger.Error("plugin", "dsSettings.Environment", "dsSettings.Environment: "+dsSettings.Environment)
	backend.Logger.Error("plugin", "dsSettings.TenancyMode", "dsSettings.TenancyMode: "+dsSettings.TenancyMode)
	backend.Logger.Error("plugin", "to.enancyAccess", o.tenancyAccess)

	if len(o.tenancyAccess) == 0 {

		err := o.getConfigProvider(dsSettings.Environment, dsSettings.TenancyMode, settings)
		if err != nil {
			return nil, errors.New("broken environment")
		}
	}

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     false,
	})
	if err != nil {
		backend.Logger.Error("plugin", "NewOCIDatasource", "failed to create cache: "+err.Error())
		return nil, err
	}
	o.cache = cache

	mux := http.NewServeMux()
	o.registerRoutes(mux)
	o.CallResourceHandler = httpadapter.New(mux)

	return o, nil
}

func (o *OCIDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	backend.Logger.Debug("plugin", "QueryData", req.PluginContext.DataSourceInstanceSettings.Name)

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := o.query(ctx, req.PluginContext, q)

		// saving the response in a hashmap based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

// CheckHealth Handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (o *OCIDatasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	backend.Logger.Debug("plugin", "CheckHealth", req.PluginContext.PluginID)

	hRes := &backend.CheckHealthResult{}
	backend.Logger.Error("plugin", "CheckHealth", "In Health Check")
	backend.Logger.Error("plugin", "CheckHealth", ctx)
	//backend.Logger.Error("plugin", "CheckHealth", o.tenancyAccess["DEFAULT"].config.Region())
	if err := o.TestConnectivity(ctx); err != nil {
		hRes.Status = backend.HealthStatusError
		hRes.Message = err.Error()
		backend.Logger.Error("plugin", "CheckHealth", err)

		return hRes, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Success",
	}, nil
}

// OCILoadSettings will read and validate Settings from the DataSourceConfig
func OCILoadSettings(req backend.DataSourceInstanceSettings) (*OCIConfigFile, error) {
	q := NewOCIConfigFile()

	// Load secured and non-secured settings
	TenancySettingsBlock := 0
	var dat OCISecuredSettings
	var nonsecdat models.OCIDatasourceSettings

	if err := json.Unmarshal(req.JSONData, &dat); err != nil {
		return nil, fmt.Errorf("can not read Secured settings: %s", err.Error())
	}

	if err := json.Unmarshal(req.JSONData, &nonsecdat); err != nil {
		return nil, fmt.Errorf("can not read settings: %s", err.Error())
	}

	// merge non secured settings into secured
	decryptedJSONData := req.DecryptedSecureJSONData
	transcode(decryptedJSONData, &dat)

	dat.Region_0 = nonsecdat.Region_0
	dat.Region_1 = nonsecdat.Region_1
	dat.Region_2 = nonsecdat.Region_2
	dat.Region_3 = nonsecdat.Region_3
	dat.Region_4 = nonsecdat.Region_4
	dat.Region_5 = nonsecdat.Region_5

	dat.Profile_0 = nonsecdat.Profile_0
	dat.Profile_1 = nonsecdat.Profile_1
	dat.Profile_2 = nonsecdat.Profile_2
	dat.Profile_3 = nonsecdat.Profile_3
	dat.Profile_4 = nonsecdat.Profile_4
	dat.Profile_5 = nonsecdat.Profile_5

	v := reflect.ValueOf(dat)
	typeOfS := v.Type()
	var key string

	for FieldIndex := 0; FieldIndex < v.NumField(); FieldIndex++ {
		splits := strings.Split(typeOfS.Field(FieldIndex).Name, "_")
		SettingsBlockIndex, interr := strconv.Atoi(splits[1])
		if interr != nil {
			return nil, fmt.Errorf("can not read settings: %s", interr.Error())
		}

		if SettingsBlockIndex == TenancySettingsBlock {
			if splits[0] == "Profile" {
				if v.Field(FieldIndex).Interface() != "" {
					key = fmt.Sprintf("%v", v.Field(FieldIndex).Interface())
				} else {
					return q, nil
				}
			} else {
				switch value := v.Field(FieldIndex).Interface(); strings.ToLower(splits[0]) {
				case "tenancy":
					q.tenancyocid[key] = fmt.Sprintf("%v", value)
				case "region":
					q.region[key] = fmt.Sprintf("%v", value)
				case "user":
					q.user[key] = fmt.Sprintf("%v", value)
				case "privkey":
					q.privkey[key] = fmt.Sprintf("%v", value)
				case "fingerprint":
					q.fingerprint[key] = fmt.Sprintf("%v", value)
				case "privkeypass":
					q.privkeypass[key] = EmptyKeyPass
				}
			}
		} else {
			TenancySettingsBlock++
			FieldIndex--
		}
	}
	return q, nil
}

func (o *OCIDatasource) getConfigProvider(environment string, tenancymode string, req backend.DataSourceInstanceSettings) error {

	switch environment {
	case "local":
		log.DefaultLogger.Debug("Configuring using User Principals")
		q, err := OCILoadSettings(req)
		if err != nil {
			return errors.New("Error Loading config settings")
		}
		for key, _ := range q.tenancyocid {
			var configProvider common.ConfigurationProvider
			// test if PEM key is valid
			block, _ := pem.Decode([]byte(q.privkey[key]))
			if block == nil {
				return errors.New("error with Private Key")
			}
			log.DefaultLogger.Error("q.tenancyocid[key]: " + q.tenancyocid[key])
			log.DefaultLogger.Error("q.user[key]: " + q.user[key])
			log.DefaultLogger.Error("q.region[key]: " + q.region[key])
			log.DefaultLogger.Error("q.fingerprint[key]: " + q.fingerprint[key])
			log.DefaultLogger.Error("q.privkey[key]: " + q.privkey[key])
			configProvider = common.NewRawConfigurationProvider(q.tenancyocid[key], q.user[key], q.region[key], q.fingerprint[key], q.privkey[key], q.privkeypass[key])

			// creating oci monitoring client
			//mrp := clientRetryPolicy()
			monitoringClient, err := monitoring.NewMonitoringClientWithConfigurationProvider(configProvider)
			loggingSearchClient, err := loggingsearch.NewLogSearchClientWithConfigurationProvider(configProvider)
			if err != nil {
				o.logger.Error("Error with config:" + key)
				return errors.New("error with loggingSearchClient")
			}
			loggingManagementClient, err := logging.NewLoggingManagementClientWithConfigurationProvider(configProvider)
			if err != nil {
				o.logger.Error("Error with config:" + key)
				return errors.New("Error creating loggingManagement client")
			}
			identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
			if err != nil {
				return errors.Wrap(err, "Error creating identity client")
			}
			tenancyocid, err := configProvider.TenancyOCID()
			if err != nil {
				return errors.New("error with TenancyOCID")
			}

			if tenancymode == "multitenancy" {
				o.monTenancyAccess[key+"/"+tenancyocid] = &TenancyAccess{monitoringClient, identityClient, configProvider}
				o.tenancyAccess[key+"/"+tenancyocid] = &logTenancyAccess{loggingSearchClient, loggingManagementClient, identityClient, configProvider}
				log.DefaultLogger.Error("Multitenancy:: tenancyAccess: ", o.tenancyAccess[key+"/"+tenancyocid])
			} else {
				o.monTenancyAccess[SingleTenancyKey] = &TenancyAccess{monitoringClient, identityClient, configProvider}
				o.tenancyAccess[SingleTenancyKey] = &logTenancyAccess{loggingSearchClient, loggingManagementClient, identityClient, configProvider}
			}
		}
		return nil

	case "OCI Instance":
		log.DefaultLogger.Error("Configuring using Instance Principal")
		var configProvider common.ConfigurationProvider
		configProvider, err := auth.InstancePrincipalConfigurationProvider()
		//log.DefaultLogger.Error("configProvider: " + configProvider)
		if err != nil {
			return errors.New("error with instance principals")
		}
		//monitoringClient, err := monitoring.NewMonitoringClientWithConfigurationProvider(configProvider)
		loggingSearchClient, err := loggingsearch.NewLogSearchClientWithConfigurationProvider(configProvider)
		if err != nil {
			backend.Logger.Error("Error with config:" + SingleTenancyKey)
			return errors.New("error with client")
		}
		loggingManagementClient, err := logging.NewLoggingManagementClientWithConfigurationProvider(configProvider)
		if err != nil {
			o.logger.Error("Error with config:")
			return errors.New("Error creating loggingManagement client")
		}
		identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
		if err != nil {
			return errors.New("Error creating identity client")
		}
		o.tenancyAccess[SingleTenancyKey] = &logTenancyAccess{loggingSearchClient, loggingManagementClient, identityClient, configProvider}
		return nil

	default:
		return errors.New("unknown environment type")
	}
}
