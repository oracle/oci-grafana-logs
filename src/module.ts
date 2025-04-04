import { DataSourcePlugin } from '@grafana/data';
import { OCIDataSource } from './datasource';
import { ConfigEditor } from './ConfigEditor';
import { QueryEditor } from './QueryEditor';
import { OCIQuery, OCIDataSourceOptions } from './types';

/**
 * Registers the OCI Data Source Plugin with Grafana.
 *
 * This plugin integrates OCI data source functionality, allowing users to configure
 * and query OCI data within Grafana.
 *
 * The plugin initializes the data source and sets up configuration and query editors.
*/
export const plugin = new DataSourcePlugin<OCIDataSource, OCIQuery, OCIDataSourceOptions>(OCIDataSource)
  .setConfigEditor(ConfigEditor) // Assigns the configuration editor component.
  .setQueryEditor(QueryEditor) // Assigns the query editor component.
