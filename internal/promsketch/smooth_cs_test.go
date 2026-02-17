package promsketch

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// Test cost (compute + memory) and accuracy under sliding window
func TestSmoothHistogramCSCAIDA(t *testing.T) {
	cost_query_interval_gsum := int64(2000)

	query_window_size := int64(10000)
	total_length := int64(200000)

	// Create a scenario
	t1 := make([]int64, 0)
	t2 := make([]int64, 0)
	t1 = append(t1, int64(0))
	t2 = append(t2, query_window_size-1)

	t1 = append(t1, int64(query_window_size/3))
	t2 = append(t2, int64(query_window_size/3)*2)

	/*
		// suffix length
		for i := int64(500); i <= int64(1000); i += 100 {
			t1 = append(t1, query_window_size-i)
			t2 = append(t2, query_window_size-1)
		}
	*/

	fmt.Println("t1:", t1)
	fmt.Println("t2:", t2)

	readCAIDA()
	fmt.Println("Finished reading input timeseries.")

	for test_case := 0; test_case < 5; test_case += 1 {
		filename := "cost_analysis_results/caida_gsum_sampling_SHCS_larger" + strconv.Itoa(test_case) + ".txt"
		fmt.Println(filename)
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w := bufio.NewWriter(f)

		// PromSketch, SHCS
		beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
		// beta_input := []float64{0.3535}
		for _, beta := range beta_input {
			fmt.Println("SHCS", beta)
			fmt.Fprintln(w, "SHCS", beta)

			sh := SmoothInitCS(beta, query_window_size)

			total_compute := 0.0
			insert_compute := 0.0
			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				sh.Update(t, strconv.FormatFloat(cases[0].vec[t].F, 'f', -1, 64), 1)
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())

				if t == total_length-1 || (t >= query_window_size-1 && (t+1)%cost_query_interval_gsum == 0) {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						merged_cs, _ := sh.QueryIntervalMergeCS(t-end_t, t-start_t, t)

						l1 := merged_cs.cs_l1()
						l2 := merged_cs.cs_l2()

						elapsed := time.Since(start)
						total_compute += float64(elapsed.Microseconds())

						fmt.Println("start_t, end_t:", start_t, end_t)
						fmt.Println("estimate:", l1, l2)
						// fmt.Fprintln(w, t, j, distinct, l1, entropy, l2)

						values := make([]float64, 0)
						for tt := start_t; tt < end_t; tt++ {
							values = append(values, float64(cases[0].vec[tt].F))
						}
						gt_distinct, gt_l1, gt_entropy, gt_l2 := gsum(values)

						fmt.Println("true:", gt_distinct, gt_l1, gt_entropy, gt_l2)

						l1_err := AbsFloat64(gt_l1-l1) / gt_l1 * 100
						l2_err := AbsFloat64(gt_l2-l2) / gt_l2 * 100
						// fmt.Fprintln(w, t, j, "errors:", distinct_err, l1_err, entropy_err, l2_err)
						fmt.Println(t, j, "errors:", l1_err, l2_err)
						fmt.Println()
						w.Flush()
						// distinct_rel_err := AbsFloat64(ground_truth[t][j].distinct-distinct) / (ground_truth[t][j].distinct) * 100

						// l1_rel_err := AbsFloat64(ground_truth[t][j].l1-l1) / (ground_truth[t][j].l1) * 100

						// entropy_rel_err := AbsFloat64(ground_truth[t][j].entropy-entropy) / (ground_truth[t][j].entropy) * 100

						// l2_rel_err := AbsFloat64(ground_truth[t][j].l2-l2) / (ground_truth[t][j].l2) * 100

						// fmt.Fprintln(w,t, j, distinct_rel_err, l1_rel_err, entropy_rel_err, l2_rel_err)
					}
				}
			}
			// fmt.Fprintln(w,"distinct error:", sh_distinct_error)
			// fmt.Fprintln(w,"l1 error:", sh_l1_error)
			// fmt.Fprintln(w,"entropy error:", sh_entropy_error)
			// fmt.Fprintln(w,"l2 error:", sh_l2_error)

			fmt.Println("insert compute:", insert_compute)
			fmt.Println("query compute:", total_compute, "us")
			fmt.Println("total compute:", total_compute+insert_compute, "us")
			fmt.Println("memory:", sh.GetMemory(), "KB")

			fmt.Fprintln(w, "insert compute:", insert_compute)
			fmt.Fprintln(w, "query compute:", total_compute, "us")
			fmt.Fprintln(w, "total compute:", total_compute+insert_compute, "us")
			fmt.Fprintln(w, "memory:", sh.GetMemory(), "KB")
		}
	}
}

func TestSmoothHistogramUnivMonCSUpdateTime(t *testing.T) {

	query_window_size := int64(100000)
	total_length := int64(2000000)

	// Create a scenario
	t1 := make([]int64, 0)
	t2 := make([]int64, 0)
	t1 = append(t1, int64(0))
	t2 = append(t2, query_window_size-1)

	t1 = append(t1, int64(query_window_size/3))
	t2 = append(t2, int64(query_window_size/3)*2)

	/*
		// suffix length
		for i := int64(500); i <= int64(1000); i += 100 {
			t1 = append(t1, query_window_size-i)
			t2 = append(t2, query_window_size-1)
		}
	*/

	fmt.Println("t1:", t1)
	fmt.Println("t2:", t2)

	readCAIDA()
	fmt.Println("Finished reading input timeseries.")

	for test_case := 0; test_case < 5; test_case += 1 {
		filename := "cost_analysis_results/caida_gsum_sampling_SHCS_larger" + strconv.Itoa(test_case) + ".txt"
		fmt.Println(filename)
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w := bufio.NewWriter(f)

		// PromSketch, SHCS
		// beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
		beta_input := []float64{0.3535}
		for _, beta := range beta_input {
			fmt.Println("SHCS", beta)
			fmt.Fprintln(w, "SHCS", beta)

			sh := SmoothInitCS(beta, query_window_size)

			insert_compute := 0.0
			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				sh.Update(t, strconv.FormatFloat(cases[0].vec[t].F, 'f', -1, 64), 1)
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())
				fmt.Println("insert time per item:", insert_compute/float64(t+1), "us")
				fmt.Println("s_count:", sh.s_count)
			}

			fmt.Println("insert time per item:", insert_compute/float64(total_length), "us")
			fmt.Println("s_count:", sh.s_count)
			fmt.Println("memory:", sh.GetMemory(), "KB")

		}
	}
}
