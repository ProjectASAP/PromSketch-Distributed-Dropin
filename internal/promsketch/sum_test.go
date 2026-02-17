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

func funcEfficientSum(epsilon float64, item_window_size int64, time_window_size int64, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with ExpoHistogram Count===================>\n")
	w.Flush()
	t2 := int64(time_window_size)

	ehs := make(map[string](*EfficientSum))
	for _, c := range cases {
		ehs[c.key] = NewEfficientSum(item_window_size, time_window_size, epsilon, 5) // value_scale)
	}

	EHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerSum2, 0)
	}
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F)
			if t >= t2 {
				start := time.Now()
				ans_sum := ehs[c.key].Query(t-t2, t, false)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerSum2{sum: ans_sum, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_sum, query_time(us), memory(KB)\n", c.key)
		w.Flush()

		var (
			total_error_sum float64 = 0
			total_time      float64 = 0
			total_memory    float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ {
			error_sum := AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			if i%100 == 0 {
				// fmt.Fprintf(w, "%f,%f,%f,%f\n", error_count, error_sum, error_sum2, time)
				w.Flush()
			}
			total_error_sum += error_sum
			total_time += time
			total_memory += memory
		}
		fmt.Fprintf(w, "Average error, time, memory: avg_sum_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_sum/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

func TestEfficientSumInsertAndQuery(t *testing.T) {
	runtime.GOMAXPROCS(40)
	// constructInputTimeSeriesZipf()
	// readPowerDataset()
	readGoogleClusterData2009()
	fmt.Println("finished construct input time series")

	epsilon_input := []float64{0.2, 0.1, 0.05, 0.01, 0.005, 0.001, 0.0001, 0.00001}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		item_window_size := time_window_size
		for _, epsilon := range epsilon_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 25 {
					break
				}
			}
			testname := fmt.Sprintf("EfficientSum_%d_%f_google", time_window_size, epsilon)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(epsilon float64, item_window_size int64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				fmt.Printf("=================>Testing epsilon=%f, window_size=%d===================>\n", epsilon, time_window_size)
				w := bufio.NewWriter(f)
				fmt.Fprintf(w, "=================>Testing epsilon=%f, window_size=%d===================>\n", epsilon, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthSUM2(time_window_size, w)
				funcEfficientSum(epsilon, item_window_size, time_window_size, GroundTruth, w)
				w.Flush()
				atomic.AddUint64(&ops, -add)
				wg.Done()
			}(epsilon, item_window_size, time_window_size)
		}
	}
	wg.Wait()
}

func funcEfficientSumSubWindow(epsilon float64, item_window_size int64, time_window_size int64, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "=================>Estimating with Efficient Sum===================>\n")
	w.Flush()
	t1 := int64(start_t)
	t2 := int64(end_t)

	ehs := make(map[string](*EfficientSum))
	for _, c := range cases {
		ehs[c.key] = NewEfficientSum(item_window_size, time_window_size, epsilon, value_scale)
	}

	EHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerSum2, 0)
	}
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			ehs[c.key].Insert(c.vec[t].T, c.vec[t].F)
			if t >= time_window_size {
				start := time.Now()
				ans_sum := ehs[c.key].Query(t-t2, t-t1, true)
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerSum2{sum: ans_sum, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_sum, query_time(us), memory(KB)\n", c.key)
		w.Flush()

		var (
			total_error_sum float64 = 0
			total_time      float64 = 0
			total_memory    float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ {
			error_sum := AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			if i%100 == 0 {
				// fmt.Fprintf(w, "%f,%f,%f,%f\n", error_count, error_sum, error_sum2, time)
				w.Flush()
			}
			total_error_sum += error_sum
			total_time += time
			total_memory += memory
		}
		fmt.Fprintf(w, "Average error, time, memory: avg_sum_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_sum/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

func TestEfficientSumInsertAndQuerySubWindow(t *testing.T) {
	runtime.GOMAXPROCS(40)
	// constructInputTimeSeriesZipf()
	// readPowerDataset()
	// constructInputTimeSeriesUniformFloat64()
	readPowerDataset()
	// constructInputTimeSeriesUniformFloat64()
	// readGoogleClusterData2009()
	fmt.Println("finished construct input time series")

	epsilon_input := []float64{0.2, 0.1, 0.05, 0.01, 0.005, 0.001, 0.0001, 0.00001}
	time_window_size_input := []int64{1000000}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		item_window_size := time_window_size
		for _, subwindow_size := range subwindow_size_input {
			for _, epsilon := range epsilon_input {
				atomic.AddUint64(&ops, add)
				for {
					if ops < 40 {
						break
					}
				}
				testname := fmt.Sprintf("EfficientSum_%d_%f_%d_%d_power", time_window_size, epsilon, time_window_size-subwindow_size.start, time_window_size-subwindow_size.end)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(epsilon float64, item_window_size int64, time_window_size int64, start_t, end_t int64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					fmt.Printf("=================>Testing epsilon=%f, window_size=%d, start_t=%d, end_t=%d===================>\n", epsilon, time_window_size, start_t, end_t)
					w := bufio.NewWriter(f)
					fmt.Fprintf(w, "=================>Testing epsilon=%f, window_size=%d, start_t=%d, end_t=%d===================>\n", epsilon, time_window_size, start_t, end_t)
					w.Flush()
					GroundTruth := calcGroundTruthSUM2SubWindow(time_window_size, w, start_t, end_t)
					funcEfficientSumSubWindow(epsilon, item_window_size, time_window_size, GroundTruth, w, start_t, end_t)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done epsilon=%f, window_size=%d, start_t=%d, end_t=%d===================>\n", epsilon, time_window_size, start_t, end_t)
					wg.Done()
				}(epsilon, item_window_size, time_window_size, subwindow_size.start, subwindow_size.end)
			}
		}
	}
	wg.Wait()
}
