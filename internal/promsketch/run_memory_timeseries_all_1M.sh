

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt


go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt


go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt

