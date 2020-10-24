---
title: muse API Reference

language_tabs:
  - shell: curl
  - go: Go

search: true
---

# Introduction

This page describes the `muse` HTTP API. `muse` is a Sia file contract
server that enables clients to store and retrieve data on Sia hosts.


# Authentication

The `muse` API is unauthenticated. Use a reverse proxy such as Caddy or Nginx to
protect your server if you plan to expose it over the Internet.


# Routes

## List Contracts

> Example Request:

```shell
curl "localhost:9580/contracts" 
```

```go
mc := muse.NewClient("localhost:9580")
contracts, err := mc.AllContracts()
```

> Example Response:

```json
[{
  "hostKey": "ed25519:8408ad8d5e7f605995bdf9ab13e5c0d84fbe1fc610c141e0578c7d26d5cfee75",
  "id": "f506d7f1c03f40554a6b15da48684b96a3661be1b5c5380cd46d8a9efee8b6ff",
  "renterKey": "ZGT+mRdBTnnnll4WxHUXb1k9FFXLi6KXI88w2mVPAbk2XUhxycLzyssGlLvYr1h4e50szNntLOofDY9z7TjCJg==",
  "hostAddress": "example.com:9982",
  "endHeight": 456000
}]
```

> Specifying a host set:

```shell
curl "localhost:9580/contracts?hostset=foo" 
```

```go
mc := muse.NewClient("localhost:9580")
contracts, err := mc.Contracts("foo")
```

> Example Response:

```json
[{
  "hostKey": "ed25519:8408ad8d5e7f605995bdf9ab13e5c0d84fbe1fc610c141e0578c7d26d5cfee75",
  "id": "f506d7f1c03f40554a6b15da48684b96a3661be1b5c5380cd46d8a9efee8b6ff",
  "renterKey": "ZGT+mRdBTnnnll4WxHUXb1k9FFXLi6KXI88w2mVPAbk2XUhxycLzyssGlLvYr1h4e50szNntLOofDY9z7TjCJg==",
  "hostAddress": "example.com:9982",
  "endHeight": 456000
}]
```

