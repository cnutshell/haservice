## Introduction

Fundamental facility to keep service high available.

## Architecture

Each node in the cluster joins gossip network.

In advance, we select 3 nodes as a central group via configuration file.

The 3 selected nodes compose a raft group, and the registered service would be running on raft leader.

## How to use agent

```bash
./agent -node-id=1 \
        -raft-addr=127.0.0.1:9001 \
        -raft-data-dir=/tmp/raft/1 \
        -serf-addr=127.0.0.1:10001 \
        -serf-cluster=127.0.0.1:10001,127.0.0.1:10002,127.0.0.1:10003

./agent -node-id=2 \
        -raft-addr=127.0.0.1:9002 \
        -raft-data-dir=/tmp/raft/2 \
        -serf-addr=127.0.0.1:10002 \
        -serf-cluster=127.0.0.1:10001,127.0.0.1:10002,127.0.0.1:10003

./agent -node-id=3 \
        -raft-addr=127.0.0.1:9003 \
        -raft-data-dir=/tmp/raft/3 \
        -serf-addr=127.0.0.1:10003 \
        -serf-cluster=127.0.0.1:10001,127.0.0.1:10002,127.0.0.1:10003
```
