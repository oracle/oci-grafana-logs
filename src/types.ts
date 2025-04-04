/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/

import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';


export enum DefaultOCIOptions {
  ConfigProfile = 'DEFAULT',
}

export const DEFAULT_TENANCY = "DEFAULT/";
export const regionsQueryRegex = /^regions\(\s*(\".+\"|\'.+\'|\$\w+)\s*\)|^regions\(\)\s*/;
export const tenanciesQueryRegex = /^tenancies\(\)\s*/;
// export const generalQueryRegex = /^search\(\s*(\".+\"|\'.+\'|\$\w+)\s*,\s*(\".+\"|\'.+\'|\$\w+)\s*(?:,\s*(\".+\"|\'.+\'|\$\w+)\s*)?\)/;
// export const generalQueryRegex = /^search\(\s*(\".+\"|\'.+\'|\$\w+)\s*,\s*(\".+\"|\'.+\'|\$\w+)\s*,\s*(\".+\"|\'.+\'|\$\w+)\s*(?:,\s*(\".+\"|\'.+\'|\$\w+)\s*)?\)/;

export const generalQueryRegex = /^search\(\s*(\".+\"|\'.+\'|\$\w+)\s*,\s*(\".+\"|\'.+\'|\$\w+)\s*(?:,\s*(\".+\"|\'.+\'|\$\w+))?\s*(?:,\s*(\".+\"|\'.+\'|\$\w+))?\)/;

/**
 * Enum representing the different OCI resource API calls.
 */
export enum OCIResourceCall {
  /**
  * Represents the API call to list tenancies.
  */
  Tenancies = 'tenancies',
  /**
  * Represents the API call to list regions.
  */
  Regions = 'regions',
  /**
  * Represents the API call to get log query.
  */
  getQuery = 'getquery',
}

/**
* Enum representing the different query placeholders used in the UI.
*/
export enum QueryPlaceholder {
    /**
    * Placeholder for the tenancy selection.
    */
	Tenancy = 'select tenancy',
	/**
    * Placeholder for the compartment selection.
    */
	Compartment = 'select compartment',
	/**
    * Placeholder for the region selection.
    */
	Region = 'select region',
  }

/**
 * The OCIQuery interface represents a query object for Oracle Cloud Infrastructure (OCI) data queries.
 * It extends the DataQuery interface and includes additional properties specific to OCI query execution.
 * 
 * Properties:
 * - searchQuery (optional): A string representing a search query that can be used to filter the data.
 * - query (optional): A string representing the actual query to be executed.
 * - tenancyName: A string representing the name of the tenancy in OCI.
 * - tenancy: A string representing the OCID of the tenancy in OCI.
 * - tenancymode: A string that indicates the mode of tenancy (e.g., "single" or "multi").
 * - regions (optional): An array or object that contains information about available regions in OCI.
 * - region (optional): A string representing a specific region in OCI.
 */
export interface OCIQuery extends DataQuery {
  searchQuery?: string;
  query?: string;
  tenancyName: string;
  tenancy: string;
  tenancymode: string;
  regions?: any;
  region?: string;
}

/**
* These are options configured for each DataSource instance
*/
export interface OCIDataSourceOptions extends DataSourceJsonData {
	tenancyName: string; // name of the base tenancy
	environment?: string; // oci-cli, oci-instance
	tenancymode?: string; // multi-profile, cross-tenancy-policy
	xtenancy0: string;

	addon1: boolean;
	addon2: boolean;
	addon3: boolean;
	addon4: boolean;

	customregionbool0: boolean;
	customregionbool1: boolean;
	customregionbool2: boolean;
	customregionbool3: boolean;
	customregionbool4: boolean;
	customregionbool5: boolean;

	customregion0: string
	customregion1: string
	customregion2: string
	customregion3: string
	customregion4: string	
	customregion5: string 

	profile0: string;
	region0: string;

	profile1: string;
	region1: string;

	profile2: string;
	region2: string;

	profile3: string;
	region3: string;

	profile4: string;
	region4: string;

	profile5: string;
	region5: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface OCISecureJsonData {
	tenancy0: string;
	user0: string;
	privkey0: string;
	fingerprint0: string;
	customdomain0: string

	tenancy1: string;
	user1: string;
	fingerprint1: string;
	privkey1: string;
	customdomain1: string

	tenancy2: string;
	user2: string;
	fingerprint2: string;
	privkey2: string;
	customdomain2: string

	tenancy3: string;
	user3: string;
	fingerprint3: string;
	privkey3: string;
	customdomain3: string

	tenancy4: string;
	user4: string;
	fingerprint4: string;
	privkey4: string;
	customdomain4: string

	tenancy5: string;
	user5: string;
	fingerprint5: string;
	privkey5: string;
	customdomain5: string
}
