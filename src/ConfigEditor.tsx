/*
** Copyright © 2023 Oracle and/or its affiliates. All rights reserved.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
*/

import React, { PureComponent } from 'react';
import { Input, Select, InlineField, FieldSet, InlineSwitch, TextArea } from '@grafana/ui';
import {
  DataSourcePluginOptionsEditorProps,
  onUpdateDatasourceJsonDataOptionSelect,
  onUpdateDatasourceJsonDataOption,
  onUpdateDatasourceJsonDataOptionChecked,
  onUpdateDatasourceSecureJsonDataOption,
} from '@grafana/data';
import { OCIDataSourceOptions } from './types';
import {
  AuthProviders,
  TenancyChoices,
  AuthProviderOptions,
  TenancyChoiceOptions,
} from './config.options';
import {
  regions,
} from './regionlist';

interface Props extends DataSourcePluginOptionsEditorProps<OCIDataSourceOptions> {}

interface State {}

export class ConfigEditor extends PureComponent<Props, State> {
  componentDidMount() {
    const { options, onOptionsChange } = this.props;
    const { jsonData } = options;

    if (!jsonData.profile0) {
      onOptionsChange({
        ...options,
        jsonData: {
          ...jsonData,
          profile0: "DEFAULT",
        },
      });
    }
  }
  render() {
    const { options } = this.props;

    return (
      <FieldSet label="Connection Details">
        <InlineField
          label="Authentication Provider"
          labelWidth={28}
          tooltip="Specify which OCI credentials chain to use"
        >
          <Select
            className="width-30"
            value={options.jsonData.environment || ''}
            options={AuthProviderOptions}
            defaultValue={options.jsonData.environment}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(this.props, 'environment')(option);
            }}
          />
        </InlineField>
        {options.jsonData.environment === AuthProviders.OCI_INSTANCE  && (
              <>
      <InlineField
          label="Cross Tenancy ocid (optional)"
          labelWidth={28}
          tooltip="AssumeRole compliant Cross Tenancy configuration. Do not use if you are not using Cross Tenancy configuration"
        >
        <Input
          className="width-30"
          value={options.jsonData.xtenancy0}
          onChange={onUpdateDatasourceJsonDataOption(this.props, 'xtenancy0')}
        />
      </InlineField>
            </>
        )}

      {options.jsonData.environment === AuthProviders.OCI_USER  && (
              <>
        <InlineField              
              label="Tenancy Mode"
              labelWidth={28}
              tooltip="Choose if want to enable multi-tenancy mode to fetch metrics accross multiple OCI tenancies"
            >
              <Select
                className="width-30"
                value={options.jsonData.tenancymode || ''}
                options={TenancyChoiceOptions}
                defaultValue={options.jsonData.tenancymode}
                onChange={(option) => {
                  onUpdateDatasourceJsonDataOptionSelect(this.props, 'tenancymode')(option);
                }}
              />
            </InlineField>
            </>
        )}   
            <br></br>


{/* User Principals - Default tenancy */}
  {options.jsonData.environment === AuthProviders.OCI_USER && (
              <>
      <FieldSet label="DEFAULT Connection Details">

      <InlineField
          label="Config Profile Name"
          labelWidth={28}
          tooltip="Config profile name. Default value is DEFAULT."
        >
        <Input
          className="width-30"
          readOnly
          value={options.jsonData.profile0}
        />
      </InlineField>
      <InlineField
          label="Region"
          labelWidth={28}
          tooltip="Specify the Region"
        >
          <Select
            className="width-30"
            value={options.jsonData.region0 || ''}
            options={regions.map((region) => ({
              label: region,
              value: region,
              }))}
            defaultValue={options.jsonData.region0}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(this.props, 'region0')(option);
            }}
          />
        </InlineField>
        <InlineField
              label="User OCID"
              labelWidth={28}
              tooltip="User OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.user0 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'user0')}
                />
      </InlineField>
      <InlineField
              label="Tenancy OCID"
              labelWidth={28}
              tooltip="Tenancy OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.tenancy0 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'tenancy0')}
                />
      </InlineField>
      <InlineField
              label="Fingerprint"
              labelWidth={28}
              tooltip="Fingerprint"
            >
              <Input
                placeholder={options.secureJsonFields.fingerprint0 ? 'configured' : ''}
                className="width-30"
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'fingerprint0')}
                />
      </InlineField>
      <InlineField
              label="Private Key"
              labelWidth={28}
              tooltip="Private Key"
            >
              <TextArea
                type="text"
                className="width-30"
                placeholder={options.secureJsonFields.privkey0 ? 'configured' : ''}
                cols={20}
                rows={4}
                maxLength={4096}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'privkey0')}
                />
      </InlineField>

      </FieldSet>
      </>
      )}  


