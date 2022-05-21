## Introduction

Fundamental facility to keep service high available.

## Architecture

Each node in the cluster joins gossip network.

In advance, we select 3 nodes as a central group via configuration file.

The 3 selected nodes compose a raft group, and the registered service would be running on raft leader.

