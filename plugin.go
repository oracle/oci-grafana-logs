// Copyright © 2018, 2020 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package main

import (
	"log"
	"os"

	"github.com/grafana/grafana_plugin_model/go/datasource"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

var pluginLogger = hclog.New(&hclog.LoggerOptions{
	Name:  "simple-json-datasource",
	Level: hclog.LevelFromString("DEBUG"),
})

func main() {

	f, err := os.OpenFile("/Users/athmural/oci/go/src/oci-grafana-logstext.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	logger := log.New(f, "prefix", log.LstdFlags)
	logger.Println("text to append")
	logger.Println("more text to append")

	pluginLogger.Debug("Running GRPC server")
	// fetch all out variables

	ociDatasource, err := NewOCIDatasource(pluginLogger)
	if err != nil {
		pluginLogger.Error("Unable to create plugin")
	}

	plugin.Serve(&plugin.ServeConfig{

		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "grafana_plugin_type",
			MagicCookieValue: "datasource",
		},
		Plugins: map[string]plugin.Plugin{
			"backend-datasource": &datasource.DatasourcePluginImpl{Plugin: ociDatasource},
		},

		// A non-nil value here enables gRPC serving for this plugin...
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