{/* User Principals - Multitenancy Tenancy 1*/}
        {options.jsonData.tenancymode === TenancyChoices.multitenancy && options.jsonData.environment === AuthProviders.OCI_USER && (
          <>                          
      <FieldSet label="Tenancy-1 Connection Details">
      <InlineField
              label="Config Profile Name"
              labelWidth={28}
              tooltip="Config profile name. Default value is DEFAULT."
            >
              <Input
                className="width-30"
                defaultValue={options.jsonData.profile1}
                onChange={onUpdateDatasourceJsonDataOption(this.props, 'profile1')}
              />
      </InlineField>
      <InlineField
          label="Region"
          labelWidth={28}
          tooltip="Specify the Region"
        >
          <Select
            className="width-30"
            value={options.jsonData.region1 || ''}
            options={regions.map((region) => ({
              label: region,
              value: region,
              }))}
            defaultValue={options.jsonData.region1}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(this.props, 'region1')(option);
            }}
          />
        </InlineField>
        <InlineField
              label="User OCID"
              labelWidth={28}
              tooltip="User OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.user1 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'user1')}
                />
      </InlineField>
      <InlineField
              label="Tenancy OCID"
              labelWidth={28}
              tooltip="Tenancy OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.tenancy1 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'tenancy1')}
                />
      </InlineField>
      <InlineField
              label="Fingerprint"
              labelWidth={28}
              tooltip="Fingerprint"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.fingerprint1 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'fingerprint1')}
                />
      </InlineField>
      <InlineField
              label="Private Key"
              labelWidth={28}
              tooltip="Private Key"
            >
              <TextArea
                type="text"
                className="width-30"
                placeholder={options.secureJsonFields.privkey1 ? 'configured' : ''}
                cols={20}
                rows={4}
                maxLength={4096}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'privkey1')}
                />
      </InlineField>
      <InlineField
          label="Add another Tenancy ?"
          labelWidth={28}
          tooltip="Add Another tenancy YES/NO"
        >
          <InlineSwitch
            className="width-30"
            defaultChecked={options.jsonData && options.jsonData.addon1 ? options.jsonData.addon1 : false}
            onChange={onUpdateDatasourceJsonDataOptionChecked(this.props, 'addon1')}
          />
        </InlineField>
      </FieldSet>
        </>
        )}

