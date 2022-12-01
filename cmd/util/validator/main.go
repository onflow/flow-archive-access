// Utility to validate Access API and Archive-Access API return values

package main

import (
	"context"
	"fmt"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type APIValidator struct {
	ctx           context.Context
	archiveClient access.AccessAPIClient
	accessClient  access.AccessAPIClient
	script        []byte
	arguments     [][]byte
	blockID       []byte
	blockHeight   uint64
	accountAddr   []byte
}

func NewAPIValidator(accessAddr string, archiveAddr string, ctx context.Context) (*APIValidator, error) {
	accessClient := getAPIClient(accessAddr)
	archiveClient := getAPIClient(archiveAddr)
	return &APIValidator{
		accessClient:  accessClient,
		archiveClient: archiveClient,
	}, nil
}

func getAPIClient(addr string) access.AccessAPIClient {
	// connect to Archive-Access instance
	MaxGRPCMessageSize := 1024 * 1024 * 20 // 20MB
	conn, err := grpc.Dial(addr,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxGRPCMessageSize)))
	if err != nil {
		panic(fmt.Sprintf("unable to create connection to node: %s", addr))
	}
	return access.NewAccessAPIClient(conn)
}

func (a *APIValidator) CheckAPIResults() error {
	ctx := context.Background()
	// ExecuteScriptAtBlockID
	err := a.checkExecuteScriptAtBlockID(ctx)
	if err != nil {
		return fmt.Errorf("unsuccessful ExecuteScriptAtBlockID comparison: %w", err)
	}
	// ExecuteScriptAtBlockHeight
	err = a.checkExecuteScriptAtBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("unsuccessful ExecuteScriptAtBlockID comparison: %w", err)
	}
	// GetAccountAtBlockHeight
	err = a.checkGetAccountAtBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("unsuccessful ExecuteScriptAtBlockID comparison: %w", err)
	}
	return nil
}

func (a *APIValidator) checkExecuteScriptAtBlockID(ctx context.Context) error {
	req := &access.ExecuteScriptAtBlockIDRequest{
		BlockId:   a.blockID,
		Script:    a.script,
		Arguments: a.arguments[:],
	}
	accessRes, err := a.accessClient.ExecuteScriptAtBlockID(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockID from access node: %w", err)
	}
	archiveRes, err := a.archiveClient.ExecuteScriptAtBlockID(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockID from access node: %w", err)
	}
	if accessRes != archiveRes {
		return fmt.Errorf("unequal results! ExecuteScriptAtBlockID from access node: %w", err)
	}
	return nil
}

func (a *APIValidator) checkExecuteScriptAtBlockHeight(ctx context.Context) error {
	req := &access.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: a.blockHeight,
		Script:      a.script,
		Arguments:   a.arguments[:],
	}
	accessRes, err := a.accessClient.ExecuteScriptAtBlockHeight(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockHeight from access node: %w", err)
	}
	archiveRes, err := a.archiveClient.ExecuteScriptAtBlockHeight(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockHeight from access node: %w", err)
	}
	if accessRes != archiveRes {
		return fmt.Errorf("unequal results! ExecuteScriptAtBlockHeight from access node: %w", err)
	}
	return nil
}

func (a *APIValidator) checkGetAccountAtBlockHeight(ctx context.Context) error {
	req := &access.GetAccountAtBlockHeightRequest{
		Address:     a.accountAddr,
		BlockHeight: a.blockHeight,
	}
	accessRes, err := a.accessClient.GetAccountAtBlockHeight(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get GetAccountAtBlockHeight from access node: %w", err)
	}
	archiveRes, err := a.archiveClient.GetAccountAtBlockHeight(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get GetAccountAtBlockHeight from access node: %w", err)
	}
	if accessRes != archiveRes {
		return fmt.Errorf("unequal results! GetAccountAtBlockHeight from access node: %w", err)
	}
	return nil
}

func main() {
	// connect to Archive-Access instance
	ctx := context.Background()
	accessAddr := ""
	archiveAddr := ""
	// connect to Access instance
	apiValidator, err := NewAPIValidator(accessAddr, archiveAddr, ctx)
	// compare
	err = apiValidator.CheckAPIResults()
	if err != nil {
		log.Error().Err(fmt.Errorf("error while comparing API responses: %w", err))
	}
	log.Info().Msg("comparison successful, Archive and AN results match")
}
