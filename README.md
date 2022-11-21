## Access API

Flow DPS implements the [Flow Access API Specification](https://developers.flow.com/nodes/access-api), except for the following endpoints:

* `SendTransaction`
* `GetLatestProtocolStateSnapshot`
* `GetExecutionResultForBlockID`

It exposes Flow-specific resources such as [`flow.Block`](https://pkg.go.dev/github.com/onflow/flow-go/model/flow#Block), [`flow.Event`](https://pkg.go.dev/github.com/onflow/flow-go/model/flow#Event), [`flow.Transaction`](https://pkg.go.dev/github.com/onflow/flow-go/model/flow#Transaction) and many others.

For more information on the various endpoints of this API, please consult the [official Flow documentation](https://docs.onflow.org/access-api).