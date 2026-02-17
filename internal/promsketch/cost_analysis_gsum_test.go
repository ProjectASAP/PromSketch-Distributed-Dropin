package promsketch

import (
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"
)

type gsum_ans struct {
	distinct float64
	l1       float64
	entropy  float64
	l2       float64
}

func gsum(values []float64) (float64, float64, float64, float64) {
	m := make(map[float64]int)
	n := float64(len(values))
	for _, v := range values {
		if _, ok := m[v]; !ok {
			m[v] = 1
		} else {
			m[v] += 1
		}
	}
	var l1, l2, entropy float64 = 0, 0, 0
	for _, v := range m {
		l1 += float64(v)
		l2 += float64(v * v)
		entropy += float64(v) * math.Log2(float64(v))
	}
	distinct := float64(len(m))
	l2 = math.Sqrt(l2)
	entropy = math.Log2(n) - entropy/n
	return distinct, l1, entropy, l2
}

func prometheus_gsum(total_length int64, time_window_size int64, t1, t2 []int64, ground_truth *([][]gsum_ans)) (float64, float64, float64) {
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
				(*ground_truth)[t][i] = gsum_ans{distinct: distinct, l1: l1, entropy: entropy, l2: l2}
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
					(*ground_truth)[t][i] = gsum_ans{distinct: distinct, l1: l1, entropy: entropy, l2: l2}
				}
			}
		}
	}
	return insert_compute, query_compute * 4, float64(time_window_size) * 8 / 1024 * 4
}

