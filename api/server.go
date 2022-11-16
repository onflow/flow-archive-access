// Copyright 2021 Optakt Labs OÜ
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/fvm/blueprints"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-archive/models/archive"
	conv "github.com/onflow/flow-archive/models/convert"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// Server is a simple implementation of the generated AccessAPIServer interface.
// It uses an index reader interface as the backend to retrieve the desired data.
// This is generally an on-disk interface, but could be a GRPC-based index as
// well, in which case there is a double redirection.
type Server struct {
	index   archive.Reader
	codec   archive.Codec
	invoker Invoker
}

// NewServer creates a new server, using the provided index reader as a backend
// for data retrieval.
func NewServer(index archive.Reader, codec archive.Codec, invoker Invoker) *Server {
	s := Server{
		index:   index,
		codec:   codec,
		invoker: invoker,
	}

	return &s
}

// Ping implements the Ping endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#ping
func (s *Server) Ping(_ context.Context, _ *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}

// GetLatestBlockHeader implements the GetLatestBlockHeader endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getlatestblockheader
func (s *Server) GetLatestBlockHeader(ctx context.Context, _ *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	height, err := s.index.Last()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last block height: %w", err)
	}

	req := access.GetBlockHeaderByHeightRequest{
		Height: height,
	}

	return s.GetBlockHeaderByHeight(ctx, &req)
}

// GetBlockHeaderByID implements the GetBlockHeaderByID endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getblockheaderbyid
func (s *Server) GetBlockHeaderByID(ctx context.Context, in *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	blockID := flow.HashToID(in.Id)
	height, err := s.index.HeightForBlock(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get height for block %x: %w", blockID, err)
	}

	req := access.GetBlockHeaderByHeightRequest{
		Height: height,
	}

	return s.GetBlockHeaderByHeight(ctx, &req)
}

// GetBlockHeaderByHeight implements the GetBlockHeaderByHeight endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getblockheaderbyheight
func (s *Server) GetBlockHeaderByHeight(_ context.Context, in *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	header, err := s.index.Header(in.Height)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve block header at height %d: %w", in.Height, err)
	}

	decoder := signature.NewBlockSignerDecoder(nil)
	signerIDs, err := decoder.DecodeSignerIDs(header)
	if err != nil {
		return nil, fmt.Errorf("could not decode signer ids at height %d: %w", in.Height, err)
	}

	blockMsg, err := convert.BlockHeaderToMessage(header, signerIDs)
	if err != nil {
		return nil, fmt.Errorf("could not convert block header to RPC entity: %w", err)
	}

	resp := access.BlockHeaderResponse{
		Block: blockMsg,
	}

	return &resp, err
}

// GetLatestBlock implements the GetLatestBlock endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getlatestblock
func (s *Server) GetLatestBlock(ctx context.Context, in *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	height, err := s.index.Last()
	if err != nil {
		return nil, fmt.Errorf("could not get last height: %w", err)
	}

	req := &access.GetBlockByHeightRequest{
		Height: height,
	}

	return s.GetBlockByHeight(ctx, req)
}

// GetBlockByID implements the GetBlockByID endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getblockbyid
func (s *Server) GetBlockByID(ctx context.Context, in *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	blockID := flow.HashToID(in.Id)
	height, err := s.index.HeightForBlock(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get height for block %x: %w", blockID, err)
	}

	req := access.GetBlockByHeightRequest{
		Height: height,
	}

	return s.GetBlockByHeight(ctx, &req)
}

// GetBlockByHeight implements the GetBlockByHeight endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getblockbyheight
func (s *Server) GetBlockByHeight(_ context.Context, in *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	header, err := s.index.Header(in.Height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", in.Height, err)
	}

	sealIDs, err := s.index.SealsByHeight(in.Height)
	if err != nil {
		return nil, fmt.Errorf("could not get seals for height %d: %w", in.Height, err)
	}

	seals := make([]*entities.BlockSeal, 0, len(sealIDs))
	for _, sealID := range sealIDs {
		seal, err := s.index.Seal(sealID)
		if err != nil {
			return nil, fmt.Errorf("could not get seal with ID %x: %w", sealID, err)
		}

		blockID := seal.BlockID
		resultID := seal.ResultID

		// See https://github.com/onflow/flow-go/blob/v0.17.4/engine/common/rpc/convert/convert.go#L180-L188
		entity := entities.BlockSeal{
			BlockId:                    blockID[:],
			ExecutionReceiptId:         resultID[:],
			ExecutionReceiptSignatures: [][]byte{}, // filling seals signature with zero
		}
		seals = append(seals, &entity)
	}

	collIDs, err := s.index.CollectionsByHeight(in.Height)
	if err != nil {
		return nil, fmt.Errorf("could not get collections for height %d: %w", in.Height, err)
	}

	collections := make([]*entities.CollectionGuarantee, 0, len(collIDs))
	for _, collID := range collIDs {
		guarantee, err := s.index.Guarantee(collID)
		if err != nil {
			return nil, fmt.Errorf("could not get collection with ID %x: %w", collID, err)
		}

		entity := entities.CollectionGuarantee{
			CollectionId: conv.IDToHash(collID),
			Signatures:   [][]byte{guarantee.Signature},
		}
		collections = append(collections, &entity)
	}

	blockID := header.ID()
	block := entities.Block{
		Id:                   blockID[:],
		Height:               in.Height,
		ParentId:             header.ParentID[:],
		Timestamp:            timestamppb.New(header.Timestamp),
		CollectionGuarantees: collections,
		BlockSeals:           seals,
		Signatures:           [][]byte{header.ParentVoterSigData},
	}

	resp := access.BlockResponse{
		Block: &block,
	}

	return &resp, nil
}

