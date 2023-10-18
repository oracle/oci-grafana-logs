/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
 */

package plugin

import (
	"bytes"
	"encoding/json"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Prepare format to decode SecureJson
func transcode(in, out interface{}) {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(in)
	json.NewDecoder(buf).Decode(out)
}

// Get the tenancy Access Key
func (o *OCIDatasource) GetTenancyAccessKey(tenancyOCID string) string {

	var takey string
	tenancymode := o.settings.TenancyMode
	if tenancymode == "multitenancy" {
		takey = tenancyOCID
	} else {
		takey = SingleTenancyKey
	}
	_, ok := o.tenancyAccess[takey]
	if ok {
		backend.Logger.Debug("GetTenancyAccessKey", "GetTenancyAccessKey", "valid takey: "+takey)
	} else {
		backend.Logger.Error("GetTenancyAccessKey", "GetTenancyAccessKey", "Invalid takey: "+takey)
		return ""
	}

	return takey
}
