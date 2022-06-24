/*
** Copyright © 2018, 2022 Oracle and/or its affiliates.
** The Universal Permissive License (UPL), Version 1.0
*/
export const AUTO = 'auto'
export const regions = ['af-johannesburg-1', 'ap-chiyoda-1', 'ap-chuncheon-1', 'ap-dcc-canberra-1', 'ap-hyderabad-1', 'ap-ibaraki-1', 'ap-melbourne-1',
                       'ap-mumbai-1', 'ap-osaka-1', 'ap-seoul-1', 'ap-singapore-1', 'ap-sydney-1', 'ap-tokyo-1', 'ca-montreal-1', 'ca-toronto-1',
                       'eu-amsterdam-1', 'eu-frankfurt-1', 'eu-madrid-1', 'eu-marseille-1', 'eu-milan-1', 'eu-paris-1', 'eu-stockholm-1', 'eu-zurich-1',
                       'il-jerusalem-1', 'me-abudhabi-1', 'me-dubai-1', 'me-jeddah-1', 'me-dcc-muscat-1', 'mx-queretaro-1', 'sa-santiago-1', 'sa-saopaulo-1', 'sa-vinhedo-1',
                       'uk-cardiff-1', 'uk-gov-cardiff-1', 'uk-gov-london-1', 'uk-london-1', 'us-ashburn-1', 'us-chicago-1', 'us-gov-ashburn-1',
                       'us-gov-chicago-1', 'us-gov-phoenix-1', 'us-langley-1', 'us-luke-1', 'us-phoenix-1']
export const namespaces = ['oci_computeagent', 'oci_blockstore', 'oci_lbaas', 'oci_telemetry']
export const aggregations = ['count()', 'max()', 'mean()', 'min()', 'rate()', 'sum()', 'percentile(.90)', 'percentile(.95)', 'percentile(.99)', 'last()']
export const windows = [AUTO, '1m', '5m', '1h']
export const resolutions = [AUTO, '1m', '5m', '1h']
export const environments = ['local', 'OCI Instance']

export const compartmentsQueryRegex = /^compartments\(\)\s*/
export const regionsQueryRegex = /^regions\(\)\s*/

