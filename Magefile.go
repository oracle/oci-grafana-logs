// Copyright © 2022 Oracle and/or its affiliates. All rights reserved.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

//go:build mage

package main

import (
	"log"
	"os"

	// mage:import
	build "github.com/grafana/grafana-plugin-sdk-go/build"
)

// Default configures the default target.
var Default = build.BuildAll

// Cleans up local folder
func CleanLocal() error {
	log.Println("Cleans the local folder")
	err := os.RemoveAll("oci-logs-datasource/")
	if err != nil {
		log.Println(err)
	}
	return err

}
