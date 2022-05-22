package group

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
)

var minPort int32 = 10000

func getPort() int {
	return int(atomic.AddInt32(&minPort, 1))
}

func TestMultipleNodes(t *testing.T) {
	nodeCount := 3
	nodes := make([]*DistNode, 0, nodeCount)

	for i := 0; i < nodeCount; i++ {
		dataDir, err := ioutil.TempDir("", "test")
		require.NoError(t, err)
		defer func(dir string) {
			_ = os.RemoveAll(dir)
		}(dataDir)

		ln, err := net.Listen(
			"tcp",
			fmt.Sprintf(":%d", getPort()),
		)
		config := Config{}
		config.Raft.StreamLayer = NewStreamLayer(ln)
		config.Raft.LocalID = raft.ServerID(fmt.Sprintf("%d", i))
		config.Raft.HeartbeatTimeout = 500 * time.Millisecond
		config.Raft.ElectionTimeout = 500 * time.Millisecond
		config.Raft.LeaderLeaseTimeout = 500 * time.Millisecond
		config.Raft.CommitTimeout = 5 * time.Millisecond
		if i == 0 {
			config.Raft.Bootstrap = true
		}

		node, err := NewDistNode(dataDir, config)
		require.NoError(t, err)
		if node.HasExistingState() {
			// no need to join
		} else {
			if i == 0 {
				err = node.WaitForLeader(3 * time.Second)
				require.NoError(t, err)
			} else {
				err = nodes[0].Join(
					fmt.Sprintf("%d", i), ln.Addr().String(),
				)
				require.NoError(t, err)
			}
		}

		nodes = append(nodes, node)
	}

	for i := 0; i < nodeCount; i++ {
		err := nodes[i].WaitForLeader(3 * time.Second)
		require.NoError(t, err)
	}
}
