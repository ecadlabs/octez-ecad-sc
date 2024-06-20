package main

import (
	"context"

	"github.com/ecadlabs/gotez/v2/client"
)

type Client interface {
	Heads(ctx context.Context, r *client.HeadsRequest) (<-chan *client.Head, <-chan error, error)
	Constants(ctx context.Context, r *client.ContextRequest) (client.Constants, error)
	BlockShellHeader(ctx context.Context, r *client.SimpleRequest) (*client.BlockShellHeader, error)
	BlockProtocols(ctx context.Context, r *client.SimpleRequest) (*client.BlockProtocols, error)
	BasicBlockInfo(ctx context.Context, chain string, block string) (*client.BasicBlockInfo, error)
	IsBootstrapped(ctx context.Context, r *client.ChainID) (*client.BootstrappedResponse, error)
}
