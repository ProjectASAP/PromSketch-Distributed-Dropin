package client

import (
	"fmt"
	"log"
	"sync"

	pb "github.com/promsketch/promsketch-dropin/api/psksketch/v1"
	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Pool manages gRPC connections to psksketch nodes
type Pool struct {
	clients map[string]pb.SketchServiceClient
	conns   map[string]*grpc.ClientConn
	mu      sync.RWMutex
}

// NewPool creates a new client pool and connects to all nodes
func NewPool(nodes []*hash.Node) (*Pool, error) {
	p := &Pool{
		clients: make(map[string]pb.SketchServiceClient, len(nodes)),
		conns:   make(map[string]*grpc.ClientConn, len(nodes)),
	}

	for _, node := range nodes {
		conn, err := grpc.NewClient(
			node.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(10*1024*1024),
				grpc.MaxCallSendMsgSize(10*1024*1024),
			),
		)
		if err != nil {
			// Close already established connections
			p.Close()
			return nil, fmt.Errorf("failed to connect to %s (%s): %w", node.ID, node.Address, err)
		}

		p.conns[node.ID] = conn
		p.clients[node.ID] = pb.NewSketchServiceClient(conn)
		log.Printf("Connected to psksketch node %s at %s", node.ID, node.Address)
	}

	return p, nil
}

// GetClient returns the gRPC client for a specific node
func (p *Pool) GetClient(nodeID string) (pb.SketchServiceClient, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	client, ok := p.clients[nodeID]
	return client, ok
}

// AllClients returns all clients
func (p *Pool) AllClients() map[string]pb.SketchServiceClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[string]pb.SketchServiceClient, len(p.clients))
	for k, v := range p.clients {
		result[k] = v
	}
	return result
}

// Close closes all gRPC connections
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, conn := range p.conns {
		if err := conn.Close(); err != nil {
			log.Printf("Error closing connection to %s: %v", id, err)
		}
	}
	p.clients = make(map[string]pb.SketchServiceClient)
	p.conns = make(map[string]*grpc.ClientConn)
}