// GetCollectionByID implements the GetCollectionByID endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getcollectionbyid
func (s *Server) GetCollectionByID(_ context.Context, in *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	collID := flow.HashToID(in.Id)
	collection, err := s.index.Collection(collID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection with ID %x: %w", in.Id, err)
	}

	collEntity := entities.Collection{
		Id: in.Id,
	}
	for _, txID := range collection.Transactions {
		collEntity.TransactionIds = append(collEntity.TransactionIds, conv.IDToHash(txID))
	}

	resp := access.CollectionResponse{
		Collection: &collEntity,
	}

	return &resp, err
}

// GetTransaction implements the GetTransaction endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#gettransaction
func (s *Server) GetTransaction(_ context.Context, in *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	txID := flow.HashToID(in.Id)
	tx, err := s.index.Transaction(txID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve transaction: %w", err)
	}

	resp := access.TransactionResponse{
		Transaction: convert.TransactionToMessage(*tx),
	}

	return &resp, nil
}

// GetTransactionResult implements the GetTransactionResult endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#gettransactionresult
func (s *Server) GetTransactionResult(_ context.Context, in *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	txID := flow.HashToID(in.Id)
	result, err := s.index.Result(txID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve transaction result: %w", err)
	}

	// We also need the height of the transaction we're looking at.
	height, err := s.index.HeightForTransaction(txID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve block height: %w", err)
	}

	block, err := s.index.Header(height)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve block header: %w", err)
	}
	blockID := block.ID()

	statusCode := uint32(0)
	if result.ErrorMessage == "" {
		statusCode = 1
	}

	status := entities.TransactionStatus_SEALED
	sealedHeight, err := s.index.Last()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last height: %w", err)
	}

	if height > sealedHeight {
		status = entities.TransactionStatus_EXECUTED
	}

	events, err := s.index.Events(height)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve events: %w", err)
	}

	resp := access.TransactionResultResponse{
		Status:       status,
		StatusCode:   statusCode,
		ErrorMessage: result.ErrorMessage,
		Events:       convert.EventsToMessages(events),
		BlockId:      blockID[:],
	}

	return &resp, nil
}

// GetTransactionResultByIndex implements the GetTransactionResultByIndex endpoint from the Flow Access API.
func (s *Server) GetTransactionResultByIndex(ctx context.Context, in *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	req := access.GetTransactionsByBlockIDRequest{
		BlockId: in.BlockId,
	}

	resp, err := s.GetTransactionResultsByBlockID(ctx, &req)
	for _, result := range resp.TransactionResults {
		for _, event := range result.Events {
			if event.TransactionIndex == in.Index {
				return result, nil
			}
		}
	}
	return nil, fmt.Errorf("could not get transactions for index %x: %w", in.Index, err)
}

// GetTransactionResultsByBlockID implements the GetTransactionResultsByBlockID endpoint from the Flow Access API.
func (s *Server) GetTransactionResultsByBlockID(ctx context.Context, in *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	blockId := flow.HashToID(in.BlockId)
	height, err := s.index.HeightForBlock(blockId)
	if err != nil {
		return nil, fmt.Errorf("could not get height for block %x: %w", blockId, err)
	}

	transactions, err := s.index.TransactionsByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get transactions for height %x: %w", height, err)
	}

	var transactionResults []*access.TransactionResultResponse
	for _, transaction := range transactions {
		req := access.GetTransactionRequest{
			Id: convert.IdentifierToMessage(transaction),
		}
		response, err := s.GetTransactionResult(ctx, &req)
		if err != nil {
			return nil, fmt.Errorf("could not get transaction for id %x: %w", transaction, err)
		}
		transactionResults = append(transactionResults, response)
	}

	resp := access.TransactionResultsResponse{
		TransactionResults: transactionResults,
	}
	return &resp, nil
}

