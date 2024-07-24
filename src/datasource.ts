/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/

import _,{ isString} from 'lodash';
import { DataSourceInstanceSettings, ScopedVars, MetricFindValue } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';
import {
  OCIResourceItem,
  ResponseParser,
  //OCIResourceMetadataItem,
} from './resource.response.parser';
import {
  OCIDataSourceOptions,
  OCIQuery,
  OCIResourceCall,
  QueryPlaceholder,
  regionsQueryRegex,
  tenanciesQueryRegex,
  generalQueryRegex,
  DEFAULT_TENANCY
} from "./types";
//import QueryModel from './query_model';


export class OCIDataSource extends DataSourceWithBackend<OCIQuery, OCIDataSourceOptions> {
  private jsonData: any;

  constructor(instanceSettings: DataSourceInstanceSettings<OCIDataSourceOptions>) {
    super(instanceSettings);
    this.jsonData = instanceSettings.jsonData;
  }

  /**
   * Filters disabled/hidden queries
   *
   * @param {string} query Query
   */
  filterQuery(query: OCIQuery): boolean {
    if (query.hide) {
      return false;
    }
    return true;
  }

  getqueryVarFormatter = (value: string): string => {
    if (typeof value === 'string') {
      return "'"+value+"'";
    } else {
      return value
    }
  };

  /**
   * Override to apply template variables
   *
   * @param {string} query Query
   * @param {ScopedVars} scopedVars Scoped variables
   */
  applyTemplateVariables(query: OCIQuery, scopedVars: ScopedVars) {
    const templateSrv = getTemplateSrv();  
    query.region = templateSrv.replace(query.region, scopedVars);
    query.tenancy = templateSrv.replace(query.tenancy, scopedVars);
    if (query.tenancy) {
      query.tenancy = templateSrv.replace(query.tenancy, scopedVars);
    }
    //const queryModel = new QueryModel(query, getTemplateSrv());
    query.searchQuery = templateSrv.replace(query.searchQuery, scopedVars, this.getqueryVarFormatter);

    console.log("templateSrv.getVariables(): "+this.getVariables());
    console.log("searchQuery: "+query.searchQuery);    
  
    return query;
  }


  interpolateProps<T extends Record<string, any>>(object: T, scopedVars: ScopedVars = {}): T {
    const templateSrv = getTemplateSrv();
    return Object.entries(object).reduce((acc, [key, value]) => {
      return {
        ...acc,
        [key]: value && isString(value) ? templateSrv.replace(value, scopedVars) : value,
      };
    }, {} as T);
  }

  // // **************************** Template variable helpers ****************************

  // /**
  //  * Matches the regex from creating template variables and returns options for the corresponding variable.
  //  * Example:
  //  * template variable with the query "regions()" will be matched with the regionsQueryRegex and list of available regions will be returned.
  //  */
  // metricFindQuery?(query: any, options?: any): Promise<MetricFindValue[]> {

  async metricFindQuery?(query: any, options?: any): Promise<MetricFindValue[]> {
    const templateSrv = getTemplateSrv();
    // const tmode = this.getJsonData().tenancymode;

    const tenancyQuery = query.match(tenanciesQueryRegex);
    if (tenancyQuery) {
      const tenancy = await this.getTenancies();
      return tenancy.map(n => {
        return { text: n.name, value: n.ocid };
      });   
    }    

    const regionQuery = query.match(regionsQueryRegex);
    if (regionQuery) {
      if (this.jsonData.tenancymode === "multitenancy") {
        const tenancy = templateSrv.replace(regionQuery[1]);
        const regions = await this.getSubscribedRegions(tenancy);
        return regions.map(n => {
          return { text: n, value: n };
        });
      } else {
        const regions = await this.getSubscribedRegions(DEFAULT_TENANCY);
        return regions.map(n => {
          return { text: n, value: n };
        });       
      }
    }


    const generalQuery = query.match(generalQueryRegex);
    if (generalQuery) {
      if (this.jsonData.tenancymode === "multitenancy") {
        const tenancy = templateSrv.replace(generalQuery[1]);
        const region = templateSrv.replace(generalQuery[2]);
        const putquery = templateSrv.replace(generalQuery[3]);
        const field = templateSrv.replace(generalQuery[4]);
        const getquery = await this.getQuery(tenancy, region, putquery, field);
        return getquery.map(n => {
          return { text: n, value: n };
        });        
      } else {
        const tenancy = DEFAULT_TENANCY;
        const region = templateSrv.replace(generalQuery[1]);
        const putquery = templateSrv.replace(generalQuery[2]);
        const field = templateSrv.replace(generalQuery[3]);
        const getquery = await this.getQuery(tenancy, region, putquery, field);
        return getquery.map(n => {
          return { text: n, value: n };
        });      
      }
    }

    return [];
  }


