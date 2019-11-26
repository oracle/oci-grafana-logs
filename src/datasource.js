/*
** Copyright © 2019 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/
import _ from 'lodash'
import { aggregations, dimensionKeysQueryRegex, namespacesQueryRegex, metricsQueryRegex, regionsQueryRegex, compartmentsQueryRegex, dimensionValuesQueryRegex, adsQueryRegex } from './constants'
import retryOrThrow from './util/retry'
import { SELECT_PLACEHOLDERS } from './query_ctrl'

export default class OCIDatasource {
  constructor(instanceSettings, $q, backendSrv, templateSrv, timeSrv) {
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
  }

  /**
   * Each Grafana Data source should contain the following functions: 
   *  - query(options) //used by panels to get data
   *  - testDatasource() //used by data source configuration page to make sure the connection is working
   *  - annotationQuery(options) // used by dashboards to get annotations
   *  - metricFindQuery(options) // used by query editor to get metric suggestions.
   * More information: https://grafana.com/docs/plugins/developing/datasources/
  */

  /** 
   * Required method
   * Used by panels to get data
   */
  query(options) {
    var query = this.buildQueryParameters(options);

    if (query.targets.length <= 0) {
      return this.q.when({ data: [] });
    }

    return this.doRequest(query).then(result => {
      var res = []
      _.forEach(result.data.results, r => {
        _.forEach(r.series, s => {
          res.push({ target: s.name, datapoints: s.points })
        })
        _.forEach(r.tables, t => {
          t.type = 'table'
          t.refId = r.refId
          res.push(t)
        })
      })

      result.data = res;
      return result;
    })
  }

  /**
   * Required method
   * Used by data source configuration page to make sure the connection is working
   */
  testDatasource() {
    return this.doRequest({
      targets: [{
        queryType: 'test',
        region: this.defaultRegion,
        tenancyOCID: this.tenancyOCID,
        compartment: '',
        environment: this.environment,
        datasourceId: this.id
      }],
      range: this.timeSrv.timeRange()
    }).then((response) => {
      if (response.status === 200) {
        return { status: 'success', message: 'Data source is working', title: 'Success' }
      }
    }).catch(() => {
      return { status: 'error', message: 'Data source is not working', title: 'Failure' }
    })
  }

  /** 
   * Required method
   * Used by query editor to get metric suggestions
   */
  metricFindQuery(target) {
    if (typeof (target) === 'string') {
      // used in template editor for creating variables
      return this.templateMetricQuery(target);
    }

    const region = target.region === SELECT_PLACEHOLDERS.REGION ? '' : this.getVariableValue(region);
    const compartment = target.compartment === SELECT_PLACEHOLDERS.COMPARTMENT ? '' : this.getVariableValue(target.compartment);
    const namespace = target.namespace === SELECT_PLACEHOLDERS.NAMESPACE ? '' : this.getVariableValue(target.namespace);

    if (_.isEmpty(compartment) || _.isEmpty(namespace)) {
      return this.q.when([]);
    }

    return this.doRequest({
      targets: [{
        environment: this.environment,
        datasourceId: this.id,
        tenancyOCID: this.tenancyOCID,
        queryType: 'search',
        region: _.isEmpty(region) ? this.defaultRegion : region,
        compartment: compartment,
        namespace: namespace
      }],
      range: this.timeSrv.timeRange()
    }).then((res) => {
      return this.mapToTextValue(res, 'search')
    })
  }

  /** 
   * Build and validate query parameters.
   */
  buildQueryParameters(options) {
    let queries = options.targets
      .filter(t => !t.hide)
      .filter(t => !_.isEmpty(this.getVariableValue(t.compartment, options.scopedVars)) && t.compartment !== SELECT_PLACEHOLDERS.COMPARTMENT)
      .filter(t => !_.isEmpty(this.getVariableValue(t.namespace, options.scopedVars)) && t.namespace !== SELECT_PLACEHOLDERS.NAMESPACE)
      .filter(t => !_.isEmpty(this.getVariableValue(t.metric, options.scopedVars)) && t.metric !== SELECT_PLACEHOLDERS.METRIC || !_.isEmpty(this.getVariableValue(t.target)));

    queries.forEach(t => {
      t.dimensions = (t.dimensions || [])
        .filter(dim => !_.isEmpty(dim.key) && dim.key !== SELECT_PLACEHOLDERS.DIMENSION_KEY)
        .filter(dim => !_.isEmpty(dim.value) && dim.value !== SELECT_PLACEHOLDERS.DIMENSION_VALUE);
    });

    // we support multiselect for dimension values, so we need to parse 1 query into multiple queries
    queries = this.splitMultiValueDimensionsIntoQuieries(queries, options);

    queries = queries.map(t => {
      const region = t.region === SELECT_PLACEHOLDERS.REGION ? '' : this.getVariableValue(t.region, options.scopedVars);
      let query = this.getVariableValue(t.target, options.scopedVars);

      if (_.isEmpty(query)) {
        //construct query
        const dimensions = (t.dimensions || []).reduce((result, dim) => {
          const d = `${this.getVariableValue(dim.key, options.scopedVars)} ${dim.operator} "${this.getVariableValue(dim.value, options.scopedVars)}"`;
          if (result.indexOf(d) < 0) {
            result.push(d);
          }
          return result;
        }, []);
        const dimension = _.isEmpty(dimensions) ? '' : `{${dimensions.join(',')}}`;
        query = `${this.getVariableValue(t.metric, options.scopedVars)}[${t.window}]${dimension}.${t.aggregation}`;
      }

      return {
        environment: this.environment,
        datasourceId: this.id,
        tenancyOCID: this.tenancyOCID,
        queryType: 'query',
        resolution: t.resolution,
        refId: t.refId,
        hide: t.hide,
        type: t.type || 'timeserie',
        region: _.isEmpty(region) ? this.defaultRegion : region,
        compartment: this.getVariableValue(t.compartment, options.scopedVars),
        namespace: this.getVariableValue(t.namespace, options.scopedVars),
        query: query
      }
    });

    options.targets = queries;

    return options;
  }

  /** 
   * Splits queries with multi valued dimensions into several quiries.
   * Example: 
   * "DeliverySucceedEvents[1m]{resourceDisplayName = ["ResouceName_1","ResouceName_1"], eventType = ["Create","Delete"]}.mean()" ->
   *  [
   *    "DeliverySucceedEvents[1m]{resourceDisplayName = "ResouceName_1", eventType = "Create"}.mean()",
   *    "DeliverySucceedEvents[1m]{resourceDisplayName = "ResouceName_2", eventType = "Create"}.mean()",
   *    "DeliverySucceedEvents[1m]{resourceDisplayName = "ResouceName_1", eventType = "Delete"}.mean()",
   *    "DeliverySucceedEvents[1m]{resourceDisplayName = "ResouceName_2", eventType = "Delete"}.mean()",
   *  ]
   */
  splitMultiValueDimensionsIntoQuieries(queries, options) {
    return queries.reduce((data, t) => {

      if (_.isEmpty(t.dimensions) || !_.isEmpty(t.target)) {
        // nothing to split or dimensions won't be used, query is set manually
        return data.concat(t);
      }

      // create a map key : [values] for multiple values
      const multipleValueDims = t.dimensions.reduce((data, dim) => {
        const key = dim.key;
        const value = this.getVariableValue(dim.value, options.scopedVars);
        if (value.startsWith("{") && value.endsWith("}")) {
          const values = value.slice(1, value.length - 1).split(',') || [];
          data[key] = (data[key] || []).concat(values);
        }
        return data;
      }, {});

      if (_.isEmpty(Object.keys(multipleValueDims))) {
        // no multiple values used, only single values
        return data.concat(t);
      }

      const splitDimensions = (dims, multiDims) => {
        let prev = [];
        let next = [];

        const firstDimKey = dims[0].key;
        const firstDimValues = multiDims[firstDimKey] || [dims[0].value];
        for (let v of firstDimValues) {
          const newDim = _.cloneDeep(dims[0]);
          newDim.value = v;
          prev.push([newDim]);
        }

        for (let i = 1; i < dims.length; i++) {
          const values = multiDims[dims[i].key] || [dims[i].value];
          for (let v of values) {
            for (let j = 0; j < prev.length; j++) {
              if (next.length >= 20) {
                // this algorithm of collecting multi valued dimensions is computantionally VERY expensive
                // set the upper limit for quiries number
                return next;
              }
              const newDim = _.cloneDeep(dims[i]);
              newDim.value = v;
              next.push(prev[j].concat(newDim));
            }
          }
          prev = next;
          next = [];
        }

        return prev;
      }

      const newDimsArray = splitDimensions(t.dimensions, multipleValueDims);

      const newQueries = [];
      for (let i = 0; i < newDimsArray.length; i++) {
        const dims = newDimsArray[i];
        const newQuery = _.cloneDeep(t);
        newQuery.dimensions = dims;
        if (i !== 0) {
          newQuery.refId = `${newQuery.refId}${i}`;
        }
        newQueries.push(newQuery);
      }
      return data.concat(newQueries);
    }, []);
  }

  // **************************** Template variable helpers ****************************

  /** 
   * Matches the regex from creating template variables and returns options for the corresponding variable.
   * Example: 
   * template variable with the query "regions()" will be matched with the regionsQueryRegex and list of available regions will be returned.
   */
  templateMetricQuery(varString) {

    let regionQuery = varString.match(regionsQueryRegex)
    if (regionQuery) {
      return this.getRegions().catch(err => { throw new Error('Unable to get regions: ' + err) })
    }


    let compartmentQuery = varString.match(compartmentsQueryRegex)
    if (compartmentQuery) {
      return this.getCompartments().catch(err => { throw new Error('Unable to get compartments: ' + err) })
    }

    let namespaceQuery = varString.match(namespacesQueryRegex)
    if (namespaceQuery) {
      let target = {
        region: this.getVariableValue(namespaceQuery[1]),
        compartment: this.getVariableValue(namespaceQuery[2]).replace(',', '').trim()
      }
      return this.getNamespaces(target).catch(err => { throw new Error('Unable to get namespaces: ' + err) })
    }

    let metricQuery = varString.match(metricsQueryRegex)
    if (metricQuery) {
      let target = {
        region: this.getVariableValue(metricQuery[1].trim()),
        compartment: this.getVariableValue(metricQuery[2].replace(',', '').trim()),
        namespace: this.getVariableValue(metricQuery[3].replace(',', '').trim())
      }
      return this.metricFindQuery(target).catch(err => { throw new Error('Unable to get metrics: ' + err) })
    }

    let dimensionsQuery = varString.match(dimensionKeysQueryRegex)
    if (dimensionsQuery) {
      let target = {
        region: this.getVariableValue(dimensionsQuery[1].trim()),
        compartment: this.getVariableValue(dimensionsQuery[2].replace(',', '').trim()),
        namespace: this.getVariableValue(dimensionsQuery[3].replace(',', '').trim()),
        metric: this.getVariableValue(dimensionsQuery[4].replace(',', '').trim()),
      }
      return this.getDimensionKeys(target).catch(err => { throw new Error('Unable to get dimensions: ' + err) })
    }

    let dimensionOptionsQuery = varString.match(dimensionValuesQueryRegex)
    if (dimensionOptionsQuery) {
      let target = {
        region: this.getVariableValue(dimensionOptionsQuery[1].trim()),
        compartment: this.getVariableValue(dimensionOptionsQuery[2].replace(',', '').trim()),
        namespace: this.getVariableValue(dimensionOptionsQuery[3].replace(',', '').trim()),
        metric: this.getVariableValue(dimensionOptionsQuery[4].replace(',', '').trim())
      }
      const dimensionKey = this.getVariableValue(dimensionOptionsQuery[5].replace(',', '').trim());
      return this.getDimensionValues(target, dimensionKey).catch(err => { throw new Error('Unable to get dimension options: ' + err) })
    }

    throw new Error('Unable to parse templating string');
  }

  getRegions() {
    return this.doRequest({
      targets: [{
        environment: this.environment,
        datasourceId: this.id,
        tenancyOCID: this.tenancyOCID,
        queryType: 'regions'
      }],
      range: this.timeSrv.timeRange()
    }).then((items) => {
      return this.mapToTextValue(items, 'regions')
    });
  }

  getCompartments() {
    return this.doRequest({
      targets: [{
        environment: this.environment,
        datasourceId: this.id,
        tenancyOCID: this.tenancyOCID,
        queryType: 'compartments',
        region: this.defaultRegion // compartments are registered for the all regions, so no difference which region to use here
      }],
      range: this.timeSrv.timeRange()
    }).then((items) => {
      return this.mapToTextValue(items, 'compartments')
    });
  }

  getNamespaces(target) {
    const region = target.region === SELECT_PLACEHOLDERS.REGION ? '' : this.getVariableValue(target.region);
    const compartment = target.compartment === SELECT_PLACEHOLDERS.COMPARTMENT ? '' : this.getVariableValue(target.compartment);
    if (_.isEmpty(compartment)) {
      return this.q.when([]);
    }

    return this.doRequest({
      targets: [{
        environment: this.environment,
        datasourceId: this.id,
        tenancyOCID: this.tenancyOCID,
        queryType: 'namespaces',
        region: _.isEmpty(region) ? this.defaultRegion : region,
        compartment: compartment
      }],
      range: this.timeSrv.timeRange()
    }).then((items) => {
      return this.mapToTextValue(items, 'namespaces')
    });
  }

  async getDimensions(target) {
    const region = target.region === SELECT_PLACEHOLDERS.REGION ? '' : this.getVariableValue(target.region);
    const compartment = target.compartment === SELECT_PLACEHOLDERS.COMPARTMENT ? '' : this.getVariableValue(target.compartment);
    const namespace = target.namespace === SELECT_PLACEHOLDERS.NAMESPACE ? '' : this.getVariableValue(target.namespace);
    
    const metric = target.metric === SELECT_PLACEHOLDERS.METRIC ? '' : this.getVariableValue(target.metric);
    const metrics = metric.startsWith("{") && metric.endsWith("}") ? metric.slice(1, metric.length - 1).split(',') : [metric];

    if (_.isEmpty(compartment) || _.isEmpty(namespace) || _.isEmpty(metrics)) {
      return this.q.when([]);
    }

    const dimensionsMap = {};
    for (let m of metrics) {
      if (dimensionsMap[m] !== undefined) {
        continue;
      }
      dimensionsMap[m] = null;
      await this.doRequest({
        targets: [{
          environment: this.environment,
          datasourceId: this.id,
          tenancyOCID: this.tenancyOCID,
          queryType: 'dimensions',
          region: _.isEmpty(region) ? this.defaultRegion : region,
          compartment: compartment,
          namespace: namespace,
          metric: m
        }],
        range: this.timeSrv.timeRange()
      }).then(result => {
        const items = this.mapToTextValue(result, 'dimensions');
        dimensionsMap[m] = [].concat(items);
      }).finally(() => {
        if (!dimensionsMap[m]) {
          dimensionsMap[m] = [];
        }
      });
    }

    let result = [];
    Object.values(dimensionsMap).forEach(dims => {
      if (_.isEmpty(result)) {
        result = dims;
      } else {
        const newResult = [];
        dims.forEach(dim => {
          if (!!result.find(d => d.value === dim.value) && !newResult.find(d => d.value === dim.value)) {
            newResult.push(dim);
          }
        });
        result = newResult;
      }
    })

    return result;
  }

  getDimensionKeys(target) {
    return this.getDimensions(target).then(dims => {
      const dimCache = dims.reduce((data, item) => {
        const values = item.value.split('=') || [];
        const key = values[0] || item.value;
        const value = values[1];

        if (!data[key]) {
          data[key] = []
        }
        data[key].push(value);
        return data;
      }, {});
      return Object.keys(dimCache);
    }).then(items => {
      return items.map(item => ({ text: item, value: item }))
    });
  }

  getDimensionValues(target, dimKey) {
    return this.getDimensions(target).then(dims => {
      const dimCache = dims.reduce((data, item) => {
        const values = item.value.split('=') || [];
        const key = values[0] || item.value;
        const value = values[1];

        if (!data[key]) {
          data[key] = []
        }
        data[key].push(value);
        return data;
      }, {});
      return dimCache[this.getVariableValue(dimKey)] || [];
    }).then(items => {
      return items.map(item => ({ text: item, value: item }))
    });
  }

  getAggregations() {
    return this.q.when(aggregations);
  }

  /** 
   * Calls grafana backend.
   * Retries 10 times before failure.
   */
  doRequest(options) {
    let _this = this
    return retryOrThrow(() => {
      return _this.backendSrv.datasourceRequest({
        url: '/api/tsdb/query',
        method: 'POST',
        data: {
          from: options.range.from.valueOf().toString(),
          to: options.range.to.valueOf().toString(),
          queries: options.targets
        }
      })
    }, 10)
  }

  /** 
   * Converts data from grafana backend to UI format
   */
  mapToTextValue(result, searchField) {
    if (_.isEmpty(result) || _.isEmpty(searchField)) {
      return [];
    }

    var table = result.data.results[searchField].tables[0];
    if (!table) {
      return [];
    }

    var map = _.map(table.rows, (row, i) => {
      if (row.length > 1) {
        return { text: row[0], value: row[1] }
      } else if (_.isObject(row[0])) {
        return { text: row[0], value: i }
      }
      return { text: row[0], value: row[0] }
    })
    return map;
  }

  // **************************** Template variables helpers ****************************

  /**  
   * Get all template variable descriptors
   */
  getVariableDescriptors(regex, type) {
    let vars = this.templateSrv.variables || [];
    if (regex) {
      vars = vars.filter(item => item.query.match(regex) !== null);
    }
    if (type) {
      vars = vars.filter(item => item.type === type)
    }
    return vars;
  }

  /** 
   * List all variable names optionally filtered by regex or/and type
   * Returns list of names with '$' at the beginning. Example: ['$dimensionKey', '$dimensionValue']
  */
  getVariables(regex, type) {
    const varDescriptors = this.getVariableDescriptors(regex, type) || [];
    return varDescriptors.map(item => `$${item.name}`);
  }

  /** 
   * @param varName valid varName contains '$'. Example: '$dimensionKey'
   * Returns an array with variable values or empty array
  */
  getVariableValue(varName, scopedVars = {}) {
    return this.templateSrv.replace(varName, scopedVars) || varName;
  }

  /** 
   * @param varName valid varName contains '$'. Example: '$dimensionKey'
   * Returns true if variable with the given name is found
  */
  isVariable(varName) {
    const varNames = this.getVariables() || [];
    return !!varNames.find(item => item === varName);
  }
}