// GetTransactionsByBlockID implements the GetTransactionsByBlockID endpoint from the Flow Access API.
func (s *Server) GetTransactionsByBlockID(ctx context.Context, in *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	blockId := flow.HashToID(in.BlockId)
	height, err := s.index.HeightForBlock(blockId)
	if err != nil {
		return nil, fmt.Errorf("could not get height for block %x: %w", blockId, err)
	}

	headerReq := access.GetBlockHeaderByHeightRequest{
		Height: height,
	}
	headerResp, err := s.GetBlockHeaderByHeight(ctx, &headerReq)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %x: %w", height, err)
	}
	header, err := convert.MessageToBlockHeader(headerResp.Block)
	if err != nil {
		return nil, fmt.Errorf("could not convert header for height %x: %w", height, err)
	}

	transactions, err := s.index.TransactionsByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get transactions for height %x: %w", height, err)
	}

	var transactionsEntity []*entities.Transaction
	for _, transaction := range transactions {
		req := access.GetTransactionRequest{
			Id: convert.IdentifierToMessage(transaction),
		}
		resp, err := s.GetTransaction(ctx, &req)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for id %x: %w", transaction, err)
		}

		transactionsEntity = append(transactionsEntity, resp.Transaction)
	}

	chain := header.ChainID.Chain()
	systemTx, err := blueprints.SystemChunkTransaction(chain)
	transactionsEntity = append(transactionsEntity, convert.TransactionToMessage(*systemTx))

	resp := access.TransactionsResponse{
		Transactions: transactionsEntity,
	}
	return &resp, nil
}

// GetAccount implements the GetAccount endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getaccount
func (s *Server) GetAccount(ctx context.Context, in *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	req := access.GetAccountAtLatestBlockRequest{
		Address: in.Address,
	}

	account, err := s.GetAccountAtLatestBlock(ctx, &req)
	if err != nil {
		return nil, err
	}

	resp := access.GetAccountResponse{
		Account: account.Account,
	}

	return &resp, nil
}

// GetAccountAtLatestBlock implements the GetAccountAtLatestBlock endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getaccountatlatestblock
func (s *Server) GetAccountAtLatestBlock(ctx context.Context, in *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	height, err := s.index.Last()
	if err != nil {
		return nil, fmt.Errorf("could not get height: %w", err)
	}

	// Simply call the height-specific endpoint with the latest height.
	req := &access.GetAccountAtBlockHeightRequest{
		Address:     in.Address,
		BlockHeight: height,
	}

	return s.GetAccountAtBlockHeight(ctx, req)
}

// GetAccountAtBlockHeight implements the GetAccountAtBlockHeight endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getaccountatblockheight
func (s *Server) GetAccountAtBlockHeight(_ context.Context, in *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	account, err := s.invoker.Account(in.BlockHeight, flow.BytesToAddress(in.Address))
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	accountMsg, err := convert.AccountToMessage(account)
	if err != nil {
		return nil, fmt.Errorf("could not convert account to RPC message: %w", err)
	}

	resp := access.AccountResponse{
		Account: accountMsg,
	}

	return &resp, nil
}

// ExecuteScriptAtLatestBlock implements the ExecuteScriptAtLatestBlock endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#executescriptatlatestblock
func (s *Server) ExecuteScriptAtLatestBlock(ctx context.Context, in *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	height, err := s.index.Last()
	if err != nil {
		return nil, fmt.Errorf("could not get last height: %w", err)
	}

	req := &access.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: height,
		Script:      in.Script,
		Arguments:   in.Arguments,
	}

	return s.ExecuteScriptAtBlockHeight(ctx, req)
}

// ExecuteScriptAtBlockID implements the ExecuteScriptAtBlockID endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#executescriptatblockid
func (s *Server) ExecuteScriptAtBlockID(ctx context.Context, in *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	blockID := flow.HashToID(in.BlockId)
	height, err := s.index.HeightForBlock(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get height for block ID %x: %w", blockID, err)
	}

	req := &access.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: height,
		Script:      in.Script,
		Arguments:   in.Arguments,
	}

	return s.ExecuteScriptAtBlockHeight(ctx, req)
}

