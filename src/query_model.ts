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
    //this.target.compartment = incomingQuery.compartment || '';
    this.target.region = incomingQuery.region || QueryPlaceholder.Region;
    //this.target.namespace = incomingQuery.namespace || QueryPlaceholder.Namespace;
    //this.target.metric = incomingQuery.metric || QueryPlaceholder.Metric;
    /*this.target.statistic = incomingQuery.statistic || QueryPlaceholder.Aggregation;
    this.target.interval = incomingQuery.interval || QueryPlaceholder.Interval;
    this.target.resourcegroup = incomingQuery.resourcegroup || QueryPlaceholder.ResourceGroup;
    this.target.dimensionValues = incomingQuery.dimensionValues || [];
    this.target.tagsValues = incomingQuery.tagsValues || [];
    this.target.groupBy = incomingQuery.groupBy || QueryPlaceholder.GroupBy;*/

    this.target.hide = incomingQuery.hide ?? false;

    /*if (this.target.resourcegroup === QueryPlaceholder.ResourceGroup) {
      this.target.resourcegroup = '';
    }*/

    if (this.target.tenancy === QueryPlaceholder.Tenancy) {
        this.target.tenancy = 'DEFAULT/';
    }   

    // handle pre query gui panels gracefully, so by default we will have raw editor
    /*this.target.rawQuery = incomingQuery.rawQuery ?? true;

    if (this.target.rawQuery) {
      this.target.searchQuery =
        incomingQuery.searchQuery || 'metric[interval]{dimensionname="dimensionvalue"}.groupingfunction.statistic';
    } else {
      this.target.searchQuery = incomingQuery.searchQuery || this.buildQuery(String(this.target.metric));
    }*/
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
  

  buildQuery(searchQuery: string) {
    // let searchQuery = this.target.metric;     

    /*if (this.target.interval === QueryPlaceholder.Interval) {
      this.target.interval = IntervalOptions[0].value;
    }   
    // for default interval
    if (this.target.interval === QueryPlaceholder.Interval) {
      this.target.interval = IntervalOptions[0].value;
    }
    searchQuery += this.target.interval;*/

    // for dimensions
    

    console.log(searchQuery)

    return searchQuery;
  }
}
