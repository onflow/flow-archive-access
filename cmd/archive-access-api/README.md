# Flow Access Server

## Description

The Flow Access Server runs on top of a DPS index to implement the [Flow Access API](https://developers.flow.com/nodes/access-api).
Both the Flow DPS Indexer and the Flow DPS Live tool can create such an index.
In the case of the indexer, the index is static and built from a previous spork's state.
For the live tool, the index is dynamic and updated on an ongoing basis from the data sent from a Flow execution node.

## Usage

```sh
Usage of archive-access-api:
  -a, --address string    address to serve GRPC API on (default "127.0.0.1:9000")
  -d, --archive string    host URL for DPS API endpoint (default "127.0.0.1:80")
  -l, --log string        log output level (default "info")
      --cache-size uint   maximum cache size for register reads in bytes (default 1000000000)
```

## Example

The following command line starts the DPS Access API server to serve requests on port `5006`.

```sh
./archive-access-api -a "127.0.0.1:5005" -p 5006
```
