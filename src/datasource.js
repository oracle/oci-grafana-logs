/*
 ** Copyright © 2018, 2022 Oracle and/or its affiliates.
 ** The Universal Permissive License (UPL), Version 1.0
 */
import _ from 'lodash'
import * as graf from '@grafana/data'
import {
  compartmentsQueryRegex,
  regionsQueryRegex
} from './constants'
import retryOrThrow from './util/retry'
import { SELECT_PLACEHOLDERS } from './query_ctrl'
import { toDataQueryResponse } from '@grafana/runtime'; // This import is required to transform each of the response so that it can be mapped to the query sent in the request. When migrating to grafana 8 with react , this is not required as it will be handled by the constructor itself

const DEFAULT_RESOURCE_GROUP = 'NoResourceGroup'

export default class OCIDatasource {
  constructor (instanceSettings, $q, backendSrv, templateSrv, timeSrv) {
    this.type = instanceSettings.type
    this.url = instanceSettings.url
    this.name = instanceSettings.name
    this.id = instanceSettings.id
    this.tenancyOCID = instanceSettings.jsonData.tenancyOCID
    this.defaultRegion = instanceSettings.jsonData.defaultRegion
    this.environment = instanceSettings.jsonData.environment
    this.q = $q
    this.backendSrv = backendSrv
    this.templateSrv = templateSrv
    this.timeSrv = timeSrv

    this.compartmentsCache = []
    this.regionsCache = []

    // this.getRegions()
    // this.getCompartments()
  }

  /**
   * Each Grafana Data source should contain the following functions:
   *  - query(request) //used by panels to get data
   *  - testDatasource() //used by data source configuration page to make sure the connection is working
   *  - annotationQuery(options) // used by dashboards to get annotations
   *  - metricFindQuery(options) // used by query editor to get metric suggestions.
   * More information: https://grafana.com/docs/plugins/developing/datasources/
   */

  /**
   * Required method
   * Used by panels to get data
   */

  async query (request) {
    var query = await this.buildQueryParameters(request)
    const {targets} = query
    if (targets.length <= 0 || !targets[0].searchQuery) {
      return this.q.when({ data: [] })
    } 

    /*
     * Keep the logic for creating the data frames within the backend logic
     */
    return this.doRequest(query)

   }

  /**
   * Required method
   * Used by data source configuration page to make sure the connection is working
   */
  testDatasource () {
    return this.doRequest({
      targets: [
        {
          queryType: 'test',
          region: this.defaultRegion,
          tenancyOCID: this.tenancyOCID,
          environment: this.environment,
          datasourceId: this.id
        }
      ],
      range: this.timeSrv.timeRange()
    })
      .then((response) => {
        if (response.status === 200) {
          return {
            status: 'success',
            message: 'Data source is working',
            title: 'Success'
          }
        }
      })
      .catch(() => {
        return {
          status: 'error',
          message: 'Data source is not working',
          title: 'Failure'
        }
      })
  }

  /**
   * Required method
   * Used by query editor to get metric suggestions
   */
  async metricFindQuery (target) {
    if (typeof target === 'string') {
      // used in template editor for creating variables
      return this.templateMetricQuery(target)
    }
    const region =
      target.region === SELECT_PLACEHOLDERS.REGION
        ? ''
        : this.getVariableValue(target.region)
    const compartment =
      target.compartment === SELECT_PLACEHOLDERS.COMPARTMENT
        ? ''
        : this.getVariableValue(target.compartment)

    const compartmentId = await this.getCompartmentId(compartment)
    return this.doRequest({
      targets: [
        {
          environment: this.environment,
          datasourceId: this.id,
          tenancyOCID: this.tenancyOCID,
          queryType: 'search',
          region: _.isEmpty(region) ? this.defaultRegion : region,
          compartment: compartmentId,
          namespace: namespace,
          resourcegroup: resourcegroup
        }
      ],
      range: this.timeSrv.timeRange()
    }).then((res) => {
      return this.mapToTextValue(res, 'search')
    })
  }

  /**
   * Build and validate query parameters.
   */
  async buildQueryParameters (request) {
    let queries = request.targets
      .filter((t) => !t.hide)

    const results = []
    // When a user is in the Explore window the panel ID value is a string but when the user is on
    // a dashboard the panel ID value is numeric so convert all panel IDs to be strings so that the
    // plugin backend gets a consistently formatted panel ID
    let panelIdStr = ''
    const parsed = parseInt(request.panelId, 10);
    if (isNaN(parsed)) {
      panelIdStr = request.panelId
    } else {
      panelIdStr = request.panelId.toString()
    }

    for (let t of queries) {
      const region =
        t.region === SELECT_PLACEHOLDERS.REGION
          ? ''
          : this.getVariableValue(t.region, request.scopedVars)
      let searchQuery = this.getVariableValue(t.searchQuery, request.scopedVars)

      const result = {
        environment: this.environment,
        datasourceId: this.id,
        queryType: 'searchLogs',
        refId: t.refId,
        hide: t.hide,
        type: t.type || 'timeserie',
        searchQuery: searchQuery,
        region: _.isEmpty(region) ? this.defaultRegion : region,
        maxDataPoints: request.maxDataPoints,
        panelId: panelIdStr
      }
      results.push(result)
    }

    request.targets = results

    return request
  }

  // **************************** Template variable helpers ****************************

