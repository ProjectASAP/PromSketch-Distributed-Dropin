#!/bin/bash
/usr/local/go/bin/go test -timeout 0  -run TestSmoothHistogramCount -v SmoothHistogram_test.go SmoothHistogram.go UnivMon.go utils.go CountMinSketch.go CountSketch.go heap.go  > ./microbenchmark_results/shcount.txt