{/* User Principals - Multitenancy Tenancy 2*/}
        {options.jsonData.addon1 === true && options.jsonData.environment === AuthProviders.OCI_USER && (
          <>
      <FieldSet label="Tenancy-2 Connection Details">
      <InlineField
              label="Config Profile Name"
              labelWidth={28}
              tooltip="Config profile name."
            >
              <Input
                className="width-30"
                defaultValue={options.jsonData.profile2}
                onChange={onUpdateDatasourceJsonDataOption(this.props, 'profile2')}
              />
      </InlineField>
      <InlineField
          label="Region"
          labelWidth={28}
          tooltip="Specify the Region"
        >
          <Select
            className="width-30"
            value={options.jsonData.region2 || ''}
            options={regions.map((region) => ({
              label: region,
              value: region,
              }))}
            defaultValue={options.jsonData.region2}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(this.props, 'region2')(option);
            }}
          />
        </InlineField>
        <InlineField
              label="User OCID"
              labelWidth={28}
              tooltip="User OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.user2 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'user2')}
                />
      </InlineField>
      <InlineField
              label="Tenancy OCID"
              labelWidth={28}
              tooltip="Tenancy OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.tenancy2 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'tenancy2')}
                />
      </InlineField>
      <InlineField
              label="Fingerprint"
              labelWidth={28}
              tooltip="Fingerprint"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.fingerprint2 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'fingerprint2')}
                />
      </InlineField>
      <InlineField
              label="Private Key"
              labelWidth={28}
              tooltip="Private Key"
            >
              <TextArea
                type="text"
                className="width-30"
                placeholder={options.secureJsonFields.privkey2 ? 'configured' : ''}
                cols={20}
                rows={4}
                maxLength={4096}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'privkey2')}
                />
      </InlineField>
      <InlineField
          label="Add another Tenancy ?"
          labelWidth={28}
          tooltip="Add Another tenancy YES/NO"
        >
          <InlineSwitch
            className="width-30"
            defaultChecked={options.jsonData && options.jsonData.addon2 ? options.jsonData.addon2 : false}
            onChange={onUpdateDatasourceJsonDataOptionChecked(this.props, 'addon2')}
          />
        </InlineField>
      </FieldSet>
          </>
        )}

{/* User Principals - Multitenancy Tenancy 3*/}
{options.jsonData.addon2 === true && options.jsonData.environment === AuthProviders.OCI_USER && (
          <>
      <FieldSet label="Tenancy-3 Connection Details">
      <InlineField
              label="Config Profile Name"
              labelWidth={28}
              tooltip="Config profile name."
            >
              <Input
                className="width-30"
                defaultValue={options.jsonData.profile3}
                onChange={onUpdateDatasourceJsonDataOption(this.props, 'profile3')}
              />
      </InlineField>
      <InlineField
          label="Region"
          labelWidth={28}
          tooltip="Specify the Region"
        >
          <Select
            className="width-30"
            value={options.jsonData.region3 || ''}
            options={regions.map((region) => ({
              label: region,
              value: region,
              }))}
            defaultValue={options.jsonData.region3}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(this.props, 'region3')(option);
            }}
          />
        </InlineField>
        <InlineField
              label="User OCID"
              labelWidth={28}
              tooltip="User OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.user3 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'user3')}
                />
      </InlineField>
      <InlineField
              label="Tenancy OCID"
              labelWidth={28}
              tooltip="Tenancy OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.tenancy3 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'tenancy3')}
                />
      </InlineField>
      <InlineField
              label="Fingerprint"
              labelWidth={28}
              tooltip="Fingerprint"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.fingerprint3 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'fingerprint3')}
                />
      </InlineField>
      <InlineField
              label="Private Key"
              labelWidth={28}
              tooltip="Private Key"
            >
              <TextArea
                type="text"
                className="width-30"
                placeholder={options.secureJsonFields.privkey3 ? 'configured' : ''}
                cols={20}
                rows={4}
                maxLength={4096}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'privkey3')}
                />
      </InlineField>
      <InlineField
          label="Add another Tenancy ?"
          labelWidth={28}
          tooltip="Add Another tenancy YES/NO"
        >
          <InlineSwitch
            className="width-30"
            defaultChecked={options.jsonData && options.jsonData.addon3 ? options.jsonData.addon3 : false}
            onChange={onUpdateDatasourceJsonDataOptionChecked(this.props, 'addon3')}
          />
        </InlineField>
      </FieldSet>
          </>
        )}

