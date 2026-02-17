package promsketch

import (
	"bufio"
	"fmt"
	"os"
	"testing"
	"time"
)

const cost_query_interval_gsum int64 = 200000

func prometheus_gsum_offline(w *bufio.Writer, total_length int64, time_window_size int64, t1, t2 []int64, ground_truth *([][]gsum_ans)) (float64, float64, float64) {
	input_values := make([]float64, 0)
	insert_compute := 0.0
	query_compute := 0.0
	for t := int64(0); t < total_length; t++ {
		start := time.Now()
		input_values = append(input_values, cases[0].vec[t].F)
		elapsed := time.Since(start)
		insert_compute += float64(elapsed.Microseconds())

		// evaluate scenarios
		if t == total_length-1 {
			for i := range len(t1) {
				start_t := t1[i] + t - time_window_size + 1
				end_t := t2[i] + t - time_window_size + 1
				start = time.Now()
				values := make([]float64, 0)
				for t := start_t; t <= end_t; t++ {
					values = append(values, cases[0].vec[t].F)
				}
				distinct, l1, entropy, l2 := gsum(values)
				elapsed = time.Since(start)
				query_compute += float64(elapsed.Microseconds())
				fmt.Fprintln(w, t, i, distinct, l1, entropy, l2)
				// (*ground_truth)[t][i] = gsum_ans{distinct: distinct, l1: l1, entropy: entropy, l2: l2}
				w.Flush()
			}
		} else {
			if t >= time_window_size-1 && (t+1)%cost_query_interval == 0 {
				for i := range len(t1) {
					start_t := t1[i] + t - time_window_size + 1
					end_t := t2[i] + t - time_window_size + 1
					start = time.Now()
					values := make([]float64, 0)
					for t := start_t; t <= end_t; t++ {
						values = append(values, cases[0].vec[t].F)
					}
					distinct, l1, entropy, l2 := gsum(values)
					elapsed = time.Since(start)
					query_compute += float64(elapsed.Microseconds())
					fmt.Fprintln(w, t, i, distinct, l1, entropy, l2)
					// (*ground_truth)[t][i] = gsum_ans{distinct: distinct, l1: l1, entropy: entropy, l2: l2}
					w.Flush()
				}
			}
		}
	}
	return insert_compute, query_compute * 4, float64(time_window_size) * 8 / 1024 * 4
}

// Test cost (compute + memory) and accuracy under sliding window
func TestCostAnalysisGSumOffline(t *testing.T) {
	constructInputTimeSeriesOutputZipf()
	fmt.Println("Finished reading input timeseries")
	query_window_size := int64(1000000)
	total_length := int64(10000000)

	// Create a scenario
	// Query these subwindows every 1000 data sample insertions
	t1 := make([]int64, 0)
	t2 := make([]int64, 0)
	t1 = append(t1, int64(0))
	t2 = append(t2, query_window_size-1)

	t1 = append(t1, int64(query_window_size/3))
	t2 = append(t2, int64(query_window_size/3)*2)

	// suffix length
	for i := int64(100000); i <= int64(1000000); i += 10000 {
		t1 = append(t1, query_window_size-i)
		t2 = append(t2, query_window_size-1)
	}

	/*
		start_t := t1[len(t1)-1]

		for i := 1; i <= 10; i++ {
			t1 = append(t1, start_t+query_window_size/10/10*int64(i-1))
			t2 = append(t2, start_t+query_window_size/10/10*int64(i)-1)
		}

		start_t = t1[len(t1)-1]
		for i := 1; i <= 10; i++ {
			t1 = append(t1, start_t+query_window_size/10/10/10*int64(i-1))
			t2 = append(t2, start_t+query_window_size/10/10/10*int64(i)-1)
		}
	*/

	f, err := os.OpenFile("./testdata/zipf_gsum_groundtruth_larger.txt", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "t1: ")
	for i := range len(t1) {
		fmt.Fprintf(w, "%d ", t1[i])
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "t2: ")
	for i := range len(t2) {
		fmt.Fprintf(w, "%d ", t2[i])
	}
	fmt.Fprintln(w)

	ground_truth := make([][]gsum_ans, total_length)
	for t := 0; t < int(total_length); t++ {
		ground_truth[t] = make([]gsum_ans, len(t1))
	}

	// Prometheus baseline
	insert_compute, query_compute, memory := prometheus_gsum_offline(w, total_length, query_window_size, t1, t2, &ground_truth)
	fmt.Fprintln(w, "Prometheus")
	fmt.Fprintf(w, "insert compute: %f us", insert_compute)
	fmt.Fprintf(w, "query compute: %f us", query_compute)
	fmt.Fprintf(w, "total compute: %f us", query_compute+insert_compute)
	fmt.Fprintf(w, "memory: %f KB", memory)

	/*
		for t := int64(0); t < total_length; t++ {
			if t == total_length-1 || (t >= query_window_size-1 && (t+1)%cost_query_interval == 0) {
				for i := range len(t1) {
					fmt.Fprintf(w, "%d %d %f %f %f %f", t, i, ground_truth[t][i].distinct, ground_truth[t][i].l1, ground_truth[t][i].entropy, ground_truth[t][i].l2)
					fmt.Fprintln(w)
				}
			}
			w.Flush()
		}
	*/
	w.Flush()
}
