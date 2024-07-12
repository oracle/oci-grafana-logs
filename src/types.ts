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
export const generalQueryRegex = /^search\s*/;

export enum OCIResourceCall {
  Tenancies = 'tenancies',
  Regions = 'regions',
}

export enum QueryPlaceholder {
  Tenancy = 'select tenancy',
  Compartment = 'select compartment',
  Region = 'select region',
}

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

	tenancy1: string;
	user1: string;
	fingerprint1: string;
	privkey1: string;

	tenancy2: string;
	user2: string;
	fingerprint2: string;
	privkey2: string;

	tenancy3: string;
	user3: string;
	fingerprint3: string;
	privkey3: string;

	tenancy4: string;
	user4: string;
	fingerprint4: string;
	privkey4: string;

	tenancy5: string;
	user5: string;
	fingerprint5: string;
	privkey5: string;
}
