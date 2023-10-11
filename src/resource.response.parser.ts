/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/

import _ from 'lodash';

export interface OCIResourceItem {
  name: string;
  ocid: string;
}

export interface OCIResourceGroupWithMetricNamesItem {
  resource_group: string;
  metric_names: string[];
}

export interface OCIResourceMetadataItem {
  key: string;
  values: string[];
}

export class ResponseParser {
  parseTenancies(results: any): OCIResourceItem[] {
    const tenancies: OCIResourceItem[] = [];
    if (!results) {
      return tenancies;
    }

    let tList: OCIResourceItem[] = JSON.parse(JSON.stringify(results));
    return tList;
  }

  parseRegions(results: any): string[] {
    const regions: string[] = [];
    if (!results) {
      return regions;
    }

    let rList: string[] = JSON.parse(JSON.stringify(results));
    return rList;
  }

  parseTenancyMode(results: any): string[] {
    const tenancymodes: string[] = [];
    if (!results) {
      return tenancymodes;
    }

    let rList: string[] = JSON.parse(JSON.stringify(results));
    return rList;
  }
}
