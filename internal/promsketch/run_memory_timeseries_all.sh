go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_sampling_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e4_zipf.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_sampling_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e5_zipf.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_sampling_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_sampling_10e6_zipf.txt



go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_ehkll_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_zipf.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_ehkll_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_zipf.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_ehkll_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_zipf.txt


go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_ehuniv_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_zipf.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_ehuniv_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_zipf.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Zipf" > memory_timeseries_num/memory_timeseries_ehuniv_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_zipf.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Zipf" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_zipf.txt


go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_sampling_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e4_dynamic.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_sampling_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e5_dynamic.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_sampling_10e6_dynamic.txt



go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehkll_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_dynamic.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehkll_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_dynamic.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_dynamic.txt


go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehuniv_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_dynamic.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehuniv_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_dynamic.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" > memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Dynamic" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_dynamic.txt


go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Google" > memory_timeseries_num/memory_timeseries_sampling_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e4_google.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Google" > memory_timeseries_num/memory_timeseries_sampling_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e5_google.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Google" > memory_timeseries_num/memory_timeseries_sampling_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=sampling -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_sampling_10e6_google.txt



go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Google" > memory_timeseries_num/memory_timeseries_ehkll_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e4_google.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Google" > memory_timeseries_num/memory_timeseries_ehkll_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e5_google.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Google" > memory_timeseries_num/memory_timeseries_ehkll_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehkll -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehkll_10e6_google.txt


go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Google" > memory_timeseries_num/memory_timeseries_ehuniv_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=10000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e4_google.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Google" > memory_timeseries_num/memory_timeseries_ehuniv_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=100000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e5_google.txt

go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Google" > memory_timeseries_num/memory_timeseries_ehuniv_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=100 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=1000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_google.txt
go test -v -timeout 0 -run TestIndexingMemory ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=1000000 -dataset="Google" >> memory_timeseries_num/memory_timeseries_ehuniv_10e6_google.txt
