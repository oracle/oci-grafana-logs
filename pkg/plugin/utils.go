/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
 */

package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/pkg/errors"
)

// Prepare format to decode SecureJson
func transcode(in, out interface{}) {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(in)
	json.NewDecoder(buf).Decode(out)
}

// GetTenancyAccessKey retrieves the tenancy access key based on the tenancy mode.
// If the tenancy mode is "multitenancy", it uses the provided tenancyOCID as the key.
// Otherwise, it uses a predefined SingleTenancyKey.
// It logs an error if the key is invalid and returns an empty string in that case.
//
// Parameters:
//
//	tenancyOCID (string): The OCID of the tenancy.
//
// Returns:
//
//	string: The tenancy access key if valid, otherwise an empty string.
func (o *OCIDatasource) GetTenancyAccessKey(tenancyOCID string) string {

	// Determine the tenancy access key based on the tenancy mode.
	var takey string
	tenancymode := o.settings.TenancyMode
	if tenancymode == "multitenancy" {
		takey = tenancyOCID
	} else {
		takey = SingleTenancyKey
	}

	// Check if the tenancy access key is valid.
	_, ok := o.tenancyAccess[takey]
	if ok {
		backend.Logger.Debug("GetTenancyAccessKey", "GetTenancyAccessKey", "valid takey: "+takey)
	} else {
		backend.Logger.Error("GetTenancyAccessKey", "GetTenancyAccessKey", "Invalid takey: "+takey)
		return ""
	}

	// Return the tenancy access key.
	return takey
}

// FilterMap filters through a map of type map[string]interface{} and returns the first value
// found that is not associated with the keys "datetime" or "count". If a valid key-value pair
// is found, the value is returned as a string. If no such valid key is found, an error is returned.
//
// Parameters:
//   - inputMap: An interface{} that is expected to be a map of type map[string]interface{}.
//
// Returns:
//   - A string representing the first value found in the map that is not associated with
//     the keys "datetime" or "count".
//   - An error if the input is not of type map[string]interface{} or if no valid key is found.
func FilterMap(inputMap interface{}) (string, error) {
	// Check if the input is a map[string]interface{}.
	m, ok := inputMap.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("input is not a map[string]interface{}")
	}

	// Iterate over the map.
	for key, value := range m {
		// If the key is not "datetime" or "count", return the value.
		if key != "datetime" && key != "count" {
			return fmt.Sprintf("%v", value), nil
		}
	}

	// No valid key was found in the map.
	return "", errors.New("no valid key found in the map")
}

// uniqueStrings removes duplicate strings from a slice, returning a new slice
// containing only unique strings in the order they first appear.
//
// Parameters:
//   - slice: A slice of strings where duplicates will be removed.
//
// Returns:
//   - A new slice containing only the unique strings from the input slice,
//     in the order they first appeared.
func uniqueStrings(slice []string) []string {
	// Create a map to keep track of seen strings.
	seen := make(map[string]struct{})

	// Create a new list to store unique strings.
	unique := []string{}

	// Iterate over the slice.
	for _, str := range slice {
		// Check if the string has been seen before.
		if _, ok := seen[str]; !ok {
			// Add the string to the map.
			seen[str] = struct{}{}

			// Add the string to the unique list.
			unique = append(unique, str)
		}
	}

	// Return the list of unique strings.
	return unique
}

// extractField extracts the value of a specified field from a JSON string.
// It unmarshals the JSON string into a map and returns the value of the specified field.
//
// Parameters:
//   - jsonStr: A string containing the JSON data.
//   - field: The name of the field whose value should be extracted from the JSON.
//
// Returns:
//   - A string containing the value of the specified field, or an error if the field is not found
//     or if there is an issue with unmarshaling the JSON.
//
// Errors:
//   - If the JSON string cannot be unmarshaled, or if the field is not found in the JSON, an error is returned.
func extractField(jsonStr string, field string) (string, error) {
	// Unmarshal the JSON string into a map.
	var data map[string]interface{}
	field = strings.Trim(field, "\\\"")
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling JSON: %v", err)
	}

	// Check if the field exists in the map.
	value, ok := data[field]
	if !ok {
		return "", errors.New("field not found in JSON " + field)
	}

	// Convert the value to a string and return it.
	return fmt.Sprintf("%v", value), nil
}
