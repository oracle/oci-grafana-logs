/*
 ** Copyright © 2018, 2022 Oracle and/or its affiliates.
 ** The Universal Permissive License (UPL), Version 1.0
 */
import { QueryCtrl } from "app/plugins/sdk";
import "./css/query-editor.css!";
import {
  regionsQueryRegex,
  tenanciesQueryRegex,
  compartmentsQueryRegex,
} from "./constants";

export const SELECT_PLACEHOLDERS = {
  COMPARTMENT: "select compartment",
  TENANCY: 'select tenancy config',
  REGION: "select region",
};

export class OCIDatasourceQueryCtrl extends QueryCtrl {
  constructor($scope, $injector, $q, uiSegmentSrv) {
    super($scope, $injector);

    this.q = $q;
    this.uiSegmentSrv = uiSegmentSrv;

    this.target.region = this.target.region || SELECT_PLACEHOLDERS.REGION;
    this.target.compartment = this.target.compartment || SELECT_PLACEHOLDERS.COMPARTMENT;
    this.target.tenancy = this.target.tenancy || SELECT_PLACEHOLDERS.TENANCY;
    this.target.searchQuery = this.target.searchQuery || "";
    this.target.tenancymode = this.datasource.tenancymode || ''

    if (this.datasource.tenancymode === "multitenancy") {
      this.target.MultiTenancy = true;
    }    
  }

  // ****************************** Options **********************************

  getTenancies() {
    return this.datasource.getTenancies().then(tenancies => {
      return this.appendVariables([ ...tenancies], tenanciesQueryRegex);
    });
  }

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

  appendVariables(options, varQueryRegex) {
    const vars = this.datasource.getVariables(varQueryRegex) || [];
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