  /**
   * Matches the regex from creating template variables and returns options for the corresponding variable.
   * Example:
   * template variable with the query "regions()" will be matched with the regionsQueryRegex and list of available regions will be returned.
   */
  templateMetricQuery (varString) {
    console.log('* getting suggestions ')
    let regionQuery = varString.match(regionsQueryRegex)
    if (regionQuery) {
      return this.getRegions().catch((err) => {
        throw new Error('Unable to get regions: ' + err)
      })
    }

    let compartmentQuery = varString.match(compartmentsQueryRegex)
    if (compartmentQuery) {
      return this.getCompartments()
        .then((compartments) => {
          return compartments.map((c) => ({ text: c.text, value: c.text }))
        })
        .catch((err) => {
          throw new Error('Unable to get compartments: ' + err)
        })
    }
     throw new Error('Unable to parse templating string')
  }

  getRegions () {
    if (this.regionsCache && this.regionsCache.length > 0) {
      return this.q.when(this.regionsCache)
    }

    return this.doRequest({
      targets: [
        {
          environment: this.environment,
          datasourceId: this.id,
          tenancyOCID: this.tenancyOCID,
          queryType: 'regions'
        }
      ],
      range: this.timeSrv.timeRange()
    }).then((items) => {
      this.regionsCache = this.mapToTextValue(items, 'regions')
      return this.regionsCache
    })
  }

  getCompartments () {
    if (this.compartmentsCache && this.compartmentsCache.length > 0) {
      return this.q.when(this.compartmentsCache)
    }

    return this.doRequest({
      targets: [
        {
          environment: this.environment,
          datasourceId: this.id,
          tenancyOCID: this.tenancyOCID,
          queryType: 'compartments',
          region: this.defaultRegion // compartments are registered for the all regions, so no difference which region to use here
        }
      ],
      range: this.timeSrv.timeRange()
    }).then((items) => {
      this.compartmentsCache = this.mapToTextValue(items, 'compartments')
      return this.compartmentsCache
    })
  }

  getCompartmentId (compartment) {
    return this.getCompartments().then((compartments) => {
      const compartmentFound = compartments.find(
        (c) => c.text === compartment || c.value === compartment
      )
      return compartmentFound ? compartmentFound.value : compartment
    })
  }

  /**
   * Calls grafana backend.
   * Retries 10 times before failure.
   */
  doRequest (request) {
    let _this = this
    return retryOrThrow(() => {
      return _this.backendSrv.datasourceRequest({
        url: '/api/ds/query',
        method: 'POST',
        data: {
          from: request.range.from.valueOf().toString(),
          to: request.range.to.valueOf().toString(),
          queries: request.targets
        }
      })
    }, 10).then((res) => toDataQueryResponse(res, request));
  }

  /**
   * Converts data from grafana backend to UI format
   */
  mapToTextValue (result, searchField) {
    if (_.isEmpty(result)) return [];

    // All drop-downs send a request to the backend and based on the query type, the backend sends a response
    // Depending on the data available , options are shaped
    // Values in fields are of type vectors (Based on the info from Grafana)

    switch (searchField) {
      case "compartments":
        return result.data[0].fields[0].values.toArray().map((name, i) => ({
          text: name,
          value: result.data[0].fields[1].values.toArray()[i],
        }));
      case "regions":
      case "search":
        return result.data[0].fields[0].values.toArray().map((name) => ({
          text: name,
          value: name,
        }));
      // remaining  cases will be completed once the fix works for the above two
      default:
        return {};
    }
  }
  // **************************** Template variables helpers ****************************

  /**
   * Get all template variable descriptors
   */
  getVariableDescriptors (regex, includeCustom = true) {
    const vars = this.templateSrv.variables || []

    if (regex) {
      let regexVars = vars.filter((item) => item.query.match(regex) !== null)
      if (includeCustom) {
        const custom = vars.filter(
          (item) => item.type === 'custom' || item.type === 'constant'
        )
        regexVars = regexVars.concat(custom)
      }
      const uniqueRegexVarsMap = new Map()
      regexVars.forEach((varObj) =>
        uniqueRegexVarsMap.set(varObj.name, varObj)
      )
      return Array.from(uniqueRegexVarsMap.values())
    }
    return vars
  }

  /**
   * List all variable names optionally filtered by regex or/and type
   * Returns list of names with '$' at the beginning. Example: ['$dimensionKey', '$dimensionValue']
   *
   * Updates:
   * Notes on implementation :
   * If a custom or constant is in  variables and  includeCustom, default is false.
   * Hence,the varDescriptors list is filtered for a unique set of var names
   */
  getVariables (regex, includeCustom) {
    const varDescriptors =
      this.getVariableDescriptors(regex, includeCustom) || []
    return varDescriptors.map((item) => `$${item.name}`)
  }

  /**
   * @param varName valid varName contains '$'. Example: '$dimensionKey'
   * Returns an array with variable values or empty array
   */
  getVariableValue (varName, scopedVars = {}) {
    return this.templateSrv.replace(varName, scopedVars) || varName
  }

  /**
   * @param varName valid varName contains '$'. Example: '$dimensionKey'
   * Returns true if variable with the given name is found
   */
  isVariable (varName) {
    const varNames = this.getVariables() || []
    return !!varNames.find((item) => item === varName)
  }
}
