#!/bin/bash
# NebulaGraph single-node bootstrap for benchmark
#
# Starts metad + storaged + graphd in a single container.
# nebula-console has all three binaries; nebula-entrypoint.sh picks the right one.
# For single-node bench we run all three roles.
#
# Default ports:
#   metad:  9559 (thrift), 19559 (HTTP), 19560 (metrics)
#   storaged: 9560 (thrift), 19600 (HTTP), 19601 (metrics)
#   graphd:   9669 (thrift), 19669 (HTTP)
#
# Benchmark client connects to graphd on 9669.

exec /usr/local/nebula/bin/nebula-entrypoint.sh \
    --meta_server_addrs=127.0.0.1:9559 \
    --local_config=true \
    --cluster_id=1 \
    nebula-graphd
