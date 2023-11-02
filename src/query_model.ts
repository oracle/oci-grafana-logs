/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/

import { OCIQuery, QueryPlaceholder } from './types';
import { ScopedVars } from '@grafana/data';
import { TemplateSrv } from '@grafana/runtime';

export default class QueryModel {
  target: OCIQuery;
  templateSrv: any;
  scopedVars: any;
  refId?: string;

  constructor(incomingQuery: OCIQuery, templateSrv?: TemplateSrv, scopedVars?: ScopedVars) {
    this.target = incomingQuery;
    this.templateSrv = templateSrv;
    this.scopedVars = scopedVars;

    this.target.tenancy = incomingQuery.tenancy || QueryPlaceholder.Tenancy;
    this.target.region = incomingQuery.region || QueryPlaceholder.Region;

    this.target.hide = incomingQuery.hide ?? false;
    if (this.target.tenancy === QueryPlaceholder.Tenancy) {
        this.target.tenancy = 'DEFAULT/';
    }
  }

  isQueryReady() {
    // check if the query is ready to be built
    if (
      this.target.tenancy === QueryPlaceholder.Tenancy ||
      this.target.region === QueryPlaceholder.Region
    ) {
      return false;
    }

    return true;
  }
}