// ExecuteScriptAtBlockHeight implements the ExecuteScriptAtBlockHeight endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#executescriptatblockheight
func (s *Server) ExecuteScriptAtBlockHeight(_ context.Context, in *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	var args []cadence.Value
	for _, arg := range in.Arguments {
		val, err := json.Decode(nil, arg)
		if err != nil {
			return nil, fmt.Errorf("could not decode script argument: %w", err)
		}

		args = append(args, val)
	}

	value, err := s.invoker.Script(in.BlockHeight, in.Script, args)
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	result, err := json.Encode(value)
	if err != nil {
		return nil, fmt.Errorf("could not encode script result: %w", err)
	}

	resp := access.ExecuteScriptResponse{
		Value: result,
	}

	return &resp, nil
}

// GetEventsForHeightRange implements the GetEventsForHeightRange endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#geteventsforheightrange
func (s *Server) GetEventsForHeightRange(_ context.Context, in *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	var types []flow.EventType
	if in.Type != "" {
		types = append(types, flow.EventType(in.Type))
	}

	var events []*access.EventsResponse_Result
	for height := in.StartHeight; height <= in.EndHeight; height++ {
		ee, err := s.index.Events(height, types...)
		if err != nil {
			return nil, fmt.Errorf("could not get events at height %d: %w", height, err)
		}

		header, err := s.index.Header(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header at height %d: %w", height, err)
		}

		timestamp, err := ptypes.TimestampProto(header.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("could not parse timestamp for block at height %d: %w", height, err)
		}

		messages := make([]*entities.Event, 0, len(ee))
		for _, event := range ee {
			messages = append(messages, convert.EventToMessage(event))
		}

		blockID := header.ID()
		result := access.EventsResponse_Result{
			BlockId:        blockID[:],
			BlockHeight:    height,
			BlockTimestamp: timestamp,
			Events:         messages,
		}

		events = append(events, &result)
	}

	resp := access.EventsResponse{
		Results: events,
	}

	return &resp, nil
}

// GetEventsForBlockIDs implements the GetEventsForBlockIDs endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#geteventsforblockids
func (s *Server) GetEventsForBlockIDs(_ context.Context, in *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	var types []flow.EventType
	if in.Type != "" {
		types = append(types, flow.EventType(in.Type))
	}

	var events []*access.EventsResponse_Result
	for _, id := range in.BlockIds {
		blockID := flow.HashToID(id)
		height, err := s.index.HeightForBlock(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not get height of block with ID %x: %w", id, err)
		}

		ee, err := s.index.Events(height, types...)
		if err != nil {
			return nil, fmt.Errorf("could not get events at height %d: %w", height, err)
		}

		header, err := s.index.Header(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header at height %d: %w", height, err)
		}

		timestamp, err := ptypes.TimestampProto(header.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("could not parse timestamp for block at height %d: %w", height, err)
		}

		messages := make([]*entities.Event, 0, len(ee))
		for _, event := range ee {
			messages = append(messages, convert.EventToMessage(event))
		}

		result := access.EventsResponse_Result{
			BlockId:        blockID[:],
			BlockHeight:    height,
			BlockTimestamp: timestamp,
			Events:         messages,
		}

		events = append(events, &result)
	}

	resp := access.EventsResponse{
		Results: events,
	}

	return &resp, nil
}

// GetNetworkParameters implements the GetNetworkParameters endpoint from the Flow Access API.
// See https://docs.onflow.org/access-api/#getnetworkparameters
func (s *Server) GetNetworkParameters(_ context.Context, _ *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	root, err := s.index.First()
	if err != nil {
		return nil, fmt.Errorf("could not get first indexed height: %w", err)
	}

	header, err := s.index.Header(root)
	if err != nil {
		return nil, fmt.Errorf("could not get header: %w", err)
	}

	return &access.GetNetworkParametersResponse{ChainId: header.ChainID.String()}, nil
}

// GetExecutionResultForBlockID is not implemented.
// See https://docs.onflow.org/access-api/#getexecutionresultforblockid
func (s *Server) GetExecutionResultForBlockID(_ context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	return nil, errors.New("GetExecutionResultForBlockID is not implemented by the Flow DPS API; please use the Flow Access API on a Flow access node directly")
}

// SendTransaction is not implemented.
// See https://docs.onflow.org/access-api/#sendtransaction
func (s *Server) SendTransaction(ctx context.Context, in *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	return nil, errors.New("SendTransaction is not implemented by the Flow DPS API; please use the Flow Access API on a Flow access node directly")
}

// GetLatestProtocolStateSnapshot is not implemented.
// See https://docs.onflow.org/access-api/#getlatestprotocolstatesnapshotrequest
func (s *Server) GetLatestProtocolStateSnapshot(ctx context.Context, in *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return nil, errors.New("GetLatestProtocolSnapshot is not implemented by the Flow DPS API; please use the Flow Access API on a Flow access node directly")
}
