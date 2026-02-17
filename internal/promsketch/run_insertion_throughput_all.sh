# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=10000 > insertion_throughput/dynamic_sampling_10e4.txt     
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=10000 >> insertion_throughput/dynamic_sampling_10e4.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=10000 >> insertion_throughput/dynamic_sampling_10e4.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=10000 >> insertion_throughput/dynamic_sampling_10e4.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=10000 >> insertion_throughput/dynamic_sampling_10e4.txt  

# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=100000 > insertion_throughput/dynamic_sampling_10e5.txt     
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=100000 >> insertion_throughput/dynamic_sampling_10e5.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=100000 >> insertion_throughput/dynamic_sampling_10e5.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=100000 >> insertion_throughput/dynamic_sampling_10e5.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=100000 >> insertion_throughput/dynamic_sampling_10e5.txt  

# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=1000000 > insertion_throughput/dynamic_sampling_10e6.txt     
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=1000000 >> insertion_throughput/dynamic_sampling_10e6.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=1000000 >> insertion_throughput/dynamic_sampling_10e6.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=1000000 >> insertion_throughput/dynamic_sampling_10e6.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=1000000 >> insertion_throughput/dynamic_sampling_10e6.txt 



# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=10000 > insertion_throughput/dynamic_ehkll_10e4.txt     
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=10000 >> insertion_throughput/dynamic_ehkll_10e4.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=10000 >> insertion_throughput/dynamic_ehkll_10e4.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=10000 >> insertion_throughput/dynamic_ehkll_10e4.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=10000 >> insertion_throughput/dynamic_ehkll_10e4.txt  

# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=100000 > insertion_throughput/dynamic_ehkll_10e5.txt     
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=100000 >> insertion_throughput/dynamic_ehkll_10e5.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=100000 >> insertion_throughput/dynamic_ehkll_10e5.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=100000 >> insertion_throughput/dynamic_ehkll_10e5.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=100000 >> insertion_throughput/dynamic_ehkll_10e5.txt  

# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=1000000 > insertion_throughput/dynamic_ehkll_10e6.txt     
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=1000000 >> insertion_throughput/dynamic_ehkll_10e6.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=1000000 >> insertion_throughput/dynamic_ehkll_10e6.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=1000000 >> insertion_throughput/dynamic_ehkll_10e6.txt  
# go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=1000000 >> insertion_throughput/dynamic_ehkll_10e6.txt 



go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=10000 > insertion_throughput/dynamic_ehuniv_10e4.txt     
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=10000 >> insertion_throughput/dynamic_ehuniv_10e4.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=10000 >> insertion_throughput/dynamic_ehuniv_10e4.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=10000 >> insertion_throughput/dynamic_ehuniv_10e4.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=10000 >> insertion_throughput/dynamic_ehuniv_10e4.txt  

go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=100000 > insertion_throughput/dynamic_ehuniv_10e5.txt     
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=100000 >> insertion_throughput/dynamic_ehuniv_10e5.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=100000 >> insertion_throughput/dynamic_ehuniv_10e5.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=100000 >> insertion_throughput/dynamic_ehuniv_10e5.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=100000 >> insertion_throughput/dynamic_ehuniv_10e5.txt  

go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=1000000 > insertion_throughput/dynamic_ehuniv_10e6.txt     
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=1000000 >> insertion_throughput/dynamic_ehuniv_10e6.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=1000000 >> insertion_throughput/dynamic_ehuniv_10e6.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=1000000 >> insertion_throughput/dynamic_ehuniv_10e6.txt  
go test -v  -count=2 -timeout 0 -run TestInsertThroughputDynamic ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=1000000 >> insertion_throughput/dynamic_ehuniv_10e6.txt 