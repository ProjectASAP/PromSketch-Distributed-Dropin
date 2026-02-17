package promsketch

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

// Test update time per item under different memory configuration and sliding window sizes
func TestUpdateTimGSum(t *testing.T) {
	readCAIDA()
	total_length := int64(7000000)
	sliding_window_sizes := []int64{10000, 100000, 1000000, 7000000}
	for _, query_window_size := range sliding_window_sizes {
		if query_window_size > total_length {
			break
		}
		fmt.Println("sliding window size:", query_window_size)
		// PromSketch, SHUniv
		beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
		for _, beta := range beta_input {
			fmt.Println("SHUniv", beta)

			shu := SmoothInitUnivMon(beta, query_window_size)

			insert_compute := 0.0
			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				shu.Update(t, strconv.FormatFloat(cases[0].vec[t].F, 'f', -1, 64))
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())
			}
			update_time := float64(insert_compute) / float64(total_length)
			fmt.Println("insert compute:", insert_compute, "us")
			fmt.Println("update time per item:", update_time, "us")
			fmt.Println("memory:", shu.GetMemory(), "KB")
		}

	}
}
