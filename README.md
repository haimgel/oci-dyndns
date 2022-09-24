# Trivial Oracle Cloud DNS updater via HTTP request

This is a trivial dynamic DNS updater for Oracle Cloud: the idea is to deploy this to [OKE](https://www.oracle.com/ca-en/cloud/cloud-native/container-engine-kubernetes/)
and have a simple HTTPS API endpoint to register my home IP address as a DNS record in the DNS zone served by Oracle DNS.

## Why?

* I want to be able to connect to my home server, and my home IP address changes frequently.
* Existing dynamic DNS services introduce an additional point of failure, I'd rather not add complexity where it's not needed.
* Oracle DNS does not support standard dynamic DNS protocols (RFC 2136, etc.).
* Calling `curl` in DHCP client hook or just as a cronjob is incredibly simple.
* I wanted to write _something_ somewhat useful to get my feet wet with Go.

## How?

Either build the executable manually, or use the provided container image. Two command-line parameters are supported:
```text
Usage of ./oci-dyndns:
  -config string
    	Configuration file name (default "config.json")
  -listen string
    	Address and port to listen to (default ":8080")
```

The configuration file is expected to contain the following:
```json
{
  "zone": "DNS zone to update",
  "host": "Host name to update, FQDN",
  "token": "HTTP authentication token -- has to be passed as token=value in the HTTP request",
  "oci": {
    "tenancy"              : "OCI user credentials that can update DNS records",
    "user"                 : "",
    "region"               : "",
    "fingerprint"          : "",
    "privateKey"           : ""
  }
}
```

## API 

Only one endpoint is available: `POST /update`. The only expected parameter is `token`. Example:
```shell
  curl -X POST 'https://my-domain/update?token=secretValue'
```

