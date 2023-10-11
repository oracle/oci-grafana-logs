# Grafana Cloud - Oracle Cloud Infrastructure Data Source for Grafana 

## Background

Grafana is a popular technology that makes it easy to visualize logs. The [Oracle Cloud Infrastructure Logs Data Source for Grafana](https://grafana.com/grafana/plugins/oci-logs-datasource/) is used to extend Grafana by adding OCI Logging as a data source. The plugin enables you to visualize log records (service, audit, and custom) and Logging derived from log records stored in the OCI Logging service.

This walkthrough is intended for use by people who would like to deploy Grafana and the OCI Logs Data Source in Grafana Cloud. 

Make sure you have access to the [Logging Service](https://docs.oracle.com/en-us/iaas/Content/Logging/Concepts/loggingoverview.htm) and that the logs you want to observe and analyze are being collected in your tenancy. See the OCI Logging documentation for information on how to collect or access:
* [Logs from your compute instances](https://docs.oracle.com/en-us/iaas/Content/Logging/Concepts/agent_management.htm)
* [Custom logs from your application/services](https://docs.oracle.com/en-us/iaas/Content/Logging/Concepts/custom_logs.htm)
* [OCI service logs](https://docs.oracle.com/en-us/iaas/Content/Logging/Concepts/service_logs.htm).

## Getting OCI Configuration values

To configure OCI Logging Grafana Data Source, you'll need to get the necessary provider and resource settings. Please note that Migrating from version 2.x.x to 3.x.x will require migrating the existing data source configuration: using version 3.x.x of the plugin with the data source configuration of version 2.x.x is **not possible**. In case you are migrating from previous version 2.x.x of the OCI Logging Grafana Plugin, you can refer to the [**Migration Instructions for Grafana OCI Logging Data Source Settings (User Principals and Single Tenancy mode only)**](migration.md). If you are configuring the plugin to work in Multitenancy Mode, you will need to repeat the following steps for each of the Tenancies you want to configure with the plugin (up to 5 additional Tenancies are supported).

### Getting the Region

To get the region for your OCI cloud, follow these steps:

1. Log in to the OCI console.
2. From the OCI menu, select the **Region** dropdown in the top right corner of the page.
3. The region is listed next to **Home**.

For details and reference, see: [Regions and Availability Domains](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm#top)
Please make note of the region as you'll need it later to configure your OCI Logging Grafana Data Source.

### Getting the Tenancy OCID

To get the tenancy OCID, follow these steps:

1. Log in to the OCI console.
2. From the OCI menu, click on your profile icon on the top right:

![OCI Administration](images/oci_administration.png)

3. Click on Tenancy
4. The tenancy OCID is listed in the **Tenancy Information** section.

![OCI Tenancy](images/oci_tenancy.png)

For details and reference, see: [Where to Get the Tenancy's OCID and User's OCID](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five)
Please make note of the tenancy OCID as you'll need it later to configure your OCI Logging Grafana Data Source.

### Getting the User OCID

To get the user OCID, follow these steps:

1. Log in to the OCI console.
2. From the OCI menu, select **Identity** > **Users**.
3. Click on the user you want to use with OCI Logging Grafana Data Source.
4. The user OCID is listed in the **User Details** section.

![OCI User](images/oci_user.png)

For details and reference, see: [Where to Get the Tenancy's OCID and User's OCID](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five).
Please make note of the user OCID as you'll need it later to configure your OCI Logging Grafana Data Source.

### Getting the Private API Key and Fingerprint

To get the private key, follow these steps:

1. Log in to your **OCI tenancy** and click on your username in the top right corner.
2. Go to **Resources** and **API Keys** and click on **Add API Key**.
3. Choose if you want to generate a new API key or use your own:
    - Select **Generate API Key Pair** if you want to generate a new API key. Click then on **Download Private Key** and **Download Public Key** to get your newly generated key
    - Select **Public Key File** or **Paste Public Key** in case you want to paste your public key: select **Paste Public Key** in the **Add API Key** dialog and copy and paste the key contents into the field, then click **Add**.


![OCI API Key](images/oci_apikey.png)

4. Once the key is added take note of the API key fingerprint listed in the **Fingerprint** column.

![OCI Fingerprint](images/oci_fingerprint.png)


For details on how to create and configure keys see [How to Generate an API Signing Key](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#two) and [How to Upload the Public Key](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#three).
Make note of the private key file location and API key fingerprint as you'll need it later to configure your OCI Logging Grafana Data Source.

## Configure OCI Identity Policies

In the OCI console under **Identity > Groups** click **Create Group** and create a new group called **grafana**. Add the user configured in the OCI CLI to the newly-created group. 

![OCIConsole-GroupList-Screenshot](images/OCIConsole-GroupList-Screenshot.png)

Under the **Policy** tab click **Create Policy** and create policies allowing the group to read tenancy log objects and content. Add the following policy statements:

- `allow group grafana to read log-groups in tenancy`
- `allow group grafana to read log-content in tenancy`
- `allow group grafana to read compartments in tenancy`
- `allow group grafana to read audit-events in tenancy`

![OCIConsole-GroupLogsPolicyCreate-Screenshot](images/OCIConsole-GroupLogsPolicyCreate-Screenshot.png)

The first two policies can also be limited to specific compartments in your tenancy by adding additional qualifiers to the policy statements.

## Install Grafana and the OCI Logging Plugin for Grafana Cloud

To [install OCI Logging Plugin](https://grafana.com/grafana/plugins/oci-Logging-datasource/) on Grafana Cloud you need a valid [Grafana Cloud Account](https://grafana.com/products/cloud/).

Log in to your Grafana Cloud Account and go to the **Administration** section:
![Administration](images/grafanacloud-administration.png)

Select **Plugins** and search for **oracle**:
![Plugins](images/grafanacloud-plugins.png)

Click on **Oracle Cloud Infrastructure Logging**. You will see the installation option for the plugin. Choose **install via grafana.con**:
![Install](images/grafanacloud-installlogs.png)

You will be forwarded to the grafana.com website where you can proceed with the one-click installation. Make sure you will choose the correct Grafana Cloud account (in this example the account is named **Oracle** yours will be different) and follow the instruction on this web page to complete the plugin installation:
![One-click](images/grafanacloud-oneclick.png)


## Configure Grafana

### Configure Plugin in Single Tenancy Mode
If you selected **single** as **Tenancy mode** then fill in the following credentials:

* `Profile Name` - A user-defined name for this profile. In **single** mode this is automatically set to **DEFAULT** and cannot be modified.
* `Region` - An OCI region. To get the value, see [**Getting Region Configuration value**](#getting-the-region).
* `User OCID` - OCID of the user calling the API. To get the value, see [**Getting User OCID Configuration value**](#getting-the-user-OCID).* 
* `Tenancy OCID` - OCID of your tenancy. To get the value, see [**Getting Tenancy OCID Configuration value**](#getting-the-tenancy-OCID).
* `Fingerprint` - Fingerprint for the key pair being used. To get the value, see [**Getting Fingerprint Configuration value**](#getting-the-private-api-key-and-fingerprint).
* `Private Key` - The contents of the private key file. To get the value, see [**Getting Private Key Configuration value**](#getting-the-private-api-key-and-fingerprint).

The configured data source will look like the following:

![Datasource Filled](images/logs_single_filled.png)

Click **Save & Test** to return to the home dashboard.


### Configure Plugin in Multi-Tenancy Mode
If you selected **multi** as **Tenancy mode** then fill in the following credentials for **each Tenancy you want to configure (up to 5 additional tenancies)**:

* `Profile Name` - A user-defined name for this profile. The first Tenancy is automatically set to **DEFAULT** and cannot be modified. You need to specify a custom and unique Profile name for each of the additional tenancies.
* `Region` - An OCI region. To get the value, see [**Getting Region Configuration value**](#getting-the-region).
* `User OCID` - OCID of the user calling the API. To get the value, see [**Getting User OCID Configuration value**](#getting-the-user-OCID).
* `Tenancy OCID` - OCID of your tenancy. To get the value, see [**Getting Tenancy OCID Configuration value**](#getting-the-tenancy-OCID).
* `Fingerprint` - Fingerprint for the key pair being used. To get the value, see [**Getting Fingerprint Configuration value**](#getting-the-private-api-key-and-fingerprint).
* `Private Key` - The contents of the private key file. To get the value, see [**Getting Private Key Configuration value**](#getting-the-private-api-key-and-fingerprint).

By default, if you selected **multi** as **Tenancy mode** you can configure one DEFAULT tenancy with an additional one. You may add others tenancy **(up to 5 additional tenancies)** using the **Add another Tenancy** checkbox.

The configured data source will look like the following:

![Datasource Filled](images/logs_multi_filled.png)

Click **Save & Test** to return to the home dashboard.

After the initial configuration, you can modify the datasource by adding a new tenancy by clicking on the **Add another Tenancy** checkbox and filling in the additional credentials. You can also disable a configured Tenancy leaving empty the **Profile Name** as in this screenshot:

![Tenancy Disabled](images/multi_disable.png)



On the Oracle Cloud Infrastructure Logs data source configuration page, fill in your **Tenancy OCID**, **Default Region**, and **Authentication Provider**. Your **Default region** is the same as your home region listed on the **Tenancy Details** page. For **Authentication Provider** choose **OCI Instance**. 

Click **Save & Test** to test the configuration of the Logs data source. Click the Dashboard icon in the left-hand navigation menu to return to the home dashboard.

## Next Steps

Check out how to use the newly installed and configured plugin in our [Using Grafana with Oracle Cloud Infrastructure Data Source](using.md) walkthrough. 