  getJsonData() {
    return this.jsonData;
  }
  
  getVariables() {
    const templateSrv = getTemplateSrv();
    return templateSrv.getVariables().map((v) => `$${v.name}`);
  }

  getVariablesRaw() {
    const templateSrv = getTemplateSrv();
    return templateSrv.getVariables();
  }  


 // **************************** Template variables helpers ****************************

  /**
   * List all variable names optionally filtered by regex or/and type
   * Returns list of names with '$' at the beginning. Example: ['$dimensionKey', '$dimensionValue']
   *
   * Updates:
   * Notes on implementation :
   * If a custom or constant is in  variables and  includeCustom, default is false.
   * Hence,the varDescriptors list is filtered for a unique set of var names
   */

  /**
   * @param varName valid varName contains '$'. Example: '$dimensionKey'
   * Returns true if variable with the given name is found
   */
  isVariable(varName: string) {
    const varNames = this.getVariables() || [];
    return !!varNames.find((item) => item === varName);
  }


  // main caller to call resource handler for get call
  async getResource(path: string): Promise<any> {
    return super.getResource(path);
  }
  // main caller to call resource handler for post call
  async postResource(path: string, body: any): Promise<any> {
    return super.postResource(path, body);
  }


  async getTenancies(): Promise<OCIResourceItem[]> {
    return this.getResource(OCIResourceCall.Tenancies).then((response) => {
      return new ResponseParser().parseTenancies(response);
    });
  }

  async getSubscribedRegions(tenancy: string): Promise<string[]> {
    if (this.isVariable(tenancy)) {
      let { tenancy: var_tenancy} = this.interpolateProps({tenancy});
      if (var_tenancy !== "") { 
        tenancy = var_tenancy
      }      
    }
    if (tenancy === '') {
      return [];
    }
    const reqBody: JSON = {
      tenancy: tenancy,
    } as unknown as JSON;
    return this.postResource(OCIResourceCall.Regions, reqBody).then((response) => {
      return new ResponseParser().parseRegions(response);
    });
  }


  async getQuery(
    tenancy: string,
    region: any,
    getquery: any,
    field: any
  ): Promise<string[]>  {
    if (this.isVariable(tenancy)) {
      let { tenancy: var_tenancy} = this.interpolateProps({tenancy});
      if (var_tenancy !== "") { 
        tenancy = var_tenancy
      }      
    }

    if (this.isVariable(getquery)) {
      let { getquery: var_getquery} = this.interpolateProps({getquery});
      if (var_getquery !== "") { 
        getquery = var_getquery
      }      
    }

    if (this.isVariable(field)) {
      let { field: var_field} = this.interpolateProps({field});
      if (var_field !== "") { 
        field = var_field
      }      
    }

    if (this.isVariable(region)) {
      let { region: var_region} = this.interpolateProps({region});
      if (var_region !== "") { 
        region = var_region
      }      
    }

    if (tenancy === '') {
      return [];
    }
    if (region === undefined || region === QueryPlaceholder.Region) {
      return [];
    }

    if (getquery === undefined || getquery === '') {
      getquery = '';
    }

    if (field === undefined || field === '') {
      field = '';
    }

  // Check for special cases or undefined interval
    let timeStart = parseInt(getTemplateSrv().replace("${__from}"), 10);
    let timeEnd = parseInt(getTemplateSrv().replace("${__to}"), 10);

    console.log("timeStart", timeStart);
    console.log("timeEnd", timeEnd);

    const reqBody: JSON = {
      tenancy: tenancy,
      region: region,
      getquery: getquery,
      field: field,
      timeStart: timeStart,
      timeEnd: timeEnd,
    } as unknown as JSON;
    return this.postResource(OCIResourceCall.getQuery, reqBody).then((response) => {
      return new ResponseParser().parseGetQuery(response);
    });
  }



}

