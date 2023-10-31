/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
 */

package constants

const (
	DEFAULT_PROFILE                = "DEFAULT/"
	DEFAULT_INSTANCE_PROFILE       = "instance_profile"
	CACHE_KEY_RESOURCE_IDS_PER_TAG = "resourceIDsPerTag"
	ALL_REGION                     = "all-subscribed-region"
	FETCH_FOR_NAMESPACE            = "namespace"
)
const MaxPagesToFetch = 20
const SingleTenancyKey = "DEFAULT/"
const NoTenancy = "NoTenancy"

// Constants for the log search result field names processed by the plugin
const LogSearchResultsField_LogContent = "logContent"
const LogSearchResultsField_Data = "data"
const LogSearchResultsField_Oracle = "oracle"
const LogSearchResultsField_Subject = "subject"
const LogSearchResultsField_Time = "time"
const LimitPerPage = 1000
const numMaxResults = (MaxPagesToFetch * LimitPerPage) + 1
const numMax1 = (MaxPagesToFetch * LimitPerPage) + 1

// Constants for the log query data response field namess
const LogSearchResponseField_timestamp = "timestamp"

const MaxLogMetricsDataPoints = 10
const DefaultLogMetricsDataPoints = 5
const MinLogMetricsDataPoints = 2

type FieldValueType int

const (
	ValueType_Undefined FieldValueType = iota
	ValueType_Float64
	ValueType_Int
	ValueType_Time
	ValueType_String
)
