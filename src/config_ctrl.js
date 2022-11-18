/*
** Copyright © 2018, 2020 Oracle and/or its affiliates.
** The Universal Permissive License (UPL), Version 1.0
*/
import { regions, environments, tenancymodes } from './constants'

export class OCIConfigCtrl {
  /** @ngInject */
  constructor ($scope, backendSrv) {
    this.backendSrv = backendSrv
    this.tenancyOCID = this.current.jsonData.tenancyOCID
    this.defaultRegion = this.current.jsonData.defaultRegion
    this.defaultCompartmentOCID = this.current.jsonData.defaultCompartmentOCID
    this.environment = this.current.jsonData.environment
    this.tenancymode = this.current.jsonData.tenancymode
  }

  getRegions () {
    return regions
  }

  getEnvironments () {
    return environments
  }

  getTenancyModes () { 
    return tenancymodes
  }

}

OCIConfigCtrl.templateUrl = 'partials/config.html'
