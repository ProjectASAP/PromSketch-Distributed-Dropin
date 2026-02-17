package promsketch

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/DataDog/sketches-go/ddsketch"
	"github.com/zzylol/go-kll"
)

const cost_query_interval int64 = 10

func prometheus_quantile(total_length int64, time_window_size int64, t1, t2 []int64, phis float64, ground_truth *([][]float64)) (float64, float64, float64) {
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
				ans := quantile(phis, values)
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
					ans := quantile(phis, values)
					elapsed = time.Since(start)
					query_compute += float64(elapsed.Microseconds())
					(*ground_truth)[t][i] = ans
				}
			}
		}
	}
	return insert_compute, query_compute, float64(time_window_size) * 8 / 1024
}

func KolmogorovSmirnovStatisticKLL(start_t, end_t int64, kll *kll.Sketch) float64 {
	values := make([]float64, 0)
	for t := start_t; t <= end_t; t++ {
		values = append(values, cases[0].vec[t].F)
	}
	sort.Float64s(values)

	var D_ks float64 = 0
	for _, x := range values {
		cdf1 := kll.Quantile(x)
		idx := sort.SearchFloat64s(values, x)
		cdf2 := float64(idx)
		for i := idx + 1; i < len(values); i++ {
			if values[i] <= x {
				cdf2 += 1
			} else {
				break
			}
		}
		cdf2 = cdf2 / float64(len(values))
		D_ks = MaxFloat64(AbsFloat64(cdf1-cdf2), D_ks)
	}
	return D_ks
}

func KolmogorovSmirnovStatisticSampling(start_t, end_t int64, samples []float64) float64 {
	values := make([]float64, 0)
	for t := start_t; t <= end_t; t++ {
		values = append(values, cases[0].vec[t].F)
	}
	sort.Float64s(values)
	sort.Float64s(samples)
	var D_ks float64 = 0
	for _, x := range values {
		idx1 := sort.SearchFloat64s(samples, x)
		cdf1 := float64(idx1)
		for i := idx1 + 1; i < len(samples); i++ {
			if samples[i] <= x {
				cdf1 += 1
			} else {
				break
			}
		}
		cdf1 = cdf1 / float64(len(samples))

		idx := sort.SearchFloat64s(values, x)
		cdf2 := float64(idx)
		for i := idx + 1; i < len(values); i++ {
			if values[i] <= x {
				cdf2 += 1
			} else {
				break
			}
		}
		cdf2 = cdf2 / float64(len(values))
		D_ks = MaxFloat64(AbsFloat64(cdf1-cdf2), D_ks)
	}
	return D_ks
}

func KolmogorovSmirnovStatisticDD(start_t, end_t int64, dd *ddsketch.DDSketch) float64 {
	values := make([]float64, 0)
	for t := start_t; t <= end_t; t++ {
		values = append(values, cases[0].vec[t].F)
	}
	sort.Float64s(values)

	var D_ks float64 = 0
	qs := []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1}
	for _, q := range qs {
		cdf1 := q
		x, _ := dd.GetValueAtQuantile(q)

		idx := sort.SearchFloat64s(values, x)
		cdf2 := float64(idx)
		for i := idx + 1; i < len(values); i++ {
			if values[i] <= x {
				cdf2 += 1
			} else {
				break
			}
		}
		cdf2 = cdf2 / float64(len(values))
		D_ks = MaxFloat64(AbsFloat64(cdf1-cdf2), D_ks)
	}
	return D_ks
}

func getRankError(phi float64, start_t, end_t int64, est float64) float64 {
	n := float64(end_t - start_t + 1)
	rank := phi * (n - 1)
	larger := 0
	smaller := 0
	for t := start_t; t <= end_t; t++ {
		v := cases[0].vec[t].F
		if v > est {
			larger += 1
		}
		if v < est {
			smaller += 1
		}
	}
	if rank >= float64(smaller) && rank <= n-float64(larger) {
		return 0
	} else {
		if rank < float64(smaller) {
			return (float64(smaller) - rank) / n
		} else {
			return (rank - (n - float64(larger))) / n
		}
	}
}

