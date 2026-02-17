

# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=100000 > insertion_scalability/zipf_ehuniv_10e5.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=32 -algo=ehuniv -sample_window=100000 >> insertion_scalability/zipf_ehuniv_10e5.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=16 -algo=ehuniv -sample_window=100000 >> insertion_scalability/zipf_ehuniv_10e5.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=8 -algo=ehuniv -sample_window=100000 >> insertion_scalability/zipf_ehuniv_10e5.txt  
go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=4 -algo=ehuniv -sample_window=100000 >> insertion_scalability/zipf_ehuniv_10e5.txt  
go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=4 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  

go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=2 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  
go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=2 -algo=ehuniv -sample_window=100000 >> insertion_scalability/zipf_ehuniv_10e5.txt  
go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=1 -algo=ehuniv -sample_window=100000 >> insertion_scalability/zipf_ehuniv_10e5.txt  

# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=1000000 > insertion_scalability/zipf_ehuniv_10e6.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=32 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=16 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=8 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=4 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=2 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  
go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=1 -algo=ehuniv -sample_window=1000000 >> insertion_scalability/zipf_ehuniv_10e6.txt  



# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=64 -algo=ehuniv -sample_window=10000 > insertion_scalability/zipf_ehuniv_10e4.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=32 -algo=ehuniv -sample_window=10000 >> insertion_scalability/zipf_ehuniv_10e4.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=16 -algo=ehuniv -sample_window=10000 >> insertion_scalability/zipf_ehuniv_10e4.txt  
# # go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=8 -algo=ehuniv -sample_window=10000 >> insertion_scalability/zipf_ehuniv_10e4.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=4 -algo=ehuniv -sample_window=10000 >> insertion_scalability/zipf_ehuniv_10e4.txt  
# go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=2 -algo=ehuniv -sample_window=10000 >> insertion_scalability/zipf_ehuniv_10e4.txt  
go test -v  -count=1  -timeout 0 -run TestInsertThroughputZipf ./ -numts=10000 -numthreads=1 -algo=ehuniv -sample_window=10000 >> insertion_scalability/zipf_ehuniv_10e4.txt  