// Test cost (compute + memory) and accuracy under sliding window
func TestCostAnalysisGSum(t *testing.T) {
	constructInputTimeSeriesZipf()
	fmt.Println("Finished reading input timeseries")
	query_window_size := int64(1000000)
	total_length := int64(2000000)

	// Create a scenario
	// Query these subwindows every 1000 data sample insertions
	t1 := make([]int64, 0)
	t2 := make([]int64, 0)
	t1 = append(t1, int64(0))
	t2 = append(t2, query_window_size-1)

	for i := 1; i <= 10; i++ {
		t1 = append(t1, query_window_size/10*int64(i-1))
		t2 = append(t2, query_window_size/10*int64(i)-1)
	}

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

	fmt.Println("t1:", t1)
	fmt.Println("t2:", t2)

	ground_truth := make([][]gsum_ans, total_length)
	for t := 0; t < int(total_length); t++ {
		ground_truth[t] = make([]gsum_ans, len(t1))
	}

	// Prometheus baseline
	insert_compute, query_compute, memory := prometheus_gsum(total_length, query_window_size, t1, t2, &ground_truth)
	fmt.Println("Prometheus")
	fmt.Println("insert compute:", insert_compute, "us")
	fmt.Println("query compute:", query_compute, "us")
	fmt.Println("total compute:", query_compute+insert_compute, "us")
	fmt.Println("memory:", memory, "KB")

	// PromSketch, SHUniv
	beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	for _, beta := range beta_input {
		fmt.Println("SHUniv", beta)
		shu_distinct_error := make([]float64, 0)
		shu_l1_error := make([]float64, 0)
		shu_entropy_error := make([]float64, 0)
		shu_l2_error := make([]float64, 0)
		shu := SmoothInitUnivMon(beta, query_window_size)

		total_compute := 0.0
		insert_compute := 0.0
		for t := int64(0); t < total_length; t++ {
			start := time.Now()
			shu.Update(t, strconv.FormatFloat(cases[0].vec[t].F, 'f', -1, 64))
			elapsed := time.Since(start)
			insert_compute += float64(elapsed.Microseconds())

			if t == total_length-1 {
				for j := range len(t1) {
					start_t := t1[j] + t - query_window_size + 1
					end_t := t2[j] + t - query_window_size + 1
					start := time.Now()
					merged_univ, _ := shu.QueryIntervalMergeUniv(t-end_t, t-start_t, t)
					count := float64(merged_univ.GetBucketSize())
					distinct := merged_univ.calcCard()
					l1 := merged_univ.calcL1()
					l2 := merged_univ.calcL2()
					l2 = math.Sqrt(l2)
					entropy := merged_univ.calcEntropy()
					entropy = math.Log2(count) - entropy/count

					elapsed := time.Since(start)
					total_compute += float64(elapsed.Microseconds())

					rel_err := AbsFloat64(ground_truth[t][j].distinct-distinct) / (ground_truth[t][j].distinct) * 100
					shu_distinct_error = append(shu_distinct_error, rel_err)
					rel_err = AbsFloat64(ground_truth[t][j].l1-l1) / (ground_truth[t][j].l1) * 100
					shu_l1_error = append(shu_l1_error, rel_err)
					rel_err = AbsFloat64(ground_truth[t][j].entropy-entropy) / (ground_truth[t][j].entropy) * 100
					shu_entropy_error = append(shu_entropy_error, rel_err)
					rel_err = AbsFloat64(ground_truth[t][j].l2-l2) / (ground_truth[t][j].l2) * 100
					shu_l2_error = append(shu_l2_error, rel_err)

				}
			} else {
				if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						merged_univ, _ := shu.QueryIntervalMergeUniv(t-end_t, t-start_t, t)
						count := float64(merged_univ.GetBucketSize())
						distinct := merged_univ.calcCard()
						l1 := merged_univ.calcL1()
						l2 := merged_univ.calcL2()
						l2 = math.Sqrt(l2)
						entropy := merged_univ.calcEntropy()
						entropy = math.Log2(count) - entropy/count
						elapsed := time.Since(start)
						total_compute += float64(elapsed.Microseconds())

						rel_err := AbsFloat64(ground_truth[t][j].distinct-distinct) / (ground_truth[t][j].distinct) * 100
						shu_distinct_error = append(shu_distinct_error, rel_err)
						rel_err = AbsFloat64(ground_truth[t][j].l1-l1) / (ground_truth[t][j].l1) * 100
						shu_l1_error = append(shu_l1_error, rel_err)
						rel_err = AbsFloat64(ground_truth[t][j].entropy-entropy) / (ground_truth[t][j].entropy) * 100
						shu_entropy_error = append(shu_entropy_error, rel_err)
						rel_err = AbsFloat64(ground_truth[t][j].l2-l2) / (ground_truth[t][j].l2) * 100
						shu_l2_error = append(shu_l2_error, rel_err)
					}
				}
			}
		}
		fmt.Println("distinct error:", shu_distinct_error)
		fmt.Println("l1 error:", shu_l1_error)
		fmt.Println("entropy error:", shu_entropy_error)
		fmt.Println("l2 error:", shu_l2_error)

		fmt.Println("insert compute:", insert_compute)
		fmt.Println("query compute:", total_compute, "us")
		fmt.Println("total compute:", total_compute+insert_compute, "us")
		fmt.Println("memory:", shu.GetMemory(), "KB")
	}

	// Sampling baselines
	sampling_distinct_error := make([][]float64, 0)
	sampling_l1_error := make([][]float64, 0)
	sampling_entropy_error := make([][]float64, 0)
	sampling_l2_error := make([][]float64, 0)
	sampling_rate := []float64{0.001, 0.01, 0.05, 0.1, 0.2, 0.3}
	for i, rate := range sampling_rate {
		fmt.Println("sampling", rate)
		query_compute := 0.0
		insert_compute := 0.0
		sampling_l1_error = append(sampling_l1_error, make([]float64, 0))
		sampling_l2_error = append(sampling_l2_error, make([]float64, 0))
		sampling_entropy_error = append(sampling_entropy_error, make([]float64, 0))
		sampling_distinct_error = append(sampling_distinct_error, make([]float64, 0))

		sampling_size := int(float64(query_window_size) * rate)
		sampling_instance := NewUniformSampling(query_window_size, rate, sampling_size)
		for t := int64(0); t < total_length; t++ {
			start := time.Now()
			sampling_instance.Insert(t, cases[0].vec[t].F)
			elapsed := time.Since(start)
			insert_compute += float64(elapsed.Microseconds())

			if t == total_length-1 {
				for j := range len(t1) {
					start_t := t1[j] + t - query_window_size + 1
					end_t := t2[j] + t - query_window_size + 1
					start := time.Now()
					l1 := sampling_instance.QueryL1(start_t, end_t)
					l2 := sampling_instance.QueryL2(start_t, end_t)
					distinct := sampling_instance.QueryDistinct(start_t, end_t)
					entropy := sampling_instance.QueryEntropy(start_t, end_t)
					elapsed := time.Since(start)
					query_compute += float64(elapsed.Microseconds())

					rel_err := AbsFloat64(ground_truth[t][j].distinct-distinct) / (ground_truth[t][j].distinct) * 100
					sampling_distinct_error[i] = append(sampling_distinct_error[i], rel_err)
					rel_err = AbsFloat64(ground_truth[t][j].l1-l1) / (ground_truth[t][j].l1) * 100
					sampling_l1_error[i] = append(sampling_l1_error[i], rel_err)
					rel_err = AbsFloat64(ground_truth[t][j].entropy-entropy) / (ground_truth[t][j].entropy) * 100
					sampling_entropy_error[i] = append(sampling_entropy_error[i], rel_err)
					rel_err = AbsFloat64(ground_truth[t][j].l2-l2) / (ground_truth[t][j].l2) * 100
					sampling_l2_error[i] = append(sampling_l2_error[i], rel_err)
				}
			} else {
				if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						l1 := sampling_instance.QueryL1(start_t, end_t)
						l2 := sampling_instance.QueryL2(start_t, end_t)
						distinct := sampling_instance.QueryDistinct(start_t, end_t)
						entropy := sampling_instance.QueryEntropy(start_t, end_t)
						elapsed := time.Since(start)
						query_compute += float64(elapsed.Microseconds())

						rel_err := AbsFloat64(ground_truth[t][j].distinct-distinct) / (ground_truth[t][j].distinct) * 100
						sampling_distinct_error[i] = append(sampling_distinct_error[i], rel_err)
						rel_err = AbsFloat64(ground_truth[t][j].l1-l1) / (ground_truth[t][j].l1) * 100
						sampling_l1_error[i] = append(sampling_l1_error[i], rel_err)
						rel_err = AbsFloat64(ground_truth[t][j].entropy-entropy) / (ground_truth[t][j].entropy) * 100
						sampling_entropy_error[i] = append(sampling_entropy_error[i], rel_err)
						rel_err = AbsFloat64(ground_truth[t][j].l2-l2) / (ground_truth[t][j].l2) * 100
						sampling_l2_error[i] = append(sampling_l2_error[i], rel_err)
					}
				}
			}

		}
		fmt.Println("distinct error:", sampling_distinct_error[i])
		fmt.Println("l1 error:", sampling_l1_error[i])
		fmt.Println("entropy error:", sampling_entropy_error[i])
		fmt.Println("l2 error:", sampling_l2_error[i])

		fmt.Println("insert compute:", insert_compute, "us")
		fmt.Println("query compute:", query_compute, "us")
		fmt.Println("total compute:", query_compute+insert_compute, "us")
		fmt.Println("memory:", sampling_instance.GetMemory(), "KB")
	}

}