{/* User Principals - Multitenancy Tenancy 4*/}
{options.jsonData.addon3 === true && options.jsonData.environment === AuthProviders.OCI_USER && (
          <>
      <FieldSet label="Tenancy-4 Connection Details">
      <InlineField
              label="Config Profile Name"
              labelWidth={28}
              tooltip="Config profile name."
            >
              <Input
                className="width-30"
                defaultValue={options.jsonData.profile4}
                onChange={onUpdateDatasourceJsonDataOption(this.props, 'profile4')}
              />
      </InlineField>
      <InlineField
          label="Region"
          labelWidth={28}
          tooltip="Specify the Region"
        >
          <Select
            className="width-30"
            value={options.jsonData.region4 || ''}
            options={regions.map((region) => ({
              label: region,
              value: region,
              }))}
            defaultValue={options.jsonData.region4}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(this.props, 'region4')(option);
            }}
          />
        </InlineField>
        <InlineField
              label="User OCID"
              labelWidth={28}
              tooltip="User OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.user4 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'user4')}
                />
      </InlineField>
      <InlineField
              label="Tenancy OCID"
              labelWidth={28}
              tooltip="Tenancy OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.tenancy4 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'tenancy4')}
                />
      </InlineField>
      <InlineField
              label="Fingerprint"
              labelWidth={28}
              tooltip="Fingerprint"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.fingerprint4 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'fingerprint4')}
                />
      </InlineField>
      <InlineField
              label="Private Key"
              labelWidth={28}
              tooltip="Private Key"
            >
              <TextArea
                type="text"
                className="width-30"
                placeholder={options.secureJsonFields.privkey4 ? 'configured' : ''}
                cols={20}
                rows={4}
                maxLength={4096}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'privkey4')}
                />
      </InlineField>
      <InlineField
          label="Add another Tenancy ?"
          labelWidth={28}
          tooltip="Add Another tenancy YES/NO"
        >
          <InlineSwitch
            className="width-30"
            defaultChecked={options.jsonData && options.jsonData.addon4 ? options.jsonData.addon4 : false}
            onChange={onUpdateDatasourceJsonDataOptionChecked(this.props, 'addon4')}
          />
        </InlineField>
      </FieldSet>
          </>
        )}

{/* User Principals - Multitenancy Tenancy 5*/}
{options.jsonData.addon4 === true && options.jsonData.environment === AuthProviders.OCI_USER && (
          <>
      <FieldSet label="Tenancy-5 Connection Details">
      <InlineField
              label="Config Profile Name"
              labelWidth={28}
              tooltip="Config profile name."
            >
              <Input
                className="width-30"
                defaultValue={options.jsonData.profile5}
                onChange={onUpdateDatasourceJsonDataOption(this.props, 'profile5')}
              />
      </InlineField>
      <InlineField
          label="Region"
          labelWidth={28}
          tooltip="Specify the Region"
        >
          <Select
            className="width-30"
            value={options.jsonData.region5 || ''}
            options={regions.map((region) => ({
              label: region,
              value: region,
              }))}
            defaultValue={options.jsonData.region5}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(this.props, 'region5')(option);
            }}
          />
        </InlineField>
        <InlineField
              label="User OCID"
              labelWidth={28}
              tooltip="User OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.user5 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'user5')}
                />
      </InlineField>
      <InlineField
              label="Tenancy OCID"
              labelWidth={28}
              tooltip="Tenancy OCID"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.tenancy5 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'tenancy5')}
                />
      </InlineField>
      <InlineField
              label="Fingerprint"
              labelWidth={28}
              tooltip="Fingerprint"
            >
              <Input
                className="width-30"
                placeholder={options.secureJsonFields.fingerprint5 ? 'configured' : ''}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'fingerprint5')}
                />
      </InlineField>
      <InlineField
              label="Private Key"
              labelWidth={28}
              tooltip="Private Key"
            >
              <TextArea
                type="text"
                className="width-30"
                placeholder={options.secureJsonFields.privkey5 ? 'configured' : ''}
                cols={20}
                rows={4}
                maxLength={4096}
                onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'privkey5')}
                />
      </InlineField>
      </FieldSet>
          </>
        )}


      </FieldSet>
    );
  }
}
