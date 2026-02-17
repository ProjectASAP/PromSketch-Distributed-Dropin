go test -v  -count=2  -timeout 0 -run TestInsertThroughputZipf ./ -numts=1 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputZipf ./ -numts=100 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputZipf ./ -numts=1000 -numthreads=64
go test -v  -count=2  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=64