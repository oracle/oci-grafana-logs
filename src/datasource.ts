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

/**
 * The OCIDataSource class extends `DataSourceWithBackend` to integrate OCI as a data source for Grafana, 
 * allowing users to query OCI resources using Grafana's UI and templating system.
 * It supports querying OCI tenancies, subscribed regions, and running custom queries.
 * 
 * @extends DataSourceWithBackend<OCIQuery, OCIDataSourceOptions>
*/
export class OCIDataSource extends DataSourceWithBackend<OCIQuery, OCIDataSourceOptions> {
  private jsonData: any;

  /**
   * Constructor for the OCIDataSource class.
   *
   * @param {DataSourceInstanceSettings<OCIDataSourceOptions>} instanceSettings - The settings for the data source instance.
  */
  constructor(instanceSettings: DataSourceInstanceSettings<OCIDataSourceOptions>) {
    super(instanceSettings);
    this.jsonData = instanceSettings.jsonData;
  }

  /**
   * Filters disabled/hidden queries
   *
   * @param {OCIQuery} query - The query to filter.
   * @returns {boolean} True if the query is not hidden, false otherwise.
   */
  filterQuery(query: OCIQuery): boolean {
    if (query.hide) {
      return false;
    }
    return true;
  }

  /**
   * Formats query variable values by wrapping them in single quotes if they are strings.
   *
   * This function is primarily used to ensure that string-based query parameters 
   * are properly formatted when substituted into OCI queries, preventing syntax errors.
   *
   * @param {string} value - The query variable value to be formatted.
   * @returns {string} - The formatted string, enclosed in single quotes if it is a string.
  */
  getqueryVarFormatter = (value: string): string => {
    if (typeof value === 'string') {
      return "'"+value+"'";
    } else {
      return value
    }
  };

  /**
   * Replaces template variables in the given OCIQuery object with their actual values.
   *
   * This function ensures that any Grafana template variables used in the query are
   * substituted with the correct values before execution. It applies template substitution
   * to the region, tenancy, and search query fields.
   *
   * @param {OCIQuery} query - The query object containing template variables.
   * @param {ScopedVars} scopedVars - The scoped variables that may contain overrides.
   * @returns {OCIQuery} - The updated query object with template variables replaced.
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
  
    return query;
  }

  /**
   * Replaces template variables in an object's string properties with their actual values.
   *
   * This function iterates over all key-value pairs in the provided object and replaces 
   * any string values containing Grafana template variables with their resolved values. 
   * It ensures that only string values are processed, leaving other data types unchanged.
   *
   * @template T - A generic type representing an object with key-value pairs.
   * @param {T} object - The object containing properties that may include template variables.
   * @param {ScopedVars} [scopedVars={}] - The scoped variables that may contain overrides for template values.
   * @returns {T} - A new object with all string properties updated with resolved template values.
  */
  interpolateProps<T extends Record<string, any>>(object: T, scopedVars: ScopedVars = {}): T {
    const templateSrv = getTemplateSrv();
    return Object.entries(object).reduce((acc, [key, value]) => {
      return {
        ...acc,
        [key]: value && isString(value) ? templateSrv.replace(value, scopedVars) : value,
      };
    }, {} as T);
  }

  // **************************** Template variable helpers ****************************

  /**
   * Executes a query for template variable values and returns the results.
   *
   * @param {any} query - The query string or object.
   * @param {any} [options] - Optional query options.
   * @returns {Promise<MetricFindValue[]>} A promise that resolves to an array of MetricFindValue objects.
  */
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

  /**
   * Gets the JSON data associated with this data source.
   *
   * @returns {any} The JSON data.
  */
  getJsonData() {
    return this.jsonData;
  }
  
  /**
   * Gets the list of variable names.
   *
   * @returns {string[]} An array of variable names with '$' at the beginning.
  */
  getVariables() {
    const templateSrv = getTemplateSrv();
    return templateSrv.getVariables().map((v) => `$${v.name}`);
  }

  /**
   * Gets the raw list of variables.
   *
   * @returns {any[]} An array of raw variable objects.
  */
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

  /**
   * Calls the backend to fetch a resource.
   *
   * @param {string} path - The path of the resource to fetch.
   * @returns {Promise<any>} A promise that resolves to the resource data.
  */
  async getResource(path: string): Promise<any> {
    return super.getResource(path);
  }

  /**
   * Calls the backend to post data to a resource.
   *
   * @param {string} path - The path of the resource.
   * @param {any} body - The request body.
   * @returns {Promise<any>} A promise that resolves to the response data.
  */
  async postResource(path: string, body: any): Promise<any> {
    return super.postResource(path, body);
  }

  /**
   * Retrieves a list of tenancies from the OCI (Oracle Cloud Infrastructure).
   *
   * @returns {Promise<OCIResourceItem[]>} A promise that resolves to an array of OCIResourceItem objects representing the tenancies.
  */
  async getTenancies(): Promise<OCIResourceItem[]> {
    return this.getResource(OCIResourceCall.Tenancies).then((response) => {
      return new ResponseParser().parseTenancies(response);
    });
  }

  /**
   * Retrieves the list of subscribed regions for a given tenancy.
   *
   * @param tenancy - The tenancy identifier. If the tenancy is a variable, it will be interpolated.
   * @returns A promise that resolves to an array of subscribed region names.
   *
   * @throws Will return an empty array if the tenancy is an empty string.
  */
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

  /**
   * Executes a query against OCI resources, resolving template variables where necessary.
   *
   * This function takes a query request containing tenancy, region, and filtering options.
   * It first resolves any template variables in the provided parameters, ensuring that 
   * dynamic values are correctly substituted before making the API request. The query 
   * then executes an HTTP request to fetch the data and returns the parsed response.
   *
   * @param {string} tenancy - The tenancy OCID or variable representing the tenancy.
   * @param {any} region - The OCI region where the query is executed.
   * @param {any} getquery - The specific query string to be executed.
   * @param {any} field - The field or resource type to filter the query results.
   * @returns {Promise<string[]>} - A promise resolving to an array of query results.
  */
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