Returns the contracts formed by the server. If a `hostset` is specified, only
the most recent contract for each host in the set is returned (where "most
recent" means "highest end height"). Otherwise, all contracts are returned,
including contracts that have expired.

### HTTP Request

`GET http://localhost:9580/contracts`

### URL Parameters

Parameter | Description
----------|------------
 hostset  | The name of the host set to query

### Errors

  Code | Description
-------|------------
  400  | Unknown host set


## Form a Contract

> Example Request:

```shell
curl "localhost:9580/form" \
  -X POST \
  -d '{
    "hostKey": "ed25519:8408ad8d5e7f605995bdf9ab13e5c0d84fbe1fc610c141e0578c7d26d5cfee75",
    "funds": "13000000000000000000000000000",
    "startHeight": 123000,
    "endHeight": 456000,
    "settings": {
      "unlockHash": "8066f825fd680559acba2c14ca7e8b0f4aa5e8a1eece3908485953d6a2e8ce3b991322eaf7d1",
      "windowSize": 1000,
      "collateral": "1020304050",
      "maxCollateral": "187293910",
      "contractPrice": "100000000000000",
      "downloadBandwidthPrice": "5010293919",
      "storagePrice": "810202449581",
      "uploadBandwidthPrice": "4429104182"
    }
  }'
```

```go
mc := muse.NewClient("localhost:9580")
contract, err := mc.Form(hostKey, funds, start, end, settings)
```

> Example Response:

```json
{
  "hostKey": "ed25519:8408ad8d5e7f605995bdf9ab13e5c0d84fbe1fc610c141e0578c7d26d5cfee75",
  "id": "f506d7f1c03f40554a6b15da48684b96a3661be1b5c5380cd46d8a9efee8b6ff",
  "renterKey": "ZGT+mRdBTnnnll4WxHUXb1k9FFXLi6KXI88w2mVPAbk2XUhxycLzyssGlLvYr1h4e50szNntLOofDY9z7TjCJg==",
  "hostAddress": "example.com:9982",
  "endHeight": 456000
}
```

Forms a contract with a host. The settings should be obtained from
[`/scan`](#scan-a-host) (or by directly invoking the RPC on the host). If the
settings have changed in the interim, the host may reject the contract.

<aside class="notice">
Only a subset of the fields returned by <code>/scan</code> need to be included in the
request. For convenience, however, you can pass the entire object.
</aside>

<aside class="warning">
The <code>renterKey</code> included in the response can spend all of the renter
funds in the contract. Do not share the key with untrusted parties.
</aside>

### HTTP Request

`POST http://localhost:9580/form`

### Errors

  Code | Description
-------|------------
  400  | Invalid request object
  500  | Host unavailable or rejected contract


## Renew a Contract

> Example Request:

```shell
curl "localhost:9580/renew" \
  -X POST \
  -d '{
    "id": "f506d7f1c03f40554a6b15da48684b96a3661be1b5c5380cd46d8a9efee8b6ff",
    "funds": "13000000000000000000000000000",
    "startHeight": 123000,
    "endHeight": 456000,
    "settings": {
      "unlockHash": "8066f825fd680559acba2c14ca7e8b0f4aa5e8a1eece3908485953d6a2e8ce3b991322eaf7d1",
      "windowSize": 1000,
      "collateral": "1020304050",
      "maxCollateral": "187293910",
      "contractPrice": "100000000000000",
      "downloadBandwidthPrice": "5010293919",
      "storagePrice": "810202449581",
      "uploadBandwidthPrice": "4429104182"
    }
  }'
```

```go
mc := muse.NewClient("localhost:9580")
contract, err := mc.Renew(id, funds, start, end, settings)
```

> Example Response:

```json
{
  "hostKey": "ed25519:8408ad8d5e7f605995bdf9ab13e5c0d84fbe1fc610c141e0578c7d26d5cfee75",
  "id": "409d8f79b468f953c0405beea48072b3da79b23ea7a9b4844b2fc6ccfc3bfbb4",
  "renterKey": "09BA6bj4J8kTvmLKzA2WS+UEfTJZdpQnW/45KRMNM+/4vZlnMOX8zTiszxMZLRfe1kXqJzA95jWOTAImC/UZTw==",
  "hostAddress": "example.com:9982",
  "endHeight": 456000
}
```

Renews a contract with a host. The ID must refer to a contract previously formed
by the server. The settings should be obtained from [`/scan`](#scan-a-host) (or
by directly invoking the RPC on the host). If the settings have changed in the
interim, the host may reject the contract.

<aside class="notice">
Only a subset of the fields returned by <code>/scan</code> need to be included in the
request. For convenience, however, you can pass the entire object.
</aside>

### HTTP Request

`POST http://localhost:9580/renew`

### Errors

  Code | Description
-------|------------
  400  | Invalid request object, or unknown ID
  500  | Host unavailable, or host rejected contract


## Delete a Contract

> Example Request:

```shell
curl "localhost:9580/delete/f506d7f1c03f40554a6b15da48684b96a3661be1b5c5380cd46d8a9efee8b6ff" \
  -X POST
```

```go
mc := muse.NewClient("localhost:9580")
err := mc.Delete(id)
```

Deletes a contract from the server. The ID must refer to a contract previously
formed by the server. The contract itself is not revised or otherwise affected
in any way. In general, contracts should only be deleted once they have expired
and are no longer needed.

<aside class="notice">
It is rarely necessary to delete a contract, since host sets can be used to
filter out contracts that you do not wish to use. However, deletion can be useful
if you have multiple contracts with the same host, or if your contract metadata
files are consuming too much disk space.
</aside>

### HTTP Request

`POST http://localhost:9580/delete/<id>`

### Errors

  Code | Description
-------|------------
  400  | Invalid contract ID
  500  | Contract file could not be removed


## Scan a Host

> Example Request:

```shell
curl "localhost:9580/scan" \
  -X POST \
  -d '{
    "hostKey": "ed25519:8408ad8d5e7f605995bdf9ab13e5c0d84fbe1fc610c141e0578c7d26d5cfee75"
  }'
```

```go
mc := muse.NewClient("localhost:9580")
settings, err := mc.Scan(hostKey)
```

> Example Response:

```json
{
  "acceptingContracts": true,
  "maxDownloadBatchSize": 4194304,
  "maxDuration": 4032,
  "maxReviseBatchSize": 4194304,
  "netAddress": "example.com:9982",
  "remainingStorage": 961430400,
  "sectorSize": 4194304,
  "totalStorage": 5291130400,
  "unlockHash": "8066f825fd680559acba2c14ca7e8b0f4aa5e8a1eece3908485953d6a2e8ce3b991322eaf7d1",
  "windowSize": 1000,
  "collateral": "1020304050",
  "maxCollateral": "187293910",
  "contractPrice": "100000000000000",
  "downloadBandwidthPrice": "5010293919",
  "storagePrice": "810202449581",
  "uploadBandwidthPrice": "4429104182",
  "baseRPCPrice": "5291046600",
  "sectorAccessPrice": "315291095",
  "revisionNumber": 6512,
  "version": "1.4.2.1"
}
```

Requests that the server connect to a host and query its current settings.

### HTTP Request

`POST http://localhost:9580/scan`

### Errors

  Code | Description
-------|------------
  400  | Invalid request object
  500  | Host unavailable




## List Host Sets

> Example Request:

```shell
curl "localhost:9580/hostsets"
```

```go
mc := muse.NewClient("localhost:9580")
hostSets, err := mc.HostSets()
```

> Example Response:

```json
[
  "foo",
  "bar",
  "baz"
]
```

Returns the names of all current host sets.

### HTTP Request

`GET http://localhost:9580/hostsets`

### Errors

None


## List Hosts in Host Set

> Example Request:

```shell
curl "localhost:9580/hostsets/foo"
```

```go
mc := muse.NewClient("localhost:9580")
hosts, err := mc.HostSet("foo")
```

> Example Response:

```json
[
  "ed25519:02082d6c02d714f7d700ae2d4d9207c2183c2d9bb1bc991fa13af8e8b198c684",
  "ed25519:b3917ced8a4fd059f0c23e8c8ae32b672d63e87b0c758cb914603b0363ac2c9a",
  "ed25519:76ee361cce9d2586acd3bf510ff0903654b553c9b657b7da983b5b000b49b0d6",
  "ed25519:88aa3399790e216907fdf09ce38f592fbc2d99f6e1b56e1fdb056f27b6167693"
]
```

Returns the public keys of all hosts in the specified host set.

### HTTP Request

`GET http://localhost:9580/hostsets/<name>`

### Errors

  Code | Description
-------|------------
  400  | Unknown host set


## Create or Modify a Host Set

> Example Request:

```shell
curl "localhost:9580/hostsets/foo" \
  -X PUT \
  -d '[
    "ed25519:02082d6c02d714f7d700ae2d4d9207c2183c2d9bb1bc991fa13af8e8b198c684",
    "ed25519:b3917ced8a4fd059f0c23e8c8ae32b672d63e87b0c758cb914603b0363ac2c9a",
    "ed25519:76ee361cce9d2586acd3bf510ff0903654b553c9b657b7da983b5b000b49b0d6",
    "ed25519:88aa3399790e216907fdf09ce38f592fbc2d99f6e1b56e1fdb056f27b6167693"
  ]'
```

```go
mc := muse.NewClient("localhost:9580")
err := mc.SetHostSet("foo", hosts)
```

Creates, modifies, or deletes a host set. If the host set does not exist, it is
created. If it exists, it is overwritten with the new values. If the request
body is an empty array, the host set is deleted.

### HTTP Request

`PUT http://localhost:9580/hostsets/<name>`

### Errors

  Code | Description
-------|------------
  400  | Invalid request object


# Shard

All `muse` servers can also be used as [shard](https://github.com/lukechampine/shard)
servers by appending `/shard` to the URL.

<br>