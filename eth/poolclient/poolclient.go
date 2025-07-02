package poolclient

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

//go:generate go run ../../tools/poolclient_generate.go

// type PoolClientInterface interface {
// 	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
// 	BlockNumber(ctx context.Context) (uint64, error)
// 	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
// 	ChainID(ctx context.Context) (*big.Int, error)
// 	// Add other methods used by go-livepeer
// }

// NewClient manages a pool of ethclient.Client instances with retry logic.
type PoolClient struct {
	clients      []*ethclient.Client
	currentIndex int
	mu           sync.Mutex
	maxRetries   int
	backoff      time.Duration
}

// PoolClientConfig holds configuration for EthClientPool.
type PoolClientConfig struct {
	RPCURLs    string
	MaxRetries int
	Backoff    time.Duration
	ctx        context.Context
}

func (pcc *PoolClientConfig) RPCURLsSlice() ([]string, error) {
	if pcc.RPCURLs == "" {
		return nil, fmt.Errorf("RPC URLs cannot be empty")
	}
	urls := strings.Split(pcc.RPCURLs, ",")
	for i := range urls {
		urls[i] = strings.TrimSpace(urls[i])
		if urls[i] == "" {
			return nil, fmt.Errorf("RPC URL cannot be empty")
		}
		if !strings.HasPrefix(urls[i], "http://") && !strings.HasPrefix(urls[i], "https://") {
			return nil, fmt.Errorf("invalid RPC URL: %s", urls[i])
		}
		if !strings.Contains(urls[i], "://") {
			return nil, fmt.Errorf("invalid RPC URL format: %s", urls[i])
		}
		urls[i] = strings.TrimSuffix(urls[i], "/") // Remove trailing slash if present
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("no valid RPC URLs provided")
	}
	return urls, nil
}

// NewPoolClient creates a new EthClientPool instance.
func NewPoolClient(cfg PoolClientConfig) (*PoolClient, error) {
	urls, err := cfg.RPCURLsSlice()
	if err != nil {
		return nil, fmt.Errorf("invalid RPC URLs: %w", err)
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("at least one RPC URL is required")
	}

	clients := make([]*ethclient.Client, 0, len(cfg.RPCURLs))
	for _, url := range urls {
		var client *ethclient.Client
		var err error
		if cfg.ctx != nil {
			client, err = ethclient.DialContext(cfg.ctx, url)
		} else {
			client, err = ethclient.Dial(url)
		}
		if err != nil {
			for _, c := range clients {
				c.Close()
			}
			return nil, fmt.Errorf("failed to connect to %s: %w", url, err)
		}
		clients = append(clients, client)
	}

	return &PoolClient{
		clients:    clients,
		maxRetries: cfg.MaxRetries,
		backoff:    cfg.Backoff,
	}, nil
}

// Close closes all underlying clients.
func (pc *PoolClient) Close() {
	for _, client := range pc.clients {
		client.Close()
	}
}

// retry executes a function across clients with retry logic.
func (pc *PoolClient) retry(fn func(client *ethclient.Client) ([]interface{}, error), expectedResults int) ([]interface{}, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	indices := rand.Perm(len(pc.clients))
	for attempt := 0; attempt <= pc.maxRetries; attempt++ {
		for _, idx := range indices {
			client := pc.clients[idx]
			results, err := fn(client)
			if err == nil {
				if expectedResults > 0 && len(results) != expectedResults {
					return nil, fmt.Errorf("expected %d results, got %d", expectedResults, len(results))
				}
				return results, nil
			}
			fmt.Printf("Attempt %d/%d failed for client %d: %v\n", attempt+1, pc.maxRetries, idx, err)
			continue
		}
		time.Sleep(pc.backoff)
	}
	return nil, fmt.Errorf("all clients failed after %d retries", pc.maxRetries)
}

func Dial(rawurl string) (*PoolClient, error) {
	cfg := PoolClientConfig{
		RPCURLs:    rawurl,
		MaxRetries: 3,
		Backoff:    100 * time.Millisecond,
	}
	return NewPoolClient(cfg)
}

// DialContext creates a new PoolClient with a context for dialing.
func DialContext(ctx context.Context, rawurl string) (*PoolClient, error) {
	cfg := PoolClientConfig{
		RPCURLs:    rawurl,
		MaxRetries: 3,
		Backoff:    100 * time.Millisecond,
		ctx:        ctx,
	}
	return NewPoolClient(cfg)
}
