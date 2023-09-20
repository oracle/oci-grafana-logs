/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
 */

package models

type OCIResource struct {
	Name string `json:"name,omitempty"`
	OCID string `json:"ocid,omitempty"`
}

type LabelFieldMetadata struct {
	LabelName  string
	LabelValue string
}

type GrafanaCommonRequest struct {
	Environment string
	TenancyMode string
	QueryType   string
	Region      string
	Tenancy     string // the actual tenancy with the format <configfile entry name/tenancyOCID>
	TenancyOCID string `json:"tenancyOCID"`
}

type GrafanaSearchLogsRequest struct {
	GrafanaCommonRequest
	SearchQuery   string
	MaxDataPoints int32
	PanelId       string
}