// Test cost (compute + memory) and accuracy under sliding window
func TestCostAnalysisQuantile(t *testing.T) {
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

	start_t = t1[len(t1)-1]
	for i := 1; i <= 10; i++ {
		t1 = append(t1, start_t+query_window_size/10/10/10*int64(i-1))
		t2 = append(t2, start_t+query_window_size/10/10/10*int64(i)-1)
	}

	fmt.Println("t1:", t1)
	fmt.Println("t2:", t2)

	phi := 0.9
	phis := []float64{0.9}
	ground_truth := make([][]float64, total_length)
	for t := 0; t < int(total_length); t++ {
		ground_truth[t] = make([]float64, len(t1))
	}

	// Prometheus baseline
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
				start_t := t1[i] + t - query_window_size + 1
				end_t := t2[i] + t - query_window_size + 1
				start = time.Now()
				values := make([]float64, 0)
				for t := start_t; t <= end_t; t++ {
					values = append(values, cases[0].vec[t].F)
				}
				ans := quantile(phi, values)
				elapsed = time.Since(start)
				query_compute += float64(elapsed.Microseconds())
				ground_truth[t][i] = ans
			}
		} else {
			if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
				for i := range len(t1) {
					start_t := t1[i] + t - query_window_size + 1
					end_t := t2[i] + t - query_window_size + 1
					start = time.Now()
					values := make([]float64, 0)
					for t := start_t; t <= end_t; t++ {
						values = append(values, cases[0].vec[t].F)
					}
					ans := quantile(phi, values)
					elapsed = time.Since(start)
					query_compute += float64(elapsed.Microseconds())
					ground_truth[t][i] = ans
				}
			}
		}
	}
	memory := float64(query_window_size) * 8 / 1024

	fmt.Println("Prometheus")
	fmt.Println("insert compute:", insert_compute, "us")
	fmt.Println("query compute:", query_compute, "us")
	fmt.Println("total compute:", query_compute+insert_compute, "us")
	fmt.Println("memory:", memory, "KB")

	// Sampling baselines
	sampling_rel_error := make([][]float64, 0)
	sampling_rank_error := make([][]float64, 0)
	sampling_rate := []float64{0.001, 0.01, 0.05, 0.1}
	for i, rate := range sampling_rate {
		fmt.Println("sampling", rate)
		query_compute := 0.0
		insert_compute := 0.0
		sampling_rel_error = append(sampling_rel_error, make([]float64, 0))
		sampling_rank_error = append(sampling_rank_error, make([]float64, 0))

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
					ans_quantile := sampling_instance.QueryQuantile(phis, start_t, end_t)
					elapsed := time.Since(start)
					query_compute += float64(elapsed.Microseconds())
					rel_err := AbsFloat64(ground_truth[t][j]-ans_quantile[0]) / (ground_truth[t][j]) * 100
					// rank_err := getRankError(phi, start_t, end_t, ans_quantile[0])
					samples := sampling_instance.GetSamples(start_t, end_t)
					rank_err := KolmogorovSmirnovStatisticSampling(start_t, end_t, samples)

					sampling_rel_error[i] = append(sampling_rel_error[i], rel_err)
					sampling_rank_error[i] = append(sampling_rank_error[i], rank_err)
				}
			} else {
				if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						ans_quantile := sampling_instance.QueryQuantile(phis, start_t, end_t)
						elapsed := time.Since(start)
						query_compute += float64(elapsed.Microseconds())
						rel_err := AbsFloat64(ground_truth[t][j]-ans_quantile[0]) / (ground_truth[t][j]) * 100
						// rank_err := getRankError(phi, start_t, end_t, ans_quantile[0])
						samples := sampling_instance.GetSamples(start_t, end_t)
						rank_err := KolmogorovSmirnovStatisticSampling(start_t, end_t, samples)

						sampling_rel_error[i] = append(sampling_rel_error[i], rel_err)
						sampling_rank_error[i] = append(sampling_rank_error[i], rank_err)
					}
				}
			}

		}
		fmt.Println("rank error:", sampling_rank_error[i])
		fmt.Println("relative error:", sampling_rel_error[i])
		fmt.Println("insert compute:", insert_compute, "us")
		fmt.Println("query compute:", query_compute, "us")
		fmt.Println("total compute:", query_compute+insert_compute, "us")
		fmt.Println("memory:", sampling_instance.GetMemory(), "KB")
	}

	// PromSketch, EHKLL
	k_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	kllk_input := []int{64, 128, 256, 512, 1024}
	for _, k := range k_input {
		for _, kll_k := range kllk_input {
			fmt.Println("EHKLL", k, kll_k)
			ehkll_rel_error := make([]float64, 0)
			ehkll_rank_error := make([]float64, 0)
			ehkll := ExpoInitKLL(k, kll_k, query_window_size)

			total_compute := 0.0
			insert_compute := 0.0
			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				ehkll.Update(t, cases[0].vec[t].F)
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())

				if t == total_length-1 {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						merged_kll := ehkll.QueryIntervalMergeKLL(start_t, end_t)
						cdf := merged_kll.CDF()
						q_value := cdf.Query(phi)
						elapsed := time.Since(start)
						total_compute += float64(elapsed.Microseconds())

						rel_err := AbsFloat64(ground_truth[t][j]-q_value) / ground_truth[t][j] * 100
						// rank_err := getRankError(phi, start_t, end_t, q_value)
						kstest := KolmogorovSmirnovStatisticKLL(start_t, end_t, merged_kll)
						rank_err := kstest
						ehkll_rank_error = append(ehkll_rank_error, rank_err)
						ehkll_rel_error = append(ehkll_rel_error, rel_err)

					}
				} else {
					if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
						for j := range len(t1) {
							start_t := t1[j] + t - query_window_size + 1
							end_t := t2[j] + t - query_window_size + 1
							start := time.Now()
							merged_kll := ehkll.QueryIntervalMergeKLL(start_t, end_t)
							cdf := merged_kll.CDF()
							q_value := cdf.Query(phi)
							elapsed := time.Since(start)
							total_compute += float64(elapsed.Microseconds())

							rel_err := AbsFloat64(ground_truth[t][j]-q_value) / ground_truth[t][j] * 100
							// rank_err := getRankError(phi, start_t, end_t, q_value)
							kstest := KolmogorovSmirnovStatisticKLL(start_t, end_t, merged_kll)
							rank_err := kstest
							ehkll_rank_error = append(ehkll_rank_error, rank_err)
							ehkll_rel_error = append(ehkll_rel_error, rel_err)
						}
					}
				}
			}
			fmt.Println("rank error:", ehkll_rank_error)
			fmt.Println("relative error:", ehkll_rel_error)
			fmt.Println("insert compute:", insert_compute)
			fmt.Println("query compute:", total_compute, "us")
			fmt.Println("total compute:", total_compute+insert_compute, "us")
			fmt.Println("memory:", ehkll.GetMemory(), "KB")
		}
	}

	// PromSketch, EHDD
	dd_acc_input := []float64{0.1, 0.05, 0.02, 0.01, 0.001}
	for _, k := range k_input {
		for _, dd_acc := range dd_acc_input {
			fmt.Println("EHDD", k, dd_acc)
			ehdd_rel_error := make([]float64, 0)
			ehdd_rank_error := make([]float64, 0)
			total_compute := 0.0
			insert_compute := 0.0
			ehdd := ExpoInitDD(k, query_window_size, dd_acc)

			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				ehdd.Update(t, cases[0].vec[t].F)
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())
				if t == total_length-1 {
					for j := range len(t1) {
						start_t := t1[j] + t - query_window_size + 1
						end_t := t2[j] + t - query_window_size + 1
						start := time.Now()
						merged_dd := ehdd.QueryIntervalMergeDD(start_t, end_t)
						q_values, _ := merged_dd.GetValuesAtQuantiles(phis)
						// fmt.Println(q_values, err)
						elapsed := time.Since(start)
						total_compute += float64(elapsed.Microseconds())

						// rank_err := getRankError(phi, start_t, end_t, q_values[0])
						rank_err := KolmogorovSmirnovStatisticDD(start_t, end_t, merged_dd)
						rel_err := AbsFloat64(ground_truth[t][j]-q_values[0]) / ground_truth[t][j] * 100

						ehdd_rel_error = append(ehdd_rel_error, rel_err)
						ehdd_rank_error = append(ehdd_rank_error, rank_err)

					}
				} else {
					if t >= query_window_size-1 && (t+1)%cost_query_interval == 0 {
						for j := range len(t1) {
							start_t := t1[j] + t - query_window_size + 1
							end_t := t2[j] + t - query_window_size + 1
							start := time.Now()
							merged_dd := ehdd.QueryIntervalMergeDD(start_t, end_t)
							q_values, _ := merged_dd.GetValuesAtQuantiles(phis)
							elapsed := time.Since(start)
							total_compute += float64(elapsed.Microseconds())

							// rank_err := getRankError(phi, start_t, end_t, q_values[0])
							rank_err := KolmogorovSmirnovStatisticDD(start_t, end_t, merged_dd)
							rel_err := AbsFloat64(ground_truth[t][j]-q_values[0]) / ground_truth[t][j] * 100
							ehdd_rel_error = append(ehdd_rel_error, rel_err)
							ehdd_rank_error = append(ehdd_rank_error, rank_err)
						}
					}
				}
			}
			fmt.Println("rank error:", ehdd_rank_error)
			fmt.Println("relative error:", ehdd_rel_error)
			fmt.Println("insert compute:", insert_compute)
			fmt.Println("query compute:", total_compute, "us")
			fmt.Println("total compute:", total_compute+insert_compute, "us")
			fmt.Println("memory:", ehdd.GetMemory(), "KB")
		}
	}

}
