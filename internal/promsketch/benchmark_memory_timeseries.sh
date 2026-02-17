go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64