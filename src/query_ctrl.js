/*
 ** Copyright © 2018, 2022 Oracle and/or its affiliates.
 ** The Universal Permissive License (UPL), Version 1.0
 */
import { QueryCtrl } from "app/plugins/sdk";
import "./css/query-editor.css!";
import {
  regionsQueryRegex,
  tenancyconfigQueryRegex,
  compartmentsQueryRegex,
} from "./constants";

export const SELECT_PLACEHOLDERS = {
  COMPARTMENT: "select compartment",
  TENANCYCONFIG: 'select tenancy config',
  REGION: "select region",
};

export class OCIDatasourceQueryCtrl extends QueryCtrl {
  constructor($scope, $injector, $q, uiSegmentSrv) {
    super($scope, $injector);

    this.q = $q;
    this.uiSegmentSrv = uiSegmentSrv;

    this.target.region = this.target.region || SELECT_PLACEHOLDERS.REGION;
    this.target.compartment = this.target.compartment || SELECT_PLACEHOLDERS.COMPARTMENT;
    this.target.tenancyconfig = this.target.tenancyconfig || SELECT_PLACEHOLDERS.TENANCYCONFIG;
    this.target.searchQuery = this.target.searchQuery || "";

    if (this.datasource.environment === "multitenancy") {
      this.target.MultiTenancy = true;
    }    
  }

  // ****************************** Options **********************************

  getRegions() {
    return this.datasource.getRegions(this.target).then((regions) => {
      return this.appendVariables([...regions], regionsQueryRegex);
    });
  }

  getCompartments() {
    return this.datasource.getCompartments(this.target).then((compartments) => {
      return this.appendVariables([...compartments], compartmentsQueryRegex);
    });
  }

  getTenancyConfig() {
    return this.datasource.getTenancyConfig().then(tenancyconfig => {
      return this.appendVariables([ ...tenancyconfig], tenancyconfigQueryRegex);
    });
  }

  appendVariables(options, varQeueryRegex) {
    const vars = this.datasource.getVariables(varQeueryRegex) || [];
    vars.forEach((value) => {
      options.unshift({ value, text: value });
    });
    return options;
  }

  // ****************************** Callbacks **********************************

  toggleEditorMode() {
    this.target.rawQuery = !this.target.rawQuery;
  }

  onChangeInternal() {
    this.panelCtrl.refresh(); // Asks the panel to refresh data.
  }

}

OCIDatasourceQueryCtrl.templateUrl = "partials/query.editor.html";
