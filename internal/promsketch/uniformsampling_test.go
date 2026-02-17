package promsketch

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func funcUniformSamplingQuantile(time_window_size int64, sampling_size int, phis []float64, GroundTruth map[string]([]AnswerQuantile), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with ExpoHistogram Count===================>\n")
	w.Flush()
	t2 := int64(time_window_size)

	ehs := make(map[string](*UniformSampling))
	for _, c := range cases {
		ehs[c.key] = NewUniformSampling(time_window_size, float64(sampling_size)/float64(time_window_size), sampling_size)
	}
	EHAnswer := make(map[string]([]AnswerQuantile))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerQuantile, 0)
	}

	var (
		total_insert_time float64 = 0
		total_inserts     float64 = 0
	)
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			start := time.Now()
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F)
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_inserts += 1
			if t >= t2 {
				start := time.Now()
				ans_quantile := ehs[c.key].QueryQuantile(phis, t-t2, t)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerQuantile{quantiles: ans_quantile, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_quantile, query_time(us), memory(KB)\n", c.key)
		w.Flush()

		avg_error := make(map[string]([][]float64))
		max_error := make(map[string]([][]float64))
		for _, c := range cases {
			avg_error[c.key] = make([][]float64, time_window_size*2-t2)
			max_error[c.key] = make([][]float64, time_window_size*2-t2)
			for j := int64(0); j < time_window_size*2-t2; j++ {
				avg_error[c.key][j] = make([]float64, len(phis))
				max_error[c.key][j] = make([]float64, len(phis))
			}
		}

		for i := 0; i < len(GroundTruth[c.key]); i++ {
			// fmt.Println("** quantile error:", GroundTruth[c.key][i].quantile, EHAnswer[c.key][i].quantile, AbsFloat64(GroundTruth[c.key][i].quantile - EHAnswer[c.key][i].quantile) / GroundTruth[c.key][i].quantile)
			// error_quantile := make([]float64, 0)
			for idx := range phis {
				// error_quantile = append(error_quantile, AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
				avg_error[c.key][i][idx] += AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx]) / GroundTruth[c.key][i].quantiles[idx]
				max_error[c.key][i][idx] = MaxFloat64(max_error[c.key][i][idx], AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
			}
			// fmt.Fprintf(w, "quantile errors: %v\n", error_quantile)
			w.Flush()
		}

		avg_error_total := float64(0.0)
		max_error_total := float64(0.0)
		total_avg_count := float64(0.0)
		var (
			total_time    float64 = 0
			total_memory  float64 = 0
			total_queries float64 = 0
		)
		fmt.Fprintf(w, "-----------------avg_error-----------------\n")
		for _, c := range cases {
			for i := 0; i < len(GroundTruth[c.key]); i++ {
				time := EHAnswer[c.key][i].time
				memory := EHAnswer[c.key][i].memory
				total_time += time
				total_memory += memory
				total_queries += 1
				for idx := range phis {
					// fmt.Println("err:", avg_error[c.key][i] / float64(test_times), "t1:", "t2:", i, int64(i) + t2 - t1)
					avg_error_total = avg_error_total + avg_error[c.key][i][idx]/float64(test_times)
					total_avg_count += 1
				}
			}
		}

		fmt.Fprintf(w, "-----------------max_error-----------------\n")
		for _, c := range cases {
			for i := 0; i < len(GroundTruth[c.key]); i++ {
				for idx := range phis {
					// fmt.Println("err:", max_error[c.key][i], "t1:", "t2:", i, int64(i) + t2 - t1)
					max_error_total = MaxFloat64(max_error_total, max_error[c.key][i][idx])
				}
			}
		}
		fmt.Fprintf(w, "avg of avg_error: %f%%\n", avg_error_total/total_avg_count*100)
		fmt.Fprintf(w, "max of max_error: %f%%\n", max_error_total*100)
		fmt.Fprintf(w, "avg of insert sampling quantile (us): %f\n", float64(total_insert_time)/total_inserts)
		fmt.Fprintf(w, "avg of query sampling quantile (us): %f\n", float64(total_time)/total_queries)
		fmt.Fprintf(w, "avg of memory sampling quantile (KB): %f\n", total_memory/total_queries)
		w.Flush()
	}
	w.Flush()
}

