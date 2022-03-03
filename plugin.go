// Copyright © 2018, 2020 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// func main() {

// 	f, err := os.OpenFile("./text.log",
// 		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	defer f.Close()

// 	logger := log.New(f, "prefix", log.LstdFlags)
// 	logger.Println("text to append")
// 	logger.Println("more text to append")

// 	pluginLogger.Debug("Running GRPC server")
// 	// fetch all out variables

// 	ociDatasource, err := NewOCIDatasource(pluginLogger)
// 	if err != nil {
// 		pluginLogger.Error("Unable to create plugin")
// 	}

// 	plugin.Serve(&plugin.ServeConfig{

// 		HandshakeConfig: plugin.HandshakeConfig{
// 			ProtocolVersion:  1,
// 			MagicCookieKey:   "grafana_plugin_type",
// 			MagicCookieValue: "datasource",
// 		},
// 		Plugins: map[string]plugin.Plugin{
// 			"backend-datasource": &datasource.DatasourcePluginImpl{Plugin: ociDatasource},
// 		},

// 		// A non-nil value here enables gRPC serving for this plugin...
// 		GRPCServer: plugin.DefaultGRPCServer,
// 	})
// }

func main() {
		// Start listening to requests sent from Grafana. This call is blocking so
	// it won't finish until Grafana shuts down the process or the plugin choose
	// to exit by itself using os.Exit. Manage automatically manages life cycle
	// of datasource instances. It accepts datasource instance factory as first
	// argument. This factory will be automatically called on incoming request
	// from Grafana to create different instances of SampleDatasource (per datasource
	// ID). When datasource configuration changed Dispose method will be called and
	// new datasource instance created using NewSampleDatasource factory.
	if err := datasource.Manage("oci-logs-datasource", NewOCIDatasource, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		os.Exit(1)
	}
}