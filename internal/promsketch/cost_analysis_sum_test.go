package promsketch

import (
	"fmt"
	"testing"
	"time"
)

const R_value float64 = 1

func sum(values []float64) float64 {
	var sum float64 = 0
	for _, v := range values {
		sum += v
	}
	return sum
}

func prometheus_sum(total_length int64, time_window_size int64, t1, t2 []int64, ground_truth *([][]float64)) (float64, float64, float64) {
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
				ans := sum(values)
				elapsed = time.Since(start)
				query_compute += float64(elapsed.Microseconds())
				(*ground_truth)[t][i] = ans
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
					ans := sum(values)
					elapsed = time.Since(start)
					query_compute += float64(elapsed.Microseconds())
					(*ground_truth)[t][i] = ans
				}
			}
		}
	}
	return insert_compute, query_compute, float64(time_window_size) * 8 / 1024
}

func TestCostAnalysisSum(t *testing.T) {
	readGoogleClusterData2009()
	// readPowerDataset()
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

	/*
		start_t = t1[len(t1)-1]
		for i := 1; i <= 10; i++ {
			t1 = append(t1, start_t+query_window_size/10/10/10*int64(i-1))
			t2 = append(t2, start_t+query_window_size/10/10/10*int64(i)-1)
		}
	*/

	fmt.Println("t1:", t1)
	fmt.Println("t2:", t2)

	ground_truth := make([][]float64, total_length)
	for t := 0; t < int(total_length); t++ {
		ground_truth[t] = make([]float64, len(t1))
	}

	// Prometheus baseline
	insert_compute, query_compute, memory := prometheus_sum(total_length, query_window_size, t1, t2, &ground_truth)
	fmt.Println("Prometheus")
	fmt.Println("insert compute:", insert_compute, "us")
	fmt.Println("query compute:", query_compute, "us")
	fmt.Println("total compute:", query_compute+insert_compute, "us")
	fmt.Println("memory:", memory, "KB")

	// Sampling baselines
	sampling_rel_error := make([][]float64, 0)
	sampling_rate := []float64{0.001, 0.01, 0.05, 0.1}
	for i, rate := range sampling_rate {
		fmt.Println("sampling", rate)
		query_compute := 0.0
		insert_compute := 0.0
		sampling_rel_error = append(sampling_rel_error, make([]float64, 0))

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
					ans_sum := sampling_instance.QuerySum(start_t, end_t)
					elapsed := time.Since(start)
					query_compute += float64(elapsed.Microseconds())
					rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / (ground_truth[t][j]) * 100
					// rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / (R_value * float64(end_t-start_t+1))
					sampling_rel_error[i] = append(sampling_rel_error[i], rel_err)
				}
			} else {
				if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						ans_sum := sampling_instance.QuerySum(start_t, end_t)
						elapsed := time.Since(start)
						query_compute += float64(elapsed.Microseconds())
						rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / (ground_truth[t][j]) * 100
						// rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / (R_value * float64(end_t-start_t+1))
						sampling_rel_error[i] = append(sampling_rel_error[i], rel_err)
					}
				}
			}

		}
		fmt.Println("relative error:", sampling_rel_error[i])
		fmt.Println("insert compute:", insert_compute, "us")
		fmt.Println("query compute:", query_compute, "us")
		fmt.Println("total compute:", query_compute+insert_compute, "us")
		fmt.Println("memory:", sampling_instance.GetMemory(), "KB")
	}

	/*
		// PromSketch, EfficientSum
		epsilon_input := []float64{0.001, 0.0001, 0.00001, 0.000001}
		for _, epsilon := range epsilon_input {

			fmt.Println("effsum", epsilon)
			effsum_rel_error := make([]float64, 0)
			total_compute := 0.0
			insert_compute := 0.0
			effsum := NewEfficientSum(query_window_size, query_window_size, epsilon, R_value)

			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				effsum.Insert(t, cases[0].vec[t].F)
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())
				if t == total_length-1 {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						ans_sum := float64(0)
						if j == 0 {
							ans_sum = effsum.Query(start_t, end_t, false)
						} else {
							ans_sum = effsum.Query(start_t, end_t, true)
						}
						elapsed := time.Since(start)
						total_compute += float64(elapsed.Microseconds())
						rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / ground_truth[t][j] * 100
						// rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / (R_value * float64(end_t-start_t+1))
						effsum_rel_error = append(effsum_rel_error, rel_err)
					}
				} else {
					if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
						for j := range len(t1) {
							start_t := t1[j] + t - query_window_size + 1
							end_t := t2[j] + t - query_window_size + 1
							start := time.Now()
							ans_sum := float64(0)
							if j == 0 {
								ans_sum = effsum.Query(start_t, end_t, false)
							} else {
								ans_sum = effsum.Query(start_t, end_t, true)
							}
							elapsed := time.Since(start)
							total_compute += float64(elapsed.Microseconds())
							rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / ground_truth[t][j] * 100
							// rel_err := AbsFloat64(ground_truth[t][j]-ans_sum) / (R_value * float64(end_t-start_t+1)) // Additive error defined in the paper
							effsum_rel_error = append(effsum_rel_error, rel_err)
						}
					}
				}
			}
			fmt.Println("relative error:", effsum_rel_error)
			fmt.Println("insert compute:", insert_compute)
			fmt.Println("query compute:", total_compute, "us")
			fmt.Println("total compute:", total_compute+insert_compute, "us")
			fmt.Println("memory:", effsum.GetMemory(), "KB")
		}
	*/
}
