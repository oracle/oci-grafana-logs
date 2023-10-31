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


export const QueryEditor: React.FC<Props> = (props) => {
  const { query, datasource, onChange, onRunQuery } = props;
  const tmode = datasource.getJsonData().tenancymode;
  const [hasLegacyTenancy, setHasLegacyTenancy] = useState(false);
  const [tenancyValue, setTenancyValue] = useState(query.tenancyName);
  const [regionValue, setRegionValue] = useState(query.region);
  const [hasCalledGetTenancyDefault, setHasCalledGetTenancyDefault] = useState(false);  
  
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && (event.shiftKey || event.ctrlKey)) {
      event.preventDefault();
      onRunQuery();
    }
  };

  const onApplyQueryChange = (changedQuery: OCIQuery, runQuery = true) => {
    if (runQuery) {  
      /* TODO: Add some logic*/      
    } else {
      onChange({ ...changedQuery });
    }
  };
  // Appends all available template variables to options used by dropdowns
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

  // Custom input field for Single Tenancy Mode
  const CustomInput = ({ ...props }) => {
    useEffect(() => {    
      if (!hasCalledGetTenancyDefault) {
        getTenancyDefault();
        setHasCalledGetTenancyDefault(true);
      }
    }, []);
    return <Input {...props} />;
  };

  // fetch the tenancies, with name as key and ocid as value
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
  
  const getTenancyDefault = async () => {
    const tname = 'DEFAULT/';
    const tvalue = 'DEFAULT/';
    onApplyQueryChange({ ...query, tenancyName: tname, tenancy: tvalue }, false);
  };

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

