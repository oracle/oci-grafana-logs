/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/

import React, {KeyboardEvent, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, FieldSet, SegmentAsync, Input, TextArea } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { getTemplateSrv } from '@grafana/runtime';
import { OCIDataSource } from './datasource';
import { OCIDataSourceOptions, OCIQuery, QueryPlaceholder } from './types';
//import QueryModel from './query_model';
import { TenancyChoices } from './config.options';

type Props = QueryEditorProps<OCIDataSource, OCIQuery, OCIDataSourceOptions>;

/**
 * QueryEditor component.
 * 
 * This component allows users to configure and execute queries by selecting
 * tenancy, region, and entering a search query.
 *
 * Features:
 * - Supports both single-tenancy and multi-tenancy modes.
 * - Fetches available tenancies and subscribed regions dynamically.
 * - Provides an input field for query entry.
 * - Allows execution of queries with keyboard shortcuts (Shift+Enter or Ctrl+Enter).
 *
 * Props:
 * @param {Props} props - The component properties.
 * @param {OCIQuery} props.query - The current query object.
 * @param {OCIDataSource} props.datasource - The OCI data source instance.
 * @param {Function} props.onChange - Callback for updating the query state.
 * @param {Function} props.onRunQuery - Callback for executing the query.
 *
 * Returns:
 * A React component that renders the query editor UI for the OCI data source.
 */

