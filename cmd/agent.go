package main

import (
	"flag"
	"log"
	"net"
	"strings"
	"time"

	"github.com/cnutshell/haservice/consensus"
	"github.com/cnutshell/haservice/member"

	"github.com/hashicorp/raft"
)

var (
	serfAddr string
	raftAddr string
	raftData string
	id       string
	cluster  string
)

func init() {
	flag.StringVar(&serfAddr, "serf-addr", "", "serf address")
	flag.StringVar(&raftAddr, "raft-addr", "", "raft address")
	flag.StringVar(&raftData, "raft-data-dir", "", "raft data directory")
	flag.StringVar(&id, "node-id", "", "node id")
	flag.StringVar(&cluster, "serf-cluster", "", "comman seperated address")
}

func main() {
	flag.Parse()
	if serfAddr == "" {
		log.Fatal("empty serf address")
	}
	if raftAddr == "" {
		log.Fatal("empty raft address")
	}
	if raftData == "" {
		log.Fatal("empty data directory")
	}
	if id == "" {
		log.Fatal("empty id")
	}
	if cluster == "" {
		log.Fatal("empty cluster")
	}

	agent, err := NewAgent(id, raftAddr, raftData, serfAddr, cluster)
	if err != nil {
		log.Fatal("error:", err)
	}
	agent.Run()
}

type Agent struct {
	node   *consensus.RaftNode
	member *member.Member
}

func (a *Agent) Run() {
	ch := make(chan bool)
	<-ch
}

func NewAgent(id, raftAddr, raftData, serfAddr, cluster string) (*Agent, error) {
	node, err := newRaftNode(id, raftAddr, raftData)
	if err != nil {
		return nil, err
	}

	memberConf := member.Config{
		NodeName:  id,
		BindAddr:  serfAddr,
		RpcAddr:   raftAddr,
		JoinAddrs: strings.Split(cluster, ","),
	}

	member, err := member.NewMember(memberConf, node, nil)
	if err != nil {
		return nil, err
	}

	return &Agent{
		node:   node,
		member: member,
	}, nil
}

func newRaftNode(id, bindAddr, dataDir string) (*consensus.RaftNode, error) {
	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, err
	}

	config := consensus.Config{}
	config.Raft.StreamLayer = consensus.NewStreamLayer(ln)
	config.Raft.LocalID = raft.ServerID(id)
	config.Raft.HeartbeatTimeout = 500 * time.Millisecond
	config.Raft.ElectionTimeout = 500 * time.Millisecond
	config.Raft.LeaderLeaseTimeout = 500 * time.Millisecond
	config.Raft.CommitTimeout = 5 * time.Millisecond
	// just enable Bootstrap, we could check initialized or not
	config.Raft.Bootstrap = true

	node, err := consensus.NewRaftNode(dataDir, config)

	return node, nil
}
