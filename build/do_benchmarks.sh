#!/usr/bin/env bash

set -e
set -x

go install golang.org/x/tools/cmd/goimports@latest
go get -u github.com/google/pprof

cd /go-control-plane

NUM_DIFF_LINES=`/go/bin/goimports -local github.com/envoyproxy/go-control-plane -d pkg | wc -l`
if [[ $NUM_DIFF_LINES > 0 ]]; then
  echo "Failed format check. Run make format"
  exit 1
fi

make benchmark MODE=0
make benchmark MODE=1
make benchmark MODE=2
make benchmark MODE=3
make benchmark MODE=4

# generate our consumable pprof profiles
# pprof -text bin/test goroutine_profile_xds.pb.gz > benchmarks/goroutine/goroutine_text.txt
# pprof -tree bin/test goroutine_profile_xds.pb.gz > benchmarks/goroutine/goroutine_tree.txt
# pprof -traces bin/test goroutine_profile_xds.pb.gz > benchmarks/goroutine/goroutine_trace.txt

# pprof -text bin/test block_profile_xds.pb.gz > benchmarks/block/block_text.txt
# pprof -tree bin/test block_profile_xds.pb.gz > benchmarks/block/block_tree.txt
# pprof -traces bin/test block_profile_xds.pb.gz > benchmarks/block/block_trace.txt

# pprof -text bin/test mutex_profile_xds.pb.gz > benchmarks/mutex/mutex_text.txt
# pprof -tree bin/test mutex_profile_xds.pb.gz > benchmarks/mutex/mutex_tree.txt
# pprof -traces bin/test mutex_profile_xds.pb.gz > benchmarks/mutex/mutex_trace.txt