export const QueryEditor: React.FC<Props> = (props) => {
  const { query, datasource, onChange, onRunQuery } = props;
  const tmode = datasource.getJsonData().tenancymode;
  const [hasLegacyTenancy, setHasLegacyTenancy] = useState(false);
  const [tenancyValue, setTenancyValue] = useState(query.tenancyName);
  const [regionValue, setRegionValue] = useState(query.region);
  const [hasCalledGetTenancyDefault, setHasCalledGetTenancyDefault] = useState(false);  
  
  /**
   * Handles keydown events in the query input field.
   * 
   * Triggers the query execution when the user presses "Enter" while holding 
   * the "Shift" or "Ctrl" key. Prevents the default behavior to avoid unintended 
   * newlines in the input field.
   *
   * @param {KeyboardEvent<HTMLTextAreaElement>} event - The keydown event object.
  */
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && (event.shiftKey || event.ctrlKey)) {
      event.preventDefault();
      onRunQuery();
    }
  };

  /**
   * Updates the query state and optionally triggers the query execution.
   *
   * If `runQuery` is `true`, additional logic can be added to process or validate 
   * the query before running it. If `runQuery` is `false`, the function only updates 
   * the query state without executing it.
   *
   * @param {OCIQuery} changedQuery - The modified query object.
   * @param {boolean} [runQuery=true] - Whether to trigger query execution after applying changes.
  */
  const onApplyQueryChange = (changedQuery: OCIQuery, runQuery = true) => {
    if (runQuery) {  
      /* TODO: Add some logic*/      
    } else {
      onChange({ ...changedQuery });
    }
  };
  /**
   * addTemplateVariablesToOptions
   *
   * Appends all available template variables to options used by dropdowns.
   *
   * @param options - The array of SelectableValue options to which template variables will be added.
   * @returns The updated array of SelectableValue options.
  */
  const addTemplateVariablesToOptions = (options: Array<SelectableValue<string>>) => {
    getTemplateSrv()
      .getVariables()
      .forEach((item) => {
        options.push({
          label: `$${item.name}`,
          value: `$${item.name}`,
        });
      });
    return options;
  }

  /**
   * CustomInput Component
   *
   * A custom input field for Single Tenancy Mode, pre-filling the tenancy with "DEFAULT/".
  */
  const CustomInput = ({ ...props }) => {
    const [isReady, setIsReady] = useState(false);
  
    useEffect(() => {
      if (!hasCalledGetTenancyDefault && isReady) {
        const getTenancyDefault = async () => {
          const tname = 'DEFAULT/';
          const tvalue = 'DEFAULT/';
          onApplyQueryChange({ ...query, tenancyName: tname, tenancy: tvalue }, false);
          setHasCalledGetTenancyDefault(true);
        };
        getTenancyDefault();
      }
    }, [isReady]);
  
    useEffect(() => {
      setIsReady(true);
    }, []);
  
    return <Input {...props} />;
  };

  /**
   * getTenancyOptions
   *
   * Fetches the available tenancies from the data source.
   *
   * @returns A promise that resolves to an array of SelectableValue options representing the tenancies.
  */
  const getTenancyOptions = async () => {
    let options: Array<SelectableValue<string>> = [];
    options = addTemplateVariablesToOptions(options)
    const response = await datasource.getTenancies();
    if (response) {
      response.forEach((item: any) => {
        const sv: SelectableValue<string> = {
          label: item.name,
          value: item.ocid,
        };
        options.push(sv);
      });
    }
    return options;
  };

  /**
   * getSubscribedRegionOptions
   *
   * Fetches the subscribed regions for the selected tenancy from the data source.
   *
   * @returns A promise that resolves to an array of SelectableValue options representing the regions.
  */
  const getSubscribedRegionOptions = async () => {
      let options: Array<SelectableValue<string>> = [];
      options = addTemplateVariablesToOptions(options)
      const response = await datasource.getSubscribedRegions(query.tenancy);
      if (response) {
        response.forEach((item: string) => {
          const sv: SelectableValue<string> = {
            label: item,
            value: item,
          };
          options.push(sv);
        });
      }
      return options;
  };
  
  /**
   * onTenancyChange
   *
   * Handles changes to the selected tenancy.
   *
   * @param data - The selected tenancy data.
  */
  const onTenancyChange = async (data: any) => {
    setTenancyValue(data);
    onApplyQueryChange(
      {
        ...query,
        tenancyName: data.label,
        tenancy: data.value,
        region: undefined,
      },
      false
    );
  };

  /**
   * onRegionChange 
   * 
   * Handles the change of the region selection.
   *
   * @param {SelectableValue} data - The selected region data.
  */
  const onRegionChange = (data: SelectableValue) => {
    setRegionValue(data.value);   
    onApplyQueryChange({ ...query, region: data.value}, false);
  };

  // set tenancyName in case dashboard was created with version 4.x
  if (query.tenancy && !hasLegacyTenancy && !query.tenancyName) {
      query.tenancyName = query.tenancy;  
      setTenancyValue(query.tenancy);
      setHasLegacyTenancy(true);
  }

  return (
    <>
      <FieldSet>
        <InlineFieldRow>
          {tmode === TenancyChoices.multitenancy && (
            <>
              <InlineField label="TENANCY" labelWidth={20}>
                <SegmentAsync
                  className="width-42"
                  allowCustomValue={false}
                  required={true}
                  loadOptions={getTenancyOptions}
                  value={tenancyValue}
                  placeholder={QueryPlaceholder.Tenancy}
                  onChange={(data) => {
                    onTenancyChange(data);
                  }}
                />
              </InlineField>
            </>
          )}
          {tmode === TenancyChoices.single && (
            <>
        <InlineField label="TENANCY" labelWidth={20}>
          <CustomInput className="width-14" value={"DEFAULT/"} readOnly />
        </InlineField>
            </>
          )}          
        </InlineFieldRow>
        <InlineFieldRow>
          <InlineField label="REGION" labelWidth={20}>
            <SegmentAsync
              className="width-14"
              allowCustomValue={true}
              required={true}
              loadOptions={getSubscribedRegionOptions}
              value={regionValue}
              placeholder={QueryPlaceholder.Region}
              onChange={(data) => {
                onRegionChange(data);
              }}
            />
          </InlineField>
        </InlineFieldRow>
        <InlineField
              label="Query"
              labelWidth={20}
            >
              <TextArea
                type="text"
                value={query.searchQuery}
                placeholder="Enter a Cloud Logging query"
                cols={128}
                rows={10}
                maxLength={4096}
                onKeyDown={onKeyDown}
                onBlur={onRunQuery}
                onChange={e => onChange({
                  ...query,
                  searchQuery: e.currentTarget.value,
                })}
                />
      </InlineField>
      </FieldSet>
    </>
  );
};