func TestUniformSamplingQuantile(t *testing.T) {
	runtime.GOMAXPROCS(64)
	// constructInputTimeSeriesZipf()
	constructInputTimeSeriesUniformFloat64()
	// readPowerDataset()
	// readGoogleClusterData2009()
	fmt.Println("finished construct input time series")


	time_window_size_input := []int64{1000, 10000, 100000, 1000000}
	sampling_rate := []float64{0.01, 0.05}
	// phis_input := []float64{0, 0.1, 0.2, 0.25, 0.3, 0.4, 0.5, 0.6, 0.7, 0.75, 0.8, 0.9, 0.99, 1}
	phis_input := []float64{0.9}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, rate := range sampling_rate {
			sampling_size := int(float64(time_window_size) * rate)
			atomic.AddUint64(&ops, add)
			for {
				if ops < 20 {
					break
				}
			}
			testname := fmt.Sprintf("USampling_Quantile_%d_%d_0.9_uniform", time_window_size, sampling_size)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(time_window_size int64, sampling_size int) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				fmt.Printf("=================>Testing sampling_size=%d, window_size=%d===================>\n", sampling_size, time_window_size)
				w := bufio.NewWriter(f)
				fmt.Fprintf(w, "=================>Testing sampling_size=%d, window_size=%d===================>\n", sampling_size, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthQuantile(time_window_size, phis_input, w)
				funcUniformSamplingQuantile(time_window_size, sampling_size, phis_input, GroundTruth, w)
				w.Flush()
				atomic.AddUint64(&ops, -add)
				wg.Done()
			}(time_window_size, sampling_size)
		}
	}
	wg.Wait()
}

