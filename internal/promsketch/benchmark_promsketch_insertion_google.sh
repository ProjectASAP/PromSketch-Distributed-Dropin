go test -v  -count=2  -timeout 0 -run TestInsertThroughputGoogle ./ -numts=1 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputGoogle ./ -numts=10 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputGoogle ./ -numts=100 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputGoogle ./ -numts=1000 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputGoogle ./ -numts=10000 -numthreads=64