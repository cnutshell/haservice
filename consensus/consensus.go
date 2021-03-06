package consensus

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/serf"
)

const INITIALIZED int32 = 1

type RaftNode struct {
	config      Config
	raft        *raft.Raft
	initialized int32
}

func NewRaftNode(dataDir string, config Config) (*RaftNode, error) {
	g := &RaftNode{
		config: config,
	}
	if err := g.setupRaft(dataDir); err != nil {
		return nil, err
	}
	return g, nil
}

func (n *RaftNode) OnJoin(
	id, addr string, lables map[string]string,
) error {
	// if initialized, no need to join
	if n.HasExistingState() {
		return nil
	}

	// wait for leader
	err := n.WaitForLeader(3 * time.Second)
	if err != nil {
		return err
	}

	err = n.Join(id, addr)
	if err == raft.ErrNotLeader {
		// only leader will success
		return nil
	}

	return err
}

func (n *RaftNode) OnUpdate(member serf.Member) error {
	return nil
}

func (n *RaftNode) OnLeave(member serf.Member) error {
	return nil
}

func (n *RaftNode) Shutdown() error {
	future := n.raft.Shutdown()
	if err := future.Error(); err != nil {
		return err
	}
	return nil
}

func (n *RaftNode) LocalAddr() net.Addr {
	return n.config.Raft.StreamLayer.ln.Addr()
}

func (n *RaftNode) ListServers() ([]raft.Server, error) {
	configFuture := n.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return nil, err
	}
	return configFuture.Configuration().Servers, nil
}

func (n *RaftNode) Leave(id string) error {
	removeFuture := n.raft.RemoveServer(raft.ServerID(id), 0, 0)
	return removeFuture.Error()
}

func (n *RaftNode) Join(id, addr string) error {
	servers, err := n.ListServers()
	if err != nil {
		return err
	}

	serverID := raft.ServerID(id)
	serverAddr := raft.ServerAddress(addr)
	for _, srv := range servers {
		if srv.ID == serverID || srv.Address == serverAddr {
			if srv.ID == serverID && srv.Address == serverAddr {
				// server has already joined
				return nil
			}
			// remove the existing server
			if err := n.RemoveServer(serverID, 0, 0); err != nil {
				return err
			}
		}
	}

	addFuture := n.raft.AddVoter(serverID, serverAddr, 0, 0)
	if err := addFuture.Error(); err != nil {
		return err
	}
	return nil
}

func (n *RaftNode) RemoveServer(
	id raft.ServerID, prevIndex uint64, timeout time.Duration,
) error {
	removeFuture := n.raft.RemoveServer(id, prevIndex, timeout)
	if err := removeFuture.Error(); err != nil {
		return err
	}
	return nil
}

func (n *RaftNode) LeaderCh() <-chan bool {
	return n.raft.LeaderCh()
}

func (n *RaftNode) State() raft.RaftState {
	return n.raft.State()
}

func (n *RaftNode) WaitForLeader(timeout time.Duration) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-time.After(timeout):
			return fmt.Errorf("timed out")
		case <-ticker.C:
			if n := n.raft.Leader(); n != "" {
				return nil
			}
		}
	}
}

func (n *RaftNode) HasExistingState() bool {
	return atomic.LoadInt32(&n.initialized) == INITIALIZED
}

func (n *RaftNode) setupRaft(dataDir string) error {
	fsm := &raft.MockFSM{}

	baseDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	// stable storage of key configurations to ensure safety
	stableStore, err := boltdb.NewBoltStore(
		filepath.Join(dataDir, "raft", "stable"),
	)
	if err != nil {
		return err
	}

	// storing and retrieving logs in a durable fashion
	logStore, err := boltdb.NewBoltStore(
		filepath.Join(dataDir, "raft", "log"),
	)
	if err != nil {
		return err
	}

	// snapshot storage and retrieval
	retain := 1
	snapshotStore, err := raft.NewFileSnapshotStore(
		baseDir,
		retain,
		os.Stderr,
	)
	if err != nil {
		return err
	}

	maxPool := 5
	timeout := 10 * time.Second
	transport := raft.NewNetworkTransport(
		n.config.Raft.StreamLayer,
		maxPool,
		timeout,
		os.Stderr,
	)

	config := raft.DefaultConfig()
	config.LocalID = n.config.Raft.LocalID
	if n.config.Raft.HeartbeatTimeout != 0 {
		config.HeartbeatTimeout = n.config.Raft.HeartbeatTimeout
	}
	if n.config.Raft.ElectionTimeout != 0 {
		config.ElectionTimeout = n.config.Raft.ElectionTimeout
	}
	if n.config.Raft.LeaderLeaseTimeout != 0 {
		config.LeaderLeaseTimeout = n.config.Raft.LeaderLeaseTimeout
	}
	if n.config.Raft.CommitTimeout != 0 {
		config.CommitTimeout = n.config.Raft.CommitTimeout
	}

	n.raft, err = raft.NewRaft(
		config,
		fsm,
		logStore,
		stableStore,
		snapshotStore,
		transport,
	)
	if err != nil {
		return err
	}

	hasState, err := raft.HasExistingState(
		logStore,
		stableStore,
		snapshotStore,
	)
	if err != nil {
		return err
	}

	if hasState {
		atomic.StoreInt32(&n.initialized, INITIALIZED)
	}

	if n.config.Raft.Bootstrap && !hasState {
		config := raft.Configuration{
			Servers: []raft.Server{{
				ID:      config.LocalID,
				Address: transport.LocalAddr(),
			}},
		}
		err = n.raft.BootstrapCluster(config).Error()
	}

	return err
}

type Config struct {
	Raft struct {
		raft.Config
		StreamLayer *StreamLayer
		Bootstrap   bool
	}
}

type StreamLayer struct {
	ln net.Listener
}

func NewStreamLayer(ln net.Listener) *StreamLayer {
	return &StreamLayer{
		ln: ln,
	}
}

func (s *StreamLayer) Dial(
	addr raft.ServerAddress,
	timeout time.Duration,
) (net.Conn, error) {

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", string(addr))
	if err != nil {
		return nil, err
	}
	return conn, err
}

func (s *StreamLayer) Accept() (net.Conn, error) {
	conn, err := s.ln.Accept()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (s *StreamLayer) Close() error {
	return s.ln.Close()
}

func (s *StreamLayer) Addr() net.Addr {
	return s.ln.Addr()
}