func funcSamplingSum(time_window_size int64, max_size int, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with Efficient Count, Sum, Sum2===================>\n")
	w.Flush()
	t2 := int64(time_window_size)

	// Sum
	ehs := make(map[string](*UniformSampling))
	for _, c := range cases {
		ehs[c.key] = NewUniformSampling(time_window_size, float64(max_size)/float64(time_window_size), max_size)
	}

	EHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerSum2, 0)
	}
	var (
		total_insert_time float64 = 0
		total_inserts     float64 = 0
	)
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			start := time.Now()
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F)
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_inserts += 1
			if t >= t2 {
				start := time.Now()
				ans_sum := ehs[c.key].QuerySum(t-t2, t)
				ans_sum2 := ehs[c.key].QuerySum2(t-t2, t)
				ans_count := ehs[c.key].QueryCount(t-t2, t)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerSum2{count: ans_count, sum2: ans_sum2, sum: ans_sum, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%s error_count, error_sum, error_sum2, query_time(us)\n", c.key)
		w.Flush()
		// assert.Equal(t, len(GroundTruth[c.key]), len(EHAnswer[c.key]), "the answer length should be the same.")

		var (
			total_error_count float64 = 0
			total_error_sum   float64 = 0
			total_error_sum2  float64 = 0
			total_time        float64 = 0
			total_memory      float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ {
			error_count := AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count) / GroundTruth[c.key][i].count
			error_sum := AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			error_sum2 := AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2
			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			total_error_count += error_count
			total_error_sum += error_sum
			total_error_sum2 += error_sum2
			total_time += time
			total_memory += memory
		}
		fmt.Fprintf(w, "Average error, time, memory: avg_count_error: %f%%, avg_sum_error: %f%%, avg_sum2_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_count/float64(len(GroundTruth[c.key]))*100, total_error_sum/float64(len(GroundTruth[c.key]))*100, total_error_sum2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

// test count_over_time, sum_over_time, sum2_over_time
func TestSamplingSum(t *testing.T) {
	runtime.GOMAXPROCS(64)
	// constructInputTimeSeriesZipf()
	// readGoogleClusterData2009()
	// constructInputTimeSeriesUniformFloat64()
	// readGoogleClusterData2009()
	// constructInputTimeSeriesUniformFloat64()
	readPowerDataset()
	fmt.Println("finished construct input time series")

	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	sampling_rate := []float64{0.01, 0.05}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, rate := range sampling_rate {
			max_size := int(float64(time_window_size) * rate)
			atomic.AddUint64(&ops, add)
			for {
				if ops < 40 {
					break
				}
			}
			testname := fmt.Sprintf("USampling_Sum_%d_%d_power", time_window_size, max_size)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(max_size int, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				fmt.Printf("=================>Testing sampling_size=%d, window_size=%d===================>\n", max_size, time_window_size)
				w := bufio.NewWriter(f)
				fmt.Fprintf(w, "=================>Testing sampling_size=%d, window_size=%d===================>\n", max_size, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthSUM2(time_window_size, w)
				funcSamplingSum(time_window_size, max_size, GroundTruth, w)
				w.Flush()
				atomic.AddUint64(&ops, -add)
				wg.Done()
			}(max_size, time_window_size)
		}
	}
	wg.Wait()
}

func funcSamplingLp(max_size int, time_window_size int64, GroundTruth map[string]([]AnswerUniv), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with Sampling Lp (L0, L1, L2, entropy)===================>\n")
	w.Flush()
	t2 := int64(time_window_size)

	// one EH+Univ for one timeseries
	ehs := make(map[string](*UniformSampling))
	for _, c := range cases {
		ehs[c.key] = NewUniformSampling(time_window_size, float64(max_size)/float64(time_window_size), max_size)
	}

	EHAnswer := make(map[string]([]AnswerUniv))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerUniv, 0)
	}

	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F) // insert data to univmon for current timeseries
			if t >= t2 {
				start := time.Now()
				ans_l1 := ehs[c.key].QueryL1(t-t2, t)
				ans_l2 := ehs[c.key].QueryL2(t-t2, t)
				ans_entropy := ehs[c.key].QueryEntropy(t-t2, t)
				ans_distinct := ehs[c.key].QueryDistinct(t-t2, t)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerUniv{card: ans_distinct, l1: ans_l1, l2: ans_l2, entropy: ans_entropy, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})

			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%s error_card, error_l1, error_l2, error_entropy, query_time(us), memory (KB)\n", c.key)
		w.Flush()

		var (
			total_error_card    float64 = 0
			total_error_l1      float64 = 0
			total_error_l2      float64 = 0
			total_error_entropy float64 = 0
			total_time          float64 = 0
			total_memory        float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ { // time dimension
			error_card := AbsFloat64(GroundTruth[c.key][i].card-EHAnswer[c.key][i].card) / GroundTruth[c.key][i].card
			error_l1 := AbsFloat64(GroundTruth[c.key][i].l1-EHAnswer[c.key][i].l1) / GroundTruth[c.key][i].l1
			error_l2 := AbsFloat64(GroundTruth[c.key][i].l2-EHAnswer[c.key][i].l2) / GroundTruth[c.key][i].l2
			error_entropy := AbsFloat64(GroundTruth[c.key][i].entropy-EHAnswer[c.key][i].entropy) / GroundTruth[c.key][i].entropy

			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			total_error_card += error_card
			total_error_l1 += error_l1
			total_error_l2 += error_l2
			total_error_entropy += error_entropy
			total_time += time
			total_memory += memory
		}

		fmt.Fprintf(w, "Average error, time, and memory: avg_card_error: %f%%, avg_l1_error: %f%%, avg_l2_error: %f%%, avg_entropy_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_card/float64(len(GroundTruth[c.key]))*100, total_error_l1/float64(len(GroundTruth[c.key]))*100, total_error_l2/float64(len(GroundTruth[c.key]))*100, total_error_entropy/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

func TestSamplingLp(t *testing.T) {
	runtime.GOMAXPROCS(64)
	// constructInputTimeSeriesZipf()
	// constructInputTimeSeriesUniv()
	constructInputTimeSeriesUniformInt64()
	// readCAIDA()

	fmt.Println("finished construct input time series")

	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000, 10000000}
	sampling_rate := []float64{0.01, 0.05} // If needed, add 0.1 sampling rate to test 10% sampling
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, rate := range sampling_rate {
			max_size := int(float64(time_window_size) * rate)
			atomic.AddUint64(&ops, add)
			for {
				if ops < 20 {
					break
				}
			}
			testname := fmt.Sprintf("USampling_gsum_%d_%d_uniform", time_window_size, max_size)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(max_size int, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing max_size=%d, window_size=%d===================>\n", max_size, time_window_size)
				fmt.Fprintf(w, "=================>Testing max_size=%d, window_size=%d===================>\n", max_size, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthUniv(time_window_size, w)
				funcSamplingLp(max_size, time_window_size, GroundTruth, w)
				w.Flush()
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done max_size=%d, window_size=%d===================>\n", max_size, time_window_size)
				wg.Done()
			}(max_size, time_window_size)
		}
	}
	wg.Wait()
}

func funcUniformSamplingQuantileSubWindow(time_window_size int64, sampling_size int, phis []float64, GroundTruth map[string]([]AnswerQuantile), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "=================>Estimating with ExpoHistogram Count===================>\n")
	w.Flush()
	t1 := start_t
	t2 := int64(end_t)

	ehs := make(map[string](*UniformSampling))
	for _, c := range cases {
		ehs[c.key] = NewUniformSampling(time_window_size, float64(sampling_size)/float64(time_window_size), sampling_size)
	}
	EHAnswer := make(map[string]([]AnswerQuantile))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerQuantile, 0)
	}

	var (
		total_insert_time float64 = 0
		total_inserts     float64 = 0
	)
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			start := time.Now()
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F)
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_inserts += 1
			if t >= time_window_size {
				start := time.Now()
				ans_quantile := ehs[c.key].QueryQuantile(phis, t-t2, t-t1)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerQuantile{quantiles: ans_quantile, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_quantile, query_time(us), memory(KB)\n", c.key)
		w.Flush()

		avg_error := make(map[string]([][]float64))
		max_error := make(map[string]([][]float64))
		for _, c := range cases {
			avg_error[c.key] = make([][]float64, time_window_size*2-t2)
			max_error[c.key] = make([][]float64, time_window_size*2-t2)
			for j := int64(0); j < time_window_size*2-t2; j++ {
				avg_error[c.key][j] = make([]float64, len(phis))
				max_error[c.key][j] = make([]float64, len(phis))
			}
		}

		for i := 0; i < len(GroundTruth[c.key]); i++ {
			// fmt.Println("** quantile error:", GroundTruth[c.key][i].quantile, EHAnswer[c.key][i].quantile, AbsFloat64(GroundTruth[c.key][i].quantile - EHAnswer[c.key][i].quantile) / GroundTruth[c.key][i].quantile)
			// error_quantile := make([]float64, 0)
			for idx := range phis {
				// error_quantile = append(error_quantile, AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
				avg_error[c.key][i][idx] += AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx]) / GroundTruth[c.key][i].quantiles[idx]
				max_error[c.key][i][idx] = MaxFloat64(max_error[c.key][i][idx], AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
			}
			// fmt.Fprintf(w, "quantile errors: %v\n", error_quantile)
			w.Flush()
		}

		avg_error_total := float64(0.0)
		max_error_total := float64(0.0)
		total_avg_count := float64(0.0)
		var (
			total_time    float64 = 0
			total_memory  float64 = 0
			total_queries float64 = 0
		)
		fmt.Fprintf(w, "-----------------avg_error-----------------\n")
		for _, c := range cases {
			for i := 0; i < len(GroundTruth[c.key]); i++ {
				time := EHAnswer[c.key][i].time
				memory := EHAnswer[c.key][i].memory
				total_time += time
				total_memory += memory
				total_queries += 1
				for idx := range phis {
					// fmt.Println("err:", avg_error[c.key][i] / float64(test_times), "t1:", "t2:", i, int64(i) + t2 - t1)
					avg_error_total = avg_error_total + avg_error[c.key][i][idx]/float64(test_times)
					total_avg_count += 1
				}
			}
		}

		fmt.Fprintf(w, "-----------------max_error-----------------\n")
		for _, c := range cases {
			for i := 0; i < len(GroundTruth[c.key]); i++ {
				for idx := range phis {
					// fmt.Println("err:", max_error[c.key][i], "t1:", "t2:", i, int64(i) + t2 - t1)
					max_error_total = MaxFloat64(max_error_total, max_error[c.key][i][idx])
				}
			}
		}
		fmt.Fprintf(w, "avg of avg_error: %f%%\n", avg_error_total/total_avg_count*100)
		fmt.Fprintf(w, "max of max_error: %f%%\n", max_error_total*100)
		fmt.Fprintf(w, "avg of insert sampling quantile (us): %f\n", float64(total_insert_time)/total_inserts)
		fmt.Fprintf(w, "avg of query sampling quantile (us): %f\n", float64(total_time)/total_queries)
		fmt.Fprintf(w, "avg of memory sampling quantile (KB): %f\n", total_memory/total_queries)
		w.Flush()
	}
	w.Flush()
}

