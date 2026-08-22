# Trivial Oracle Cloud DNS updater via HTTP API

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
  "username": "HTTP authentication user name",
  "password": "HTTP authentication password",
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

Only one endpoint is available: `/nic/update` (`GET` or `POST`). The HTTP basic authentication is expected. Example:
```shell
  curl 'https://username:password@my-domain/nic/update'
```

The endpoint speaks [dyndns2](https://help.dyn.com/remote-access-api/perform-update/), so it can be used directly by
standard clients such as `ddclient`:

```
protocol=dyndns2, server=my-domain, login=username, password=password  my-host.example.com
```

The reply is a `text/plain` dyndns2 status line. `good` and `nochg` have no equivalent in the HTTP status, so
they are distinguished only by the body; failures keep a real HTTP status as well:

| Response     | HTTP | Meaning                                                             |
|--------------|------|---------------------------------------------------------------------|
| `good <ip>`  | 200  | The DNS record was updated to `<ip>`                                 |
| `nochg <ip>` | 200  | The record already held `<ip>`; nothing was written                  |
| `badauth`    | 401  | The basic authentication credentials did not match                   |
| `911`        | 500  | Something failed on the server side; the client should retry later   |

The IP address is always the caller's own address, taken from `X-Forwarded-For` if present and from the connection
otherwise. The `hostname` and `myip` parameters that dyndns2 clients send are ignored: the hosts to update come from
the configuration file.

Records are read before they are written, so a client polling on a short interval does not rewrite an
already-correct record.

For non-dyndns2 callers, sending `Accept: application/json` returns the outcome as `{"message": "..."}` instead,
under the same HTTP status codes as above. A weight of zero (`Accept: application/json;q=0`) opts back out.

