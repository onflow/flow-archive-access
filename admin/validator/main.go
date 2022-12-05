// Utility to validate Access API and Archive-Access API return values

package main

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"os"
	"time"
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
	accountAddr := flow.HexToAddress("e467b9dd11fa00df").Bytes()
	recentBlock, err := accessClient.GetLatestBlock(ctx, &access.GetLatestBlockRequest{})
	// allow for archive node to sync block
	time.Sleep(5)
	if err != nil {
		return nil, fmt.Errorf("unable to get latest block from AN")
	}
	blockID := recentBlock.GetBlock().GetId()
	blockHeight := recentBlock.GetBlock().Height
	scriptPath := "get_token_balance.cdc"
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read cadence script to initilaize: %w", err)
	}
	scriptArgs := make([][]byte, 0)
	return &APIValidator{
		ctx:           ctx,
		accountAddr:   accountAddr,
		script:        script,
		blockID:       blockID,
		arguments:     scriptArgs,
		blockHeight:   blockHeight,
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

func (a *APIValidator) CheckAPIResults(ctx context.Context) error {
	log.Info().Msg("starting comparison")
	// ExecuteScriptAtBlockID
	err := a.checkExecuteScriptAtBlockID(ctx)
	if err != nil {
		return fmt.Errorf("unsuccessful ExecuteScriptAtBlockID comparison: %w", err)
	}
	log.Info().Msg("checkExecuteScriptAtBlockID successful")
	// ExecuteScriptAtBlockHeight
	err = a.checkExecuteScriptAtBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("unsuccessful ExecuteScriptAtBlockHeight comparison: %w", err)
	}
	log.Info().Msg("checkExecuteScriptAtBlockHeight successful")
	// GetAccountAtBlockHeight
	err = a.checkGetAccountAtBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("unsuccessful checkGetAccountAtBlockHeight comparison: %w", err)
	}
	log.Info().Msg("checkGetAccountAtBlockHeight successful")
	return nil
}

func (a *APIValidator) checkExecuteScriptAtBlockID(ctx context.Context) error {
	req := &access.ExecuteScriptAtBlockIDRequest{
		BlockId:   a.blockID,
		Script:    a.script,
		Arguments: a.arguments,
	}
	accessRes, err := a.accessClient.ExecuteScriptAtBlockID(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockID from access node: %w", err)
	}
	log.Debug().Msg(fmt.Sprintf("received GetAccountAtBlockHeight response from AN: %s", accessRes.String()))
	archiveRes, err := a.archiveClient.ExecuteScriptAtBlockID(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockID from archive node: %w", err)
	}
	log.Debug().Msg(fmt.Sprintf("received GetAccountAtBlockHeight response from Archive: %s", archiveRes.String()))
	if accessRes.String() != archiveRes.String() {
		return fmt.Errorf("unequal results! ExecuteScriptAtBlockID from access node: %w", err)
	}
	return nil
}

func (a *APIValidator) checkExecuteScriptAtBlockHeight(ctx context.Context) error {
	req := &access.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: a.blockHeight,
		Script:      a.script,
		Arguments:   a.arguments,
	}
	accessRes, err := a.accessClient.ExecuteScriptAtBlockHeight(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockHeight from access node: %w", err)
	}
	log.Debug().Msg(fmt.Sprintf("received ExecuteScriptAtBlockHeight response from AN: %s", accessRes.String()))
	archiveRes, err := a.archiveClient.ExecuteScriptAtBlockHeight(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get ExecuteScriptAtBlockHeight from access node: %w", err)
	}
	log.Debug().Msg(fmt.Sprintf("received ExecuteScriptAtBlockHeight response from Archive: %s", archiveRes.String()))
	if accessRes.String() != archiveRes.String() {
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
	log.Debug().Msg(fmt.Sprintf("received GetAccountAtBlockHeight response from AN: %s", accessRes.String()))
	archiveRes, err := a.archiveClient.GetAccountAtBlockHeight(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get GetAccountAtBlockHeight from access node: %w", err)
	}
	log.Debug().Msg(fmt.Sprintf("received GetAccountAtBlockHeight response from Archive: %s", archiveRes.String()))
	if accessRes.String() != archiveRes.String() {
		return fmt.Errorf("unequal results! GetAccountAtBlockHeight from access node: %w", err)
	}
	return nil
}

func main() {
	// connect to Archive-Access instance
	ctx := context.TODO()
	accessAddr := "access.mainnet.nodes.onflow.org:9000"
	archiveAddr := "archive.mainnet.nodes.onflow.org:9000"
	// connect to Access instance
	apiValidator, err := NewAPIValidator(accessAddr, archiveAddr, ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to initialize validator")
		return
	}
	// compare
	err = apiValidator.CheckAPIResults(ctx)
	if err != nil {
		print(err.Error())
		log.Info().Err(fmt.Errorf("error while comparing API responses: %w", err))
		return
	}
	log.Info().Msg("comparison successful, Archive and AN results match")
}