func TestUniformSamplingQuantileSubWindow(t *testing.T) {
	runtime.GOMAXPROCS(64)
	readPowerDataset()
	// constructInputTimeSeriesUniformFloat64()
	// constructInputTimeSeriesZipf()
	// readPowerDataset()
	// readPowerDataset()
	// constructInputTimeSeriesUniformFloat64()
// 	constructInputTimeSeriesZipf()
	// constructInputTimeSeriesZipf()
	// constructInputTimeSeriesZipf()
	readGoogleClusterData2009()

	fmt.Println("finished construct input time series")

	time_window_size_input := []int64{1000000}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	sampling_rate := []float64{0.01, 0.05}
	phis_input := []float64{0.5}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, subwindow_size := range subwindow_size_input {
			for _, rate := range sampling_rate {
				sampling_size := int(rate * float64(time_window_size))
				atomic.AddUint64(&ops, add)
				for {
					if ops < 20 {
						break
					}
				}
				testname := fmt.Sprintf("ReservoirSampling_Quantile_%d_%d_%d_%d_0.5_power", time_window_size, sampling_size, time_window_size-subwindow_size.end, time_window_size-subwindow_size.start)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(time_window_size int64, sampling_size int, start_t, end_t int64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					fmt.Printf("=================>Testing sampling_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", sampling_size, time_window_size, start_t, end_t)
					w := bufio.NewWriter(f)
					fmt.Fprintf(w, "=================>Testing sampling_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", sampling_size, time_window_size, start_t, end_t)
					w.Flush()
					GroundTruth := calcGroundTruthQuantileSubWindow(time_window_size, phis_input, w, start_t, end_t)
					funcUniformSamplingQuantileSubWindow(time_window_size, sampling_size, phis_input, GroundTruth, w, start_t, end_t)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done sampling_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", sampling_size, time_window_size, start_t, end_t)
					wg.Done()
				}(time_window_size, sampling_size, subwindow_size.start, subwindow_size.end)
			}
		}
	}
	wg.Wait()
}

