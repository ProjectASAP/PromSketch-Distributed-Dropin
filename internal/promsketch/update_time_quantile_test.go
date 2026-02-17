package promsketch

import (
	"fmt"
	"testing"
	"time"
)

// Test update time per item under different memory configuration and sliding window sizes
func TestUpdateTimeQuantile(t *testing.T) {
	readGoogleClusterData2009()
	total_length := int64(2000000)
	sliding_window_sizes := []int64{10000, 100000, 1000000, 10000000}
	for _, query_window_size := range sliding_window_sizes {
		if query_window_size > total_length {
			break
		}
		fmt.Println("sliding window size:", query_window_size)
		k_input := []int64{10, 20, 50, 100, 200, 500, 1000}
		kllk_input := []int{64, 128, 256, 512, 1024}
		for _, k := range k_input {
			for _, kll_k := range kllk_input {
				fmt.Println("EHKLL", k, kll_k)
				ehkll := ExpoInitKLL(k, kll_k, query_window_size)

				insert_compute := 0.0
				for t := int64(0); t < total_length; t++ {
					start := time.Now()
					ehkll.Update(t, cases[0].vec[t].F)
					elapsed := time.Since(start)
					insert_compute += float64(elapsed.Microseconds())
				}
				update_time := float64(insert_compute) / float64(total_length)
				fmt.Println("insert compute:", insert_compute, "us")
				fmt.Println("update time per item:", update_time, "us")
				fmt.Println("memory:", ehkll.GetMemory(), "KB")
			}
		}
	}
}
