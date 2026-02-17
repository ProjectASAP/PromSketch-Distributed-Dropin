package promsketch

import (
	"fmt"
	"testing"
	"time"
)

// Test update time per item under different memory configuration and sliding window sizes
func TestUpdateTimeSampling(t *testing.T) {
	readGoogleClusterData2009()
	total_length := int64(2000000)
	sliding_window_sizes := []int64{10000, 100000, 1000000, 10000000}
	for _, query_window_size := range sliding_window_sizes {
		if query_window_size > total_length {
			break
		}
		fmt.Println("sliding window size:", query_window_size)
		sampling_rate := []float64{0.001, 0.01, 0.05, 0.1, 0.2}
		for _, rate := range sampling_rate {
			fmt.Println("sampling", rate)

			insert_compute := 0.0
			sampling_size := int(float64(query_window_size) * rate)
			sampling_instance := NewUniformSampling(query_window_size, rate, sampling_size)
			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				sampling_instance.Insert(t, cases[0].vec[t].F)
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())
			}
			update_time := float64(insert_compute) / float64(total_length)
			fmt.Println("insert compute:", insert_compute, "us")
			fmt.Println("update time per item:", update_time, "us")
			fmt.Println("memory:", sampling_instance.GetMemory(), "KB")
		}
	}
}
