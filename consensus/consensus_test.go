package consensus

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
)

var minPort int32 = 10000

// TODO: update in order to avoid port conflict
func getPort() int {
	return int(atomic.AddInt32(&minPort, 1))
}

func TestInitialized(t *testing.T) {
	nodeCount := 3
	nodes := make([]*RaftNode, 0, nodeCount)

	for i := 0; i < nodeCount; i++ {
		bindAddr := fmt.Sprintf("127.0.0.1:%d", getPort())
		// Keep initialized data
		dataDir := filepath.Join("/tmp/test", fmt.Sprintf("%d", i))

		node, err := newTestNode(t, i, bindAddr, dataDir)
		require.NoError(t, err)
		nodes = append(nodes, node)

		go func(index int) {
			for {
				leader, ok := <-node.LeaderCh()
				if ok {
					t.Logf("=====> node %d leader change %t", index, leader)
				}
			}
		}(i)
	}

	selected := len(nodes) - 1
	// if not initialized, just bootstrap it
	if !nodes[selected].HasExistingState() {
		err := nodes[selected].WaitForLeader(3 * time.Second)
		require.NoError(t, err)

		for i := 0; i < selected; i++ {
			err = nodes[selected].Join(
				fmt.Sprintf("%d", i), nodes[i].LocalAddr().String(),
			)
			require.NoError(t, err)
		}
	}

	// check cluster ready or not
	for i := 0; i < nodeCount; i++ {
		err := nodes[i].WaitForLeader(3 * time.Second)
		require.NoError(t, err)
	}
}

func TestBootstrap(t *testing.T) {
	nodeCount := 3
	nodes := make([]*RaftNode, 0, nodeCount)

	for i := 0; i < nodeCount; i++ {
		bindAddr := fmt.Sprintf("127.0.0.1:%d", getPort())

		dataDir, err := ioutil.TempDir("", "test")
		require.NoError(t, err)
		defer func(dir string) {
			_ = os.RemoveAll(dir)
		}(dataDir)

		node, err := newTestNode(t, i, bindAddr, dataDir)
		require.NoError(t, err)
		defer func() {
			_ = node.Shutdown()
		}()
		nodes = append(nodes, node)
		go func(index int) {
			for {
				leader, ok := <-node.LeaderCh()
				if ok {
					t.Logf("=====> node %d leader change %t", index, leader)
				}
			}
		}(i)
	}

	// select the last node as leader
	selected := len(nodes) - 1
	err := nodes[selected].WaitForLeader(3 * time.Second)
	require.NoError(t, err)

	// other node join to the selected leader
	for i := 0; i < selected; i++ {
		err = nodes[selected].Join(
			fmt.Sprintf("%d", i), nodes[i].LocalAddr().String(),
		)
		require.NoError(t, err)
	}

	// check cluster ready or not
	for i := 0; i < nodeCount; i++ {
		err := nodes[i].WaitForLeader(3 * time.Second)
		require.NoError(t, err)
	}
}

func newTestNode(
	t *testing.T, id int, bindAddr, dataDir string,
) (*RaftNode, error) {
	ln, err := net.Listen("tcp", bindAddr)
	require.NoError(t, err)

	config := Config{}
	config.Raft.StreamLayer = NewStreamLayer(ln)
	config.Raft.LocalID = raft.ServerID(fmt.Sprintf("%d", id))
	config.Raft.HeartbeatTimeout = 500 * time.Millisecond
	config.Raft.ElectionTimeout = 500 * time.Millisecond
	config.Raft.LeaderLeaseTimeout = 500 * time.Millisecond
	config.Raft.CommitTimeout = 5 * time.Millisecond
	// just enable Bootstrap, we could check initialized or not
	config.Raft.Bootstrap = true

	node, err := NewRaftNode(dataDir, config)
	require.NoError(t, err)

	return node, nil
}
