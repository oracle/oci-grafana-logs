/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
 */

package models

// OCIResource represents a generic OCI resource with a name and OCID.
type OCIResource struct {
	// Name is the display name of the OCI resource.
	Name string `json:"name,omitempty"`
	// OCID is the Oracle Cloud Identifier of the OCI resource.
	OCID string `json:"ocid,omitempty"`
}

// The label fields for the log metric representing key-value metadata for a label.
type LabelFieldMetadata struct {
	LabelName  string
	LabelValue string
}

// GrafanaCommonRequest represents a common request structure for Grafana queries.
type GrafanaCommonRequest struct {
	Environment string // The environment in which the request is being executed (e.g., dev, prod).
	TenancyMode string // The mode of tenancy (e.g., single-tenant or multi-tenant).
	QueryType   string // The type of query being executed (e.g., metrics, logs, alerts).
	Region      string // The OCI region where the request is being processed (e.g., us-phoenix-1).
	Tenancy     string // the actual tenancy with the format <configfile entry name/tenancyOCID>
	TenancyOCID string `json:"tenancyOCID"` // The unique OCID of the tenancy, used for identification.
}

// GrafanaSearchLogsRequest represents a request to search logs in Grafana.
type GrafanaSearchLogsRequest struct {
	GrafanaCommonRequest        // Embeds common request fields such as Environment, Region, and Tenancy.
	SearchQuery          string // The query string used to filter logs.
	MaxDataPoints        int32  // The maximum number of data points to return in the response.
	PanelId              string // The ID of the Grafana panel requesting the log data.
}