func funcSamplingSumSubWindow(time_window_size int64, max_size int, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "=================>Estimating with Efficient Count, Sum, Sum2===================>\n")
	w.Flush()
	t1 := start_t
	t2 := int64(end_t)

	// Sum
	ehs := make(map[string](*UniformSampling))
	for _, c := range cases {
		ehs[c.key] = NewUniformSampling(time_window_size, float64(max_size)/float64(time_window_size), max_size)
	}

	EHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerSum2, 0)
	}
	var (
		total_insert_time float64 = 0
		total_inserts     float64 = 0
	)
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			start := time.Now()
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F)
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_inserts += 1
			if t >= time_window_size {
				start := time.Now()
				ans_sum := ehs[c.key].QuerySum(t-t2, t-t1)
				ans_sum2 := ehs[c.key].QuerySum2(t-t2, t-t1)
				ans_count := ehs[c.key].QueryCount(t-t2, t-t1)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerSum2{count: ans_count, sum2: ans_sum2, sum: ans_sum, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%s error_count, error_sum, error_sum2, query_time(us)\n", c.key)
		w.Flush()
		// assert.Equal(t, len(GroundTruth[c.key]), len(EHAnswer[c.key]), "the answer length should be the same.")

		var (
			total_error_count float64 = 0
			total_error_sum   float64 = 0
			total_error_sum2  float64 = 0
			total_time        float64 = 0
			total_memory      float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ {
			error_count := AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count) / GroundTruth[c.key][i].count
			error_sum := AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			error_sum2 := AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2
			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			total_error_count += error_count
			total_error_sum += error_sum
			total_error_sum2 += error_sum2
			total_time += time
			total_memory += memory
		}
		fmt.Fprintf(w, "Average error, time, memory: avg_count_error: %f%%, avg_sum_error: %f%%, avg_sum2_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_count/float64(len(GroundTruth[c.key]))*100, total_error_sum/float64(len(GroundTruth[c.key]))*100, total_error_sum2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

// test count_over_time, sum_over_time, sum2_over_time
func TestSamplingSumSubWindow(t *testing.T) {
	runtime.GOMAXPROCS(64)
	// readPowerDataset()
	// constructInputTimeSeriesZipf()
	readPowerDataset()
	// constructInputTimeSeriesUniformFloat64()
	// readGoogleClusterData2009()

	fmt.Println("finished construct input time series")

	time_window_size_input := []int64{1000000}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	sampling_rate := []float64{0.01, 0.05}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, subwindow_size := range subwindow_size_input {
			for _, rate := range sampling_rate {
				max_size := int(float64(time_window_size) * rate)
				atomic.AddUint64(&ops, add)
				for {
					if ops < 40 {
						break
					}
				}
				testname := fmt.Sprintf("USampling_Sum_%d_%d_%d_%d_power", time_window_size, max_size, time_window_size-subwindow_size.end, time_window_size-subwindow_size.start)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(max_size int, time_window_size int64, start_t, end_t int64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					fmt.Printf("=================>Testing sampling_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", max_size, time_window_size, start_t, end_t)
					w := bufio.NewWriter(f)
					fmt.Fprintf(w, "=================>Testing sampling_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", max_size, time_window_size, start_t, end_t)
					w.Flush()
					GroundTruth := calcGroundTruthSUM2SubWindow(time_window_size, w, start_t, end_t)
					funcSamplingSumSubWindow(time_window_size, max_size, GroundTruth, w, start_t, end_t)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done sampling_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", max_size, time_window_size, start_t, end_t)
					wg.Done()
				}(max_size, time_window_size, subwindow_size.start, subwindow_size.end)
			}
		}
	}
	wg.Wait()
}

func funcSamplingLpSubWindow(max_size int, time_window_size int64, GroundTruth map[string]([]AnswerUniv), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "=================>Estimating with Sampling Lp (L0, L1, L2, entropy)===================>\n")
	w.Flush()
	t1 := start_t
	t2 := int64(end_t)

	// one EH+Univ for one timeseries
	ehs := make(map[string](*UniformSampling))
	for _, c := range cases {
		ehs[c.key] = NewUniformSampling(time_window_size, float64(max_size)/float64(time_window_size), max_size)
	}

	EHAnswer := make(map[string]([]AnswerUniv))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerUniv, 0)
	}

	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F) // insert data to univmon for current timeseries
			if t >= time_window_size {
				start := time.Now()
				ans_l1 := ehs[c.key].QueryL1(t-t2, t-t1)
				ans_l2 := ehs[c.key].QueryL2(t-t2, t-t1)
				ans_entropy := ehs[c.key].QueryEntropy(t-t2, t-t1)
				ans_distinct := ehs[c.key].QueryDistinct(t-t2, t-t1)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerUniv{card: ans_distinct, l1: ans_l1, l2: ans_l2, entropy: ans_entropy, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})

			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%s error_card, error_l1, error_l2, error_entropy, query_time(us), memory (KB)\n", c.key)
		w.Flush()

		var (
			total_error_card    float64 = 0
			total_error_l1      float64 = 0
			total_error_l2      float64 = 0
			total_error_entropy float64 = 0
			total_time          float64 = 0
			total_memory        float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ { // time dimension
			error_card := AbsFloat64(GroundTruth[c.key][i].card-EHAnswer[c.key][i].card) / GroundTruth[c.key][i].card
			error_l1 := AbsFloat64(GroundTruth[c.key][i].l1-EHAnswer[c.key][i].l1) / GroundTruth[c.key][i].l1
			error_l2 := AbsFloat64(GroundTruth[c.key][i].l2-EHAnswer[c.key][i].l2) / GroundTruth[c.key][i].l2
			error_entropy := AbsFloat64(GroundTruth[c.key][i].entropy-EHAnswer[c.key][i].entropy) / GroundTruth[c.key][i].entropy

			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			total_error_card += error_card
			total_error_l1 += error_l1
			total_error_l2 += error_l2
			total_error_entropy += error_entropy
			total_time += time
			total_memory += memory
		}

		fmt.Fprintf(w, "Average error, time, and memory: avg_card_error: %f%%, avg_l1_error: %f%%, avg_l2_error: %f%%, avg_entropy_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_card/float64(len(GroundTruth[c.key]))*100, total_error_l1/float64(len(GroundTruth[c.key]))*100, total_error_l2/float64(len(GroundTruth[c.key]))*100, total_error_entropy/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

func TestSamplingLpSubWindow(t *testing.T) {
	runtime.GOMAXPROCS(64)
	// readCAIDA()
	constructInputTimeSeriesUniformInt64()
	// constructInputTimeSeriesZipf()
	//constructInputTimeSeriesUniv()
	fmt.Println("finished construct input time series")

	time_window_size_input := []int64{1000000}
	sampling_rate := []float64{0.01, 0.05, 0.1}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, subwindow_size := range subwindow_size_input {
			for _, rate := range sampling_rate {
				max_size := int(float64(time_window_size) * rate)
				atomic.AddUint64(&ops, add)
				for {
					if ops < 30 {
						break
					}
				}
				testname := fmt.Sprintf("USampling_Lp_%d_%d_%d_%d_uniform", time_window_size, max_size, time_window_size-subwindow_size.end, time_window_size-subwindow_size.start)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(max_size int, time_window_size int64, start_t int64, end_t int64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					w := bufio.NewWriter(f)
					fmt.Printf("=================>Testing max_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", max_size, time_window_size, start_t, end_t)
					fmt.Fprintf(w, "=================>Testing max_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", max_size, time_window_size, start_t, end_t)
					w.Flush()
					GroundTruth := calcGroundTruthUnivSubWindow(time_window_size, w, start_t, end_t)
					funcSamplingLpSubWindow(max_size, time_window_size, GroundTruth, w, start_t, end_t)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done max_size=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", max_size, time_window_size, start_t, end_t)
					wg.Done()
				}(max_size, time_window_size, subwindow_size.start, subwindow_size.end)
			}
		}
	}
	wg.Wait()
}
