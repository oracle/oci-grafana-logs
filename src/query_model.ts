/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/

import { OCIQuery, QueryPlaceholder } from './types';
import { ScopedVars } from '@grafana/data';
import { TemplateSrv } from '@grafana/runtime';

/**
 * QueryModel is responsible for managing and processing an OCI query.
 *
 * This class initializes query parameters, applies template variable substitution,
 * and provides utility methods to determine if a query is ready to execute.
*/
export default class QueryModel {
  target: OCIQuery;
  templateSrv: any;
  scopedVars: any;
  refId?: string;

  /**
   * Constructs a new QueryModel instance.
   *
   * @param {OCIQuery} incomingQuery - The query object containing user-defined parameters.
   * @param {TemplateSrv} [templateSrv] - Grafana's template service for handling template variables.
   * @param {ScopedVars} [scopedVars] - Scoped variables used for dynamic substitutions.
  */
  constructor(incomingQuery: OCIQuery, templateSrv?: TemplateSrv, scopedVars?: ScopedVars) {
    this.target = incomingQuery;
    this.templateSrv = templateSrv;
    this.scopedVars = scopedVars;

    this.target.tenancy = incomingQuery.tenancy || QueryPlaceholder.Tenancy;
    this.target.region = incomingQuery.region || QueryPlaceholder.Region;
    this.target.searchQuery = incomingQuery.searchQuery || '';

    this.target.hide = incomingQuery.hide ?? false;
    if (this.target.tenancy === QueryPlaceholder.Tenancy) {
        this.target.tenancy = 'DEFAULT/';
    }
  }

  /**
   * Determines if the query is ready to be executed.
   *
   * A query is considered ready if it has valid tenancy and region values.
   *
   * @returns {boolean} True if the query is ready, otherwise false.
  */
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
