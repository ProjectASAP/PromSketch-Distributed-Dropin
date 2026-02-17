package promsketch

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/praserx/ipconv"
)

func TestSHUnivPool(t *testing.T) {
	t_now := time.Now()
	shu := SmoothInitUnivMon(0.06, 100000)
	since := time.Since(t_now)
	fmt.Println("smooth init univ total time =", since)
	fmt.Println("smooth init univ each time =", since/time.Duration(UnivPoolCAP))

	var total_insert int64 = 5000
	var total_since float64 = 0
	for t_idx := int64(0); t_idx < total_insert; t_idx++ {
		value := float64(0)
		for {
			value = rand.NormFloat64()*stdDev + stdMean
			if value >= 0 && value <= value_scale {
				break
			}
		}

		t_now := time.Now()
		shu.Update(t_idx, strconv.FormatFloat(value, 'f', -1, 64)) // insert data to univmon for current timeseries
		since := time.Since(t_now)
		total_since += since.Seconds()
		//fmt.Println("shu insertion time =", since)
	}
	fmt.Println("average shu insertion time (ms)=", total_since/float64(total_insert)*1000)
}

// TODO: changes_univmon, topk_over_time

type AnswerSum struct {
	count float64
	sum   float64
}

/*
func calcGroundTruth() (map[string]([]Answer)) {
	t1 := int64(time_window_size/3)
	t2 := int64(time_window_size/3 *2)
	m := float64(t2 - t1)

	// Ground Truth
	GroundTruth := make(map[string]([]Answer))
	for _, c := range cases {
		GroundTruth[c.key] = make([]Answer, 0)
	}
	for t := t2; t < time_window_size * 2; t++ {
		cardMap := make(map[string]float64)
		sumMap := make(map[string]float64)
		entropyMap := make(map[string]float64)
		l2Map := make(map[string]float64)
		for tt := t - t2; tt < t - t1; tt++ {
			for _, c := range cases {
				if _, ok := cardMap[c.key]; ok {
					cardMap[c.key] += 1
				} else {
					cardMap[c.key] = 1
				}
				if _, ok := sumMap[c.key]; ok {
					sumMap[c.key] += c.vec[tt].F
				} else {
					sumMap[c.key] = c.vec[tt].F
				}
				if _, ok := entropyMap[c.key]; ok {
					entropyMap[c.key] += c.vec[tt].F * math.Log(c.vec[tt].F)
				} else {
					entropyMap[c.key] = c.vec[tt].F * math.Log(c.vec[tt].F)
				}
				if _, ok := l2Map[c.key]; ok {
					l2Map[c.key] += c.vec[tt].F * c.vec[tt].F
				} else {
					l2Map[c.key] = c.vec[tt].F * c.vec[tt].F
				}
			}
		}
		for _, c := range cases {
			GroundTruth[c.key] = append(GroundTruth[c.key], Answer{card: cardMap[c.key],
						sum: sumMap[c.key],
						entropy: math.Log(m) / math.Log(2) - entropyMap[c.key] / m,
						l2: l2Map[c.key],})
		}
	}
	return GroundTruth
}
*/

// TODO: test for changes
func TestSmoothHistogramChanges(t *testing.T) {

}

func funcSmoothHistogramCount(beta float64, time_window_size int64, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with SmoothHistogram Count===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)

	sh := make(map[string](*SmoothHistogramCount))
	for _, c := range cases {
		sh[c.key] = SmoothInitCount(beta, time_window_size)
	}

	SHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		SHAnswer[c.key] = make([]AnswerSum2, 0)
	}

	var total_elapsed float64 = 0
	for t := int64(0); t < time_window_size*2; t++ {
		var total_insert float64 = 0
		for _, c := range cases {
			start_insert := time.Now()
			sh[c.key].Update(c.vec[t].T, c.vec[t].F)
			elapsed_insert := time.Since(start_insert)
			total_insert += float64(elapsed_insert.Microseconds())
			if t >= t2 {
				start := time.Now()
				count := sh[c.key].QueryT1T2IntervalCount(t-t2, t-t1, t)
				sum := sh[c.key].QueryT1T2IntervalSum(t-t2, t-t1, t)
				sum2 := sh[c.key].QueryT1T2IntervalSum2(t-t2, t-t1, t)
				elapsed := time.Since(start)
				total_elapsed += float64(elapsed.Microseconds())
				SHAnswer[c.key] = append(SHAnswer[c.key], AnswerSum2{count: count, sum: sum, sum2: sum2, time: float64(elapsed.Microseconds()), memory: sh[c.key].GetMemory()})
			}
		}
		if t%10000 == 0 {
			// fmt.Fprintf(w, "At %d time\n", t)
			w.Flush()
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%s error_count, error_sum, error_sum2\n", c.key)
		w.Flush()
		// assert.Equal(t, len(GroundTruth[c.key]), len(SHAnswer[c.key]), "the answer length should be the same.")

		var (
			total_error_count float64 = 0
			total_error_sum   float64 = 0
			total_error_sum2  float64 = 0
			total_time        float64 = 0
			total_memory      float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ {
			// fmt.Println("count:", GroundTruth[c.key][i].count, SHAnswer[c.key][i].count)
			// fmt.Println("sum:", GroundTruth[c.key][i].sum, SHAnswer[c.key][i].sum)
			// fmt.Println("sum2:", GroundTruth[c.key][i].sum2, SHAnswer[c.key][i].sum2)

			error_count := AbsFloat64(GroundTruth[c.key][i].count-SHAnswer[c.key][i].count) / GroundTruth[c.key][i].count
			error_sum := AbsFloat64(GroundTruth[c.key][i].sum-SHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			error_sum2 := AbsFloat64(GroundTruth[c.key][i].sum2-SHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2
			time := SHAnswer[c.key][i].time
			memory := SHAnswer[c.key][i].memory

			// fmt.Fprintf(w, "%f,%f,%f,%f\n", error_count, error_sum, error_sum2, time)
			if i%100 == 0 {
				w.Flush()
			}
			total_error_count += error_count
			total_error_sum += error_sum
			total_error_sum2 += error_sum2
			total_time += time
			total_memory += memory
			/*
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].count-SHAnswer[c.key][i].count)/GroundTruth[c.key][i].count < 0.05 {
						return true
					} else {
						return false
					}
				}, "Card error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum-SHAnswer[c.key][i].sum)/GroundTruth[c.key][i].sum < 0.05 {
						return true
					} else {
						return false
					}
				}, "Sum error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum2-SHAnswer[c.key][i].sum2)/GroundTruth[c.key][i].sum2 < 0.05 {
						return true
					} else {
						return false
					}
				}, "Sum2 error too large")
			*/
		}
		fmt.Fprintf(w, "Average error and time: avg_count_error: %f%%, avg_sum_error: %f%%, avg_sum2_error: %f%%, avg_time: %f(us), avg_memory: %f (KB)\n", total_error_count/float64(len(GroundTruth[c.key]))*100, total_error_sum/float64(len(GroundTruth[c.key]))*100, total_error_sum2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

// test count_over_time, sum_over_time, avg_over_time, zscore_over_time, holt_winter, predict_linear
func TestSmoothHistogramCount(t *testing.T) {
	constructInputTimeSeriesZipf()
	fmt.Println("finished construct input time series")
	runtime.GOMAXPROCS(40)

	beta_input := []float64{1.0, 0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, beta := range beta_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 40 {
					break
				}
			}
			testname := fmt.Sprintf("SHCount_%d_%f", time_window_size, beta)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(beta float64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				fmt.Fprintf(w, "=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthSUM2(time_window_size, w)
				funcSmoothHistogramCount(beta, time_window_size, GroundTruth, w)
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done SHCount beta=%f, window_size=%d===================>\n", beta, time_window_size)
				wg.Done()
			}(beta, time_window_size)
		}
	}
	wg.Wait()
}

// test for sum2_over_time, stdvar_over_time, stddev_over_time
/*
func TestSmoothHistogramSUM2(t * testing.T) {
	t1 := int64(time_window_size/3)
	t2 := int64(time_window_size/3 *2)

	constructInputTimeSeries()
	GroundTruth := calcGroundTruthSUM2()

	seed1 := make([]uint32, CM_ROW_NO)
	rand.Seed(time.Now().UnixNano())
	for r := 0; r < CM_ROW_NO; r++ {
		seed1[r] = rand.Uint32()
	}

	var beta float64 = 0.1
	sh := SmoothInitSum2(beta, seed1, time_window_size)

	SHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		SHAnswer[c.key] = make([]AnswerSum2, 0)
	}

	for t := int64(0); t < time_window_size * 2; t++ {
		for _, c := range cases {
			sh.SmoothUpdateSum2(c.key, c.vec[t].T, c.vec[t].F)
			if t >= t2 {
				count := sh.QueryT1T2IntervalCount(c.key, t - t2, t - t1)
				sum := sh.QueryT1T2IntervalSum(c.key, t - t2, t - t1)
				sum2 := sh.QueryT1T2IntervalSum2(c.key, t - t2, t - t1)
				SHAnswer[c.key] = append(SHAnswer[c.key], AnswerSum2{count: count, sum: sum, sum2: sum2})
			}
		}
	}

	for _, c := range cases {
		fmt.Println(c.key)
		assert.Equal(t, len(GroundTruth[c.key]), len(SHAnswer[c.key]), "the answer length should be the same.")

		for i := 0; i < len(GroundTruth[c.key]); i++ {
			fmt.Println("count:", GroundTruth[c.key][i].count, SHAnswer[c.key][i].count)
			fmt.Println("sum:", GroundTruth[c.key][i].sum, SHAnswer[c.key][i].sum)
			fmt.Println("sum2:", GroundTruth[c.key][i].sum2, SHAnswer[c.key][i].sum2)

			assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].count - SHAnswer[c.key][i].count) / GroundTruth[c.key][i].count < 0.05 {
						return true
					} else {
						return false
					}
				}, "Card error too large")
			assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum - SHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum < 0.05 {
						return true
					} else {
						return false
					}
				}, "Sum error too large")
			assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum2 - SHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2 < 0.05 {
						return true
					} else {
						return false
					}
				}, "Sum2 error too large")
		}
	}
}
*/

func SmoothHistogramUniv(beta float64, time_window_size int64, GroundTruth map[string]([]AnswerUniv), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with SmoothHistogram UnivMon===================>\n")
	w.Flush()
	// t1 := int64(time_window_size / 3)
	// t2 := int64(time_window_size / 3 * 2)
	t1 := int64(0)
	t2 := int64(time_window_size)
	// m := float64(t2 - t1)

	// one SH+Univ for one timeseries
	shs := make(map[string](*SmoothHistogramUnivMon))
	for _, c := range cases {
		shs[c.key] = SmoothInitUnivMon(beta, time_window_size)
	}

	SHAnswer := make(map[string]([]AnswerUniv))
	for _, c := range cases {
		SHAnswer[c.key] = make([]AnswerUniv, 0)
	}

	// for debugging just one query
	// raw_data := make([]string, 0)

	var total_insert_time float64 = 0
	var total_insert float64 = 0
	// for t := int64(0); t < int64(time_window_size/2*3); t++ {
	for t := int64(0); t < int64(time_window_size/2*3); t++ {
		for _, c := range cases {
			start := time.Now()
			shs[c.key].Update(c.vec[t].T, strconv.FormatFloat(c.vec[t].F, 'f', -1, 64)) // insert data to univmon for current timeseries
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_insert += 1
			// fmt.Println("avg insert time= (us)", total_insert_time/total_insert)
			if t >= t2 {
				start := time.Now()
				merged_univ, _ := shs[c.key].QueryIntervalMergeUniv(t-t2, t-t1, t)
				card := merged_univ.calcCard()
				l1 := merged_univ.calcL1()
				l2 := merged_univ.calcL2()
				l2 = math.Sqrt(l2)
				entropynorm := merged_univ.calcEntropy()
				// 	entropy := math.Log2(m) - entropynorm/m

				elapsed := time.Since(start)
				// fmt.Println("query time=", elapsed)
				SHAnswer[c.key] = append(SHAnswer[c.key], AnswerUniv{card: card, l1: l1, l2: l2, entropy: entropynorm, time: float64(elapsed.Microseconds()), memory: shs[c.key].GetMemory()})
				// fmt.Fprintf(w, "time %d, card %f, l1 %f, entropynorm %f, l2 %f\n", t, card, l1, entropynorm, l2)
				// fmt.Fprintf(w, "bucket num %d\n", shs[c.key].s_count)
			}
		}
	}
	w.Flush()

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_card, error_l1, error_l2, error_entropy, gt_time(us), sketch_time(us)\n", c.key)
		w.Flush()
		// assert.Equal(t, len(GroundTruth[c.key]), len(SHAnswer[c.key]), "the answer length should be the same.")
		var (
			total_error_card    float64   = 0
			total_error_l1      float64   = 0
			total_error_l2      float64   = 0
			total_error_entropy float64   = 0
			total_time          float64   = 0
			total_sketch_time   float64   = 0
			total_memory        float64   = 0
			card_error_arr      []float64 = make([]float64, 0)
			l1_error_arr        []float64 = make([]float64, 0)
			l2_error_arr        []float64 = make([]float64, 0)
			entropy_error_arr   []float64 = make([]float64, 0)
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ { // time dimension
			error_card := AbsFloat64(GroundTruth[c.key][i].card-SHAnswer[c.key][i].card) / GroundTruth[c.key][i].card
			error_l1 := AbsFloat64(GroundTruth[c.key][i].l1-SHAnswer[c.key][i].l1) / GroundTruth[c.key][i].l1
			error_l2 := AbsFloat64(GroundTruth[c.key][i].l2-SHAnswer[c.key][i].l2) / GroundTruth[c.key][i].l2
			error_entropy := float64(0)
			if GroundTruth[c.key][i].entropy != 0 {
				error_entropy = AbsFloat64(GroundTruth[c.key][i].entropy-SHAnswer[c.key][i].entropy) / GroundTruth[c.key][i].entropy
			}
			// fmt.Fprintf(w, "%f,%f,%f,%f,%f,%f\n", error_card, error_l1, error_l2, error_entropy, GroundTruth[c.key][i].time, SHAnswer[c.key][i].time)
			if i%100 == 0 {
				w.Flush()
			}
			l1_error_arr = append(l1_error_arr, error_l1)
			card_error_arr = append(card_error_arr, error_card)
			entropy_error_arr = append(entropy_error_arr, error_entropy)
			l2_error_arr = append(l2_error_arr, error_l2)

			total_error_card += error_card
			total_error_l1 += error_l1
			total_error_l2 += error_l2
			total_error_entropy += error_entropy
			total_time += GroundTruth[c.key][i].time
			total_sketch_time += SHAnswer[c.key][i].time
			total_memory += SHAnswer[c.key][i].memory
		}

		if len(GroundTruth[c.key]) > 0 {
			fmt.Fprintf(w, "Average error and time: avg_card_error: %f%%, avg_l1_error: %f%%, avg_entropy_error: %f%%, avg_l2_error: %f%%, avg_gt_time: %f(us), avg_sketch_time: %f(us), avg_memory: %f(KB)\n", total_error_card/float64(len(GroundTruth[c.key]))*100, total_error_l1/float64(len(GroundTruth[c.key]))*100, total_error_entropy/float64(len(GroundTruth[c.key]))*100, total_error_l2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_sketch_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
			fmt.Fprintf(w, "Average sketch insert time: %f(us)\n", total_insert_time/total_insert)
			fmt.Fprintf(w, "Median error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Median(card_error_arr)*100, Median(l1_error_arr)*100, Median(entropy_error_arr)*100, Median(l2_error_arr)*100)
			fmt.Fprintf(w, "Max error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Max(card_error_arr)*100, Max(l1_error_arr)*100, Max(entropy_error_arr)*100, Max(l2_error_arr)*100)
			fmt.Fprintf(w, "Min error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Min(card_error_arr)*100, Min(l1_error_arr)*100, Min(entropy_error_arr)*100, Min(l2_error_arr)*100)
			w.Flush()
		}
	}
	w.Flush()
}

func SmoothHistogramUnivSubWindow(beta float64, time_window_size int64, GroundTruth map[string]([]AnswerUniv), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "=================>Estimating with SmoothHistogram UnivMon===================>\n")
	w.Flush()
	// t1 := int64(time_window_size / 3)
	// t2 := int64(time_window_size / 3 * 2)
	t1 := int64(start_t)
	t2 := int64(end_t)
	// m := float64(t2 - t1)

	// one SH+Univ for one timeseries
	shs := make(map[string](*SmoothHistogramUnivMon))
	for _, c := range cases {
		shs[c.key] = SmoothInitUnivMon(beta, time_window_size)
	}

	SHAnswer := make(map[string]([]AnswerUniv))
	for _, c := range cases {
		SHAnswer[c.key] = make([]AnswerUniv, 0)
	}

	// for debugging just one query
	// raw_data := make([]string, 0)

	var total_insert_time float64 = 0
	var total_insert float64 = 0
	// for t := int64(0); t < int64(time_window_size/2*3); t++ {
	for t := int64(0); t < int64(time_window_size+100); t++ {
		for _, c := range cases {
			start := time.Now()
			shs[c.key].Update(c.vec[t].T, strconv.FormatFloat(c.vec[t].F, 'f', -1, 64)) // insert data to univmon for current timeseries
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_insert += 1
			// fmt.Println("avg insert time= (us)", total_insert_time/total_insert)
			if t >= time_window_size {
				start := time.Now()
				merged_univ, _ := shs[c.key].QueryIntervalMergeUniv(t-t2, t-t1, t)
				card := merged_univ.calcCard()
				l1 := merged_univ.calcL1()
				l2 := merged_univ.calcL2()
				l2 = math.Sqrt(l2)
				entropynorm := merged_univ.calcEntropy()
				// 	entropy := math.Log2(m) - entropynorm/m

				elapsed := time.Since(start)
				// fmt.Println("query time=", elapsed)
				SHAnswer[c.key] = append(SHAnswer[c.key], AnswerUniv{card: card, l1: l1, l2: l2, entropy: entropynorm, time: float64(elapsed.Microseconds()), memory: shs[c.key].GetMemory()})
				fmt.Fprintf(w, "time %d, card %f, l1 %f, entropynorm %f, l2 %f\n", t, card, l1, entropynorm, l2)
				fmt.Fprintf(w, "bucket num %d\n", shs[c.key].s_count)
			}
		}
	}
	w.Flush()

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_card, error_l1, error_l2, error_entropy, gt_time(us), sketch_time(us)\n", c.key)
		w.Flush()
		// assert.Equal(t, len(GroundTruth[c.key]), len(SHAnswer[c.key]), "the answer length should be the same.")
		var (
			total_error_card    float64   = 0
			total_error_l1      float64   = 0
			total_error_l2      float64   = 0
			total_error_entropy float64   = 0
			total_time          float64   = 0
			total_sketch_time   float64   = 0
			total_memory        float64   = 0
			card_error_arr      []float64 = make([]float64, 0)
			l1_error_arr        []float64 = make([]float64, 0)
			l2_error_arr        []float64 = make([]float64, 0)
			entropy_error_arr   []float64 = make([]float64, 0)
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ { // time dimension
			error_card := AbsFloat64(GroundTruth[c.key][i].card-SHAnswer[c.key][i].card) / GroundTruth[c.key][i].card
			error_l1 := AbsFloat64(GroundTruth[c.key][i].l1-SHAnswer[c.key][i].l1) / GroundTruth[c.key][i].l1
			error_l2 := AbsFloat64(GroundTruth[c.key][i].l2-SHAnswer[c.key][i].l2) / GroundTruth[c.key][i].l2
			error_entropy := float64(0)
			if GroundTruth[c.key][i].entropy != 0 {
				error_entropy = AbsFloat64(GroundTruth[c.key][i].entropy-SHAnswer[c.key][i].entropy) / GroundTruth[c.key][i].entropy
			}
			// fmt.Fprintf(w, "%f,%f,%f,%f,%f,%f\n", error_card, error_l1, error_l2, error_entropy, GroundTruth[c.key][i].time, SHAnswer[c.key][i].time)
			if i%100 == 0 {
				w.Flush()
			}
			l1_error_arr = append(l1_error_arr, error_l1)
			card_error_arr = append(card_error_arr, error_card)
			entropy_error_arr = append(entropy_error_arr, error_entropy)
			l2_error_arr = append(l2_error_arr, error_l2)

			total_error_card += error_card
			total_error_l1 += error_l1
			total_error_l2 += error_l2
			total_error_entropy += error_entropy
			total_time += GroundTruth[c.key][i].time
			total_sketch_time += SHAnswer[c.key][i].time
			total_memory += SHAnswer[c.key][i].memory
		}

		if len(GroundTruth[c.key]) > 0 {
			fmt.Fprintf(w, "Average error and time: avg_card_error: %f%%, avg_l1_error: %f%%, avg_entropy_error: %f%%, avg_l2_error: %f%%, avg_gt_time: %f(us), avg_sketch_time: %f(us), avg_memory: %f(KB)\n", total_error_card/float64(len(GroundTruth[c.key]))*100, total_error_l1/float64(len(GroundTruth[c.key]))*100, total_error_entropy/float64(len(GroundTruth[c.key]))*100, total_error_l2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_sketch_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
			fmt.Fprintf(w, "Average sketch insert time: %f(us)\n", total_insert_time/total_insert)
			fmt.Fprintf(w, "Median error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Median(card_error_arr)*100, Median(l1_error_arr)*100, Median(entropy_error_arr)*100, Median(l2_error_arr)*100)
			fmt.Fprintf(w, "Max error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Max(card_error_arr)*100, Max(l1_error_arr)*100, Max(entropy_error_arr)*100, Max(l2_error_arr)*100)
			fmt.Fprintf(w, "Min error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Min(card_error_arr)*100, Min(l1_error_arr)*100, Min(entropy_error_arr)*100, Min(l2_error_arr)*100)
			w.Flush()
		}
	}
	w.Flush()
}

func SmoothHistogramUnivTopKOnly(beta float64, time_window_size int64, GroundTruth map[string]([]AnswerUniv), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with SmoothHistogram UnivMon===================>\n")
	w.Flush()
	// t1 := int64(time_window_size / 3)
	// t2 := int64(time_window_size / 3 * 2)
	t1 := int64(0)
	t2 := int64(time_window_size)
	// m := float64(t2 - t1)

	// one SH+Univ for one timeseries
	shs := make(map[string](*SmoothHistogramUnivMon))
	for _, c := range cases {
		shs[c.key] = SmoothInitUnivMon(beta, time_window_size)
	}

	SHAnswer := make(map[string]([]AnswerUniv))
	for _, c := range cases {
		SHAnswer[c.key] = make([]AnswerUniv, 0)
	}

	// for debugging just one query
	// raw_data := make([]string, 0)

	var total_insert_time float64 = 0
	var total_insert float64 = 0
	// for t := int64(0); t < int64(time_window_size/2*3); t++ {
	for t := int64(0); t < int64(time_window_size+100); t++ {
		for _, c := range cases {
			start := time.Now()
			shs[c.key].Update(c.vec[t].T, strconv.FormatFloat(c.vec[t].F, 'f', -1, 64)) // insert data to univmon for current timeseries
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_insert += 1
			// fmt.Println("avg insert time= (us)", total_insert_time/total_insert)
			if t >= t2 {
				start := time.Now()
				merged_univ, _ := shs[c.key].QueryIntervalMergeUniv(t-t2, t-t1, t)
				card := merged_univ.calcCard()
				l1 := merged_univ.calcL1()
				l2 := merged_univ.calcL2()
				l2 = math.Sqrt(l2)
				entropynorm := merged_univ.calcEntropy()
				// 	entropy := math.Log2(m) - entropynorm/m

				elapsed := time.Since(start)
				// fmt.Println("query time=", elapsed)
				SHAnswer[c.key] = append(SHAnswer[c.key], AnswerUniv{card: card, l1: l1, l2: l2, entropy: entropynorm, time: float64(elapsed.Microseconds()), memory: shs[c.key].GetMemory()})
				fmt.Fprintf(w, "time %d, card %f, l1 %f, entropynorm %f, l2 %f\n", t, card, l1, entropynorm, l2)
				fmt.Fprintf(w, "bucket num %d\n", shs[c.key].s_count)
			}
		}
	}
	w.Flush()

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_card, error_l1, error_l2, error_entropy, gt_time(us), sketch_time(us)\n", c.key)
		w.Flush()
		// assert.Equal(t, len(GroundTruth[c.key]), len(SHAnswer[c.key]), "the answer length should be the same.")
		var (
			total_error_card    float64   = 0
			total_error_l1      float64   = 0
			total_error_l2      float64   = 0
			total_error_entropy float64   = 0
			total_time          float64   = 0
			total_sketch_time   float64   = 0
			total_memory        float64   = 0
			card_error_arr      []float64 = make([]float64, 0)
			l1_error_arr        []float64 = make([]float64, 0)
			l2_error_arr        []float64 = make([]float64, 0)
			entropy_error_arr   []float64 = make([]float64, 0)
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ { // time dimension
			error_card := AbsFloat64(GroundTruth[c.key][i].card-SHAnswer[c.key][i].card) / GroundTruth[c.key][i].card
			error_l1 := AbsFloat64(GroundTruth[c.key][i].l1-SHAnswer[c.key][i].l1) / GroundTruth[c.key][i].l1
			error_l2 := AbsFloat64(GroundTruth[c.key][i].l2-SHAnswer[c.key][i].l2) / GroundTruth[c.key][i].l2
			error_entropy := float64(0)
			if GroundTruth[c.key][i].entropy != 0 {
				error_entropy = AbsFloat64(GroundTruth[c.key][i].entropy-SHAnswer[c.key][i].entropy) / GroundTruth[c.key][i].entropy
			}
			// fmt.Fprintf(w, "%f,%f,%f,%f,%f,%f\n", error_card, error_l1, error_l2, error_entropy, GroundTruth[c.key][i].time, SHAnswer[c.key][i].time)
			if i%100 == 0 {
				w.Flush()
			}
			l1_error_arr = append(l1_error_arr, error_l1)
			card_error_arr = append(card_error_arr, error_card)
			entropy_error_arr = append(entropy_error_arr, error_entropy)
			l2_error_arr = append(l2_error_arr, error_l2)

			total_error_card += error_card
			total_error_l1 += error_l1
			total_error_l2 += error_l2
			total_error_entropy += error_entropy
			total_time += GroundTruth[c.key][i].time
			total_sketch_time += SHAnswer[c.key][i].time
			total_memory += SHAnswer[c.key][i].memory
			/*
				fmt.Println("card:", GroundTruth[c.key][i].card, SHAnswer[c.key][i].card, AbsFloat64(GroundTruth[c.key][i].card-SHAnswer[c.key][i].card)/GroundTruth[c.key][i].card)
				fmt.Println("l1:", GroundTruth[c.key][i].l1, SHAnswer[c.key][i].l1, AbsFloat64(GroundTruth[c.key][i].l1-SHAnswer[c.key][i].l1)/GroundTruth[c.key][i].l1)
				fmt.Println("l2:", GroundTruth[c.key][i].l2, SHAnswer[c.key][i].l2, AbsFloat64(GroundTruth[c.key][i].l2-SHAnswer[c.key][i].l2)/GroundTruth[c.key][i].l2)
				fmt.Println("entropy:", GroundTruth[c.key][i].entropy, SHAnswer[c.key][i].entropy, AbsFloat64(GroundTruth[c.key][i].entropy-SHAnswer[c.key][i].entropy)/GroundTruth[c.key][i].entropy)
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].card-SHAnswer[c.key][i].card)/GroundTruth[c.key][i].card < 0.05 {
						return true
					} else {
						return false
					}
				}, "Card error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].l1-SHAnswer[c.key][i].l1)/GroundTruth[c.key][i].l1 < 0.05 {
						return true
					} else {
						return false
					}
				}, "L1 error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].l2-SHAnswer[c.key][i].l2)/GroundTruth[c.key][i].l2 < 0.05 {
						return true
					} else {
						return false
					}
				}, "L2 error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].entropy-SHAnswer[c.key][i].entropy)/GroundTruth[c.key][i].entropy < 0.05 {
						return true
					} else {
						return false
					}
				}, "Entropy error too large")
			*/
		}

		if len(GroundTruth[c.key]) > 0 {
			fmt.Fprintf(w, "Average error and time: avg_card_error: %f%%, avg_l1_error: %f%%, avg_entropy_error: %f%%, avg_l2_error: %f%%, avg_gt_time: %f(us), avg_sketch_time: %f(us), avg_memory: %f(KB)\n", total_error_card/float64(len(GroundTruth[c.key]))*100, total_error_l1/float64(len(GroundTruth[c.key]))*100, total_error_entropy/float64(len(GroundTruth[c.key]))*100, total_error_l2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_sketch_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
			fmt.Fprintf(w, "Average sketch insert time: %f(us)\n", total_insert_time/total_insert)
			fmt.Fprintf(w, "Median error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Median(card_error_arr)*100, Median(l1_error_arr)*100, Median(entropy_error_arr)*100, Median(l2_error_arr)*100)
			fmt.Fprintf(w, "Max error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Max(card_error_arr)*100, Max(l1_error_arr)*100, Max(entropy_error_arr)*100, Max(l2_error_arr)*100)
			fmt.Fprintf(w, "Min error: card: %f%%, l1: %f%%, entropy: %f%%, l2: %f%%\n", Min(card_error_arr)*100, Min(l1_error_arr)*100, Min(entropy_error_arr)*100, Min(l2_error_arr)*100)
			w.Flush()
		}
	}
	w.Flush()
}

func calcGroundTruthUnivTopKOnly(time_window_size int64, w *bufio.Writer) map[string]([]AnswerUniv) {
	fmt.Fprintf(w, "=================>Calculating Smooth Histogram UnivMon GroundTruth===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)
	// m := float64(t2 - t1)

	// Ground Truth
	GroundTruth := make(map[string]([]AnswerUniv))

	for _, c := range cases {
		GroundTruth[c.key] = make([]AnswerUniv, 0) // c.key is the timeseries name
	}
	for t := t2; t < int64(time_window_size+100); t++ {
		for _, c := range cases {
			start := time.Now()
			l1Map := make(map[float64]float64)
			for tt := t - t2; tt < t-t1; tt++ {
				if _, ok := l1Map[c.vec[tt].F]; ok {
					l1Map[c.vec[tt].F] += 1
				} else {
					l1Map[c.vec[tt].F] = 1
				}
			}
			var l1 float64 = 0.0
			var l2 float64 = 0.0
			var entropynorm float64 = 0.0

			keys := make([]float64, 0, len(l1Map))

			for key := range l1Map {
				keys = append(keys, key)
			}

			sort.SliceStable(keys, func(i, j int) bool {
				return l1Map[keys[i]] > l1Map[keys[j]]
			})

			for _, key := range keys {
				// if i >= 500 {
				// 	break
				// }
				value := l1Map[key]
				l1 += value
				entropynorm += value * math.Log2(value)
				l2 += value * value
			}

			/*
				for _, value := range l1Map {
					l1 += value
					entropynorm += value * math.Log2(value)
					l2 += value * value
				}
			*/
			elapsed := time.Since(start)
			GroundTruth[c.key] = append(GroundTruth[c.key], AnswerUniv{card: float64(len(l1Map)),
				l1:      l1,
				entropy: entropynorm, // math.Log2(m) - entropynorm/m,
				l2:      math.Sqrt(l2),
				time:    float64(elapsed.Microseconds())})
		}
	}
	return GroundTruth
}

func calcGroundTruthUniv(time_window_size int64, w *bufio.Writer) map[string]([]AnswerUniv) {
	fmt.Fprintf(w, "=================>Calculating Smooth Histogram UnivMon GroundTruth===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)
	// m := float64(t2 - t1)

	// Ground Truth
	GroundTruth := make(map[string]([]AnswerUniv))

	for _, c := range cases {
		GroundTruth[c.key] = make([]AnswerUniv, 0) // c.key is the timeseries name
	}
	for t := t2; t < int64(time_window_size/2*3); t++ {
		for _, c := range cases {
			start := time.Now()
			l1Map := make(map[float64]float64)
			for tt := t - t2; tt < t-t1; tt++ {
				if _, ok := l1Map[c.vec[tt].F]; ok {
					l1Map[c.vec[tt].F] += 1
				} else {
					l1Map[c.vec[tt].F] = 1
				}
			}
			var l1 float64 = 0.0
			var l2 float64 = 0.0
			var entropynorm float64 = 0.0
			for _, value := range l1Map {
				l1 += value
				entropynorm += value * math.Log2(value)
				l2 += value * value
			}
			elapsed := time.Since(start)
			GroundTruth[c.key] = append(GroundTruth[c.key], AnswerUniv{card: float64(len(l1Map)),
				l1:      l1,
				entropy: entropynorm, // math.Log2(m) - entropynorm/m,
				l2:      math.Sqrt(l2),
				time:    float64(elapsed.Microseconds())})
		}
	}
	return GroundTruth
}

func calcGroundTruthUnivSubWindow(time_window_size int64, w *bufio.Writer, start_t, end_t int64) map[string]([]AnswerUniv) {
	fmt.Fprintf(w, "=================>Calculating Smooth Histogram UnivMon GroundTruth===================>\n")
	w.Flush()
	t1 := int64(start_t)
	t2 := int64(end_t)
	// m := float64(t2 - t1)

	// Ground Truth
	GroundTruth := make(map[string]([]AnswerUniv))

	for _, c := range cases {
		GroundTruth[c.key] = make([]AnswerUniv, 0) // c.key is the timeseries name
	}
	for t := time_window_size; t < int64(time_window_size+100); t++ {
		for _, c := range cases {
			start := time.Now()
			l1Map := make(map[float64]float64)
			for tt := t - t2; tt < t-t1; tt++ {
				if _, ok := l1Map[c.vec[tt].F]; ok {
					l1Map[c.vec[tt].F] += 1
				} else {
					l1Map[c.vec[tt].F] = 1
				}
			}
			var l1 float64 = 0.0
			var l2 float64 = 0.0
			var entropynorm float64 = 0.0
			for _, value := range l1Map {
				l1 += value
				entropynorm += value * math.Log2(value)
				l2 += value * value
			}
			elapsed := time.Since(start)
			GroundTruth[c.key] = append(GroundTruth[c.key], AnswerUniv{card: float64(len(l1Map)),
				l1:      l1,
				entropy: entropynorm, // math.Log2(m) - entropynorm/m,
				l2:      math.Sqrt(l2),
				time:    float64(elapsed.Microseconds())})
		}
	}
	return GroundTruth
}

func TestSmoothHistogramUnivDebug(t *testing.T) {
	constructInputTimeSeriesUniv()
	fmt.Println("finished construct input time series")
	beta_input := []float64{0.044}
	// beta_input := []float64{0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	time_window_size_input := []int64{1000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, beta := range beta_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 2 {
					break
				}
			}
			testname := fmt.Sprintf("SHUniv_%d_%f", time_window_size, beta)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(beta float64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				fmt.Fprintf(w, "=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				w.Flush()
				// GroundTruth := calcGroundTruthUniv(time_window_size, w)
				GroundTruth := calcGroundTruthUniv(0, w)
				SmoothHistogramUniv(beta, time_window_size, GroundTruth, w)
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done beta=%f, window_size=%d===================>\n", beta, time_window_size)
				wg.Done()
			}(beta, time_window_size)
		}
	}
	wg.Wait()
}

func readCAIDA() {
	vec := make(Vector, 0)
	t := int64(0)
	filename := []string{"./testdata/equinix-nyc.dirA.20181220-130100.UTC.anon.pcap", "./testdata/equinix-nyc.dirA.20181220-130200.UTC.anon.pcap"}
	for i := 0; i < 2; i++ {
		if handle, err := pcap.OpenOffline(filename[i]); err != nil {
			panic(err)
		} else {
			packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
			for packet := range packetSource.Packets() {
				// ethLayer := packet.Layer(layers.LayerTypeEthernet)
				/*
					if ethLayer != nil {
						ethPacket, _ := ethLayer.(*layers.Ethernet)
						fmt.Println("Ethernet source MAC address:", ethPacket.SrcMAC)
						fmt.Println("Ethernet destination MAC address:", ethPacket.DstMAC)
					}
				*/

				// Extract and print the IP layer
				ipLayer := packet.Layer(layers.LayerTypeIPv4)
				if ipLayer != nil {
					t += 1
					ipPacket, _ := ipLayer.(*layers.IPv4)
					srcip, _ := ipconv.IPv4ToInt(ipPacket.SrcIP)
					vec = append(vec, Sample{T: t, F: float64(srcip)})
					// fmt.Println("IP source address:", ipPacket.SrcIP)
					// fmt.Println("IP destination address:", ipPacket.DstIP)
					if t > 2000000 {
						goto exit
					}
				}
			}
		}
	}
exit:
	tmp := TestCase{
		key: "source_ip",
		vec: vec,
	}
	cases = append(cases, tmp)
	fmt.Println("total packet num:", t)
}

func TestReadCAIDA(t *testing.T) {
	readCAIDA()
}

func constructInputTimeSeriesUnivOffline() {
	// Construct input timeseries
	for i := 0; i < total_time_series; i++ {
		key := "machine" + strconv.Itoa(i)
		testname := key
		f, err := os.Open("./testdata/zipf/" + testname + ".txt")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		// const maxCapacity int = 100000000 // your required line length
		// buf := make([]byte, maxCapacity)
		// scanner.Buffer(buf, maxCapacity)
		vec := make(Vector, 0)
		for scanner.Scan() {
			splits := strings.Fields(scanner.Text())
			T, _ := strconv.ParseInt(splits[0], 10, 64)
			F, _ := strconv.ParseFloat(strings.TrimSpace(splits[1]), 64)
			vec = append(vec, Sample{T: T, F: F})
		}
		tmp := TestCase{
			key: key,
			vec: vec,
		}
		cases = append(cases, tmp)
	}
}

func constructInputTimeSeriesUniv() {
	// Construct input timeseries
	for i := 0; i < total_time_series; i++ {
		key := "machine" + strconv.Itoa(i)
		vec := make(Vector, 0)
		var s float64 = 2
		var v float64 = 1
		var RAND *rand.Rand = rand.New(rand.NewSource(time.Now().Unix()))
		z := rand.NewZipf(RAND, s, v, uint64(value_scale/10))
		for t := int64(0); t < time_range; t++ {
			value := float64(int(z.Uint64())) + 1
			// TODO: DDSketch and KLL only works with positive float64 currently
			vec = append(vec, Sample{T: t, F: value})
		}

		tmp := TestCase{
			key: key,
			vec: vec,
		}
		cases = append(cases, tmp)
	}
}

func TestSmoothHistogramUnivTopKOnly(t *testing.T) {
	runtime.GOMAXPROCS(40)
	constructInputTimeSeriesUnivOffline()
	fmt.Println("finished construct input time series")

	beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	// beta_input := []float64{0.25, 0.177, 0.125}
	// time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000, 10000000}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000, 10000000, 100000000}
	// time_window_size_input := []int64{1000000, 10000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, beta := range beta_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 16 {
					break
				}
			}
			testname := fmt.Sprintf("SHUniv_%d_%f", time_window_size, beta)
			f, err := os.OpenFile("./microbenchmark_results/shuniv-topkonly/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(beta float64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				fmt.Fprintf(w, "=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthUnivTopKOnly(time_window_size, w)
				// GroundTruth := make(map[string]([]AnswerUniv))
				SmoothHistogramUnivTopKOnly(beta, time_window_size, GroundTruth, w)
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done beta=%f, window_size=%d===================>\n", beta, time_window_size)
				wg.Done()
			}(beta, time_window_size)
		}
	}
	wg.Wait()
}

func TestSmoothHistogramUniv(t *testing.T) {
	runtime.GOMAXPROCS(40)
	// constructInputTimeSeriesUnivOffline()
	constructInputTimeSeriesUniformInt64()
	// readCAIDA()
	fmt.Println("finished construct input time series")

	beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	// beta_input := []float64{0.25, 0.177, 0.125}
	// time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000, 10000000}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000, 10000000}
	// time_window_size_input := []int64{1000000, 10000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, beta := range beta_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 20 {
					break
				}
			}
			testname := fmt.Sprintf("SHUniv_%d_%f_uniform", time_window_size, beta)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(beta float64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				fmt.Fprintf(w, "=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthUniv(time_window_size, w)
				// GroundTruth := make(map[string]([]AnswerUniv))
				SmoothHistogramUniv(beta, time_window_size, GroundTruth, w)
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done beta=%f, window_size=%d===================>\n", beta, time_window_size)
				wg.Done()
			}(beta, time_window_size)
		}
	}
	wg.Wait()
}

func TestSmoothHistogramUnivSubWindow(t *testing.T) {
	runtime.GOMAXPROCS(40)
	// constructInputTimeSeriesUnivOffline()
	constructInputTimeSeriesUniformInt64()
	// constructInputTimeSeriesZipf()
	fmt.Println("finished construct input time series")

	beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	// beta_input := []float64{0.25, 0.177, 0.125}
	// time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000, 10000000}
	time_window_size_input := []int64{1000000}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	// time_window_size_input := []int64{1000000, 10000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, subwindow_size := range subwindow_size_input {
			for _, beta := range beta_input {
				atomic.AddUint64(&ops, add)
				for {
					if ops < 16 {
						break
					}
				}
				testname := fmt.Sprintf("SHUniv_%d_%f_%d_%d_uniform", time_window_size, beta, time_window_size-subwindow_size.start, time_window_size-subwindow_size.end)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(beta float64, time_window_size int64, start_t, end_t int64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					w := bufio.NewWriter(f)
					fmt.Printf("=================>Testing beta=%f, window_size=%d, start_t=%d, end_t=%d===================>\n", beta, time_window_size, start_t, end_t)
					fmt.Fprintf(w, "=================>Testing beta=%f, window_size=%d, start_t=%d, end_t=%d===================>\n", beta, time_window_size, start_t, end_t)
					w.Flush()
					GroundTruth := calcGroundTruthUnivSubWindow(time_window_size, w, start_t, end_t)
					// GroundTruth := make(map[string]([]AnswerUniv))
					SmoothHistogramUnivSubWindow(beta, time_window_size, GroundTruth, w, start_t, end_t)
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done beta=%f, window_size=%d, start_t=%d, end_t=%d===================>\n", beta, time_window_size, start_t, end_t)
					wg.Done()
				}(beta, time_window_size, subwindow_size.start, subwindow_size.end)
			}
		}
	}
	wg.Wait()
}

func funcSmoothHistogramL2(beta float64, time_window_size int64, GroundTruth map[string]([]AnswerUniv), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with SmoothHistogram L2===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)
	shs := make(map[string](*SmoothHistogramCS))
	for _, c := range cases {
		shs[c.key] = SmoothInitCS(beta, time_window_size)
	}

	SHAnswer := make(map[string]([]AnswerUniv))
	for _, c := range cases {
		SHAnswer[c.key] = make([]AnswerUniv, 0)
	}

	var total_insert_time float64 = 0
	var total_insert float64 = 0
	for t := int64(0); t < int64(time_window_size/2*3); t++ {
		for _, c := range cases {
			start := time.Now()
			shs[c.key].Update(c.vec[t].T, strconv.FormatFloat(c.vec[t].F, 'f', -1, 64), 1) // insert data to univmon for current timeseries
			elapsed := time.Since(start)
			total_insert_time += float64(elapsed.Microseconds())
			total_insert += 1

			if t >= t2 {
				start := time.Now()
				merged_cs, _ := shs[c.key].QueryIntervalMergeCS(t-t2, t-t1, t)

				l2 := merged_cs.cs_l2_new()
				elapsed := time.Since(start)

				SHAnswer[c.key] = append(SHAnswer[c.key], AnswerUniv{l2: l2, time: float64(elapsed.Microseconds())})

			}
		}
	}

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%serror_card, error_l1, error_l2, error_entropy, gt_time(us), sketch_time(us)\n", c.key)
		w.Flush()
		var (
			total_error_l2    float64 = 0
			total_time        float64 = 0
			total_sketch_time float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ { // time dimension
			error_l2 := AbsFloat64(GroundTruth[c.key][i].l2-SHAnswer[c.key][i].l2) / GroundTruth[c.key][i].l2
			total_error_l2 += error_l2
			total_time += GroundTruth[c.key][i].time
			total_sketch_time += SHAnswer[c.key][i].time
		}
		if len(GroundTruth[c.key]) > 0 {
			fmt.Fprintf(w, "Average error and time:  avg_l2_error: %f%%,  avg_gt_time: %f(us), avg_sketch_time: %f(us)\n", total_error_l2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_sketch_time/float64(len(GroundTruth[c.key])))
			fmt.Fprintf(w, "Average sketch insert time: %f(us)\n", total_insert_time/total_insert)
			fmt.Fprintf(w, "bucket number: %d\n", shs[c.key].s_count)
			w.Flush()
		}
	}
	w.Flush()
}

func TestSmoothHistogramL2(t *testing.T) {
	runtime.GOMAXPROCS(40)
	constructInputTimeSeriesUniv()
	fmt.Println("finished construct input time series")

	beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	// beta_input := []float64{1}
	time_window_size_input := []int64{100000, 1000000, 10000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, beta := range beta_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 40 {
					break
				}
			}
			testname := fmt.Sprintf("SHL2_%d_%f", time_window_size, beta)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(beta float64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				fmt.Fprintf(w, "=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthUniv(time_window_size, w)
				funcSmoothHistogramL2(beta, time_window_size, GroundTruth, w)
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done beta=%f, window_size=%d===================>\n", beta, time_window_size)
				wg.Done()
			}(beta, time_window_size)
		}
	}
	wg.Wait()
}

/*
func funcSmoothHistogramCS(beta float64, time_window_size int64, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with SmoothHistogram CS===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)

	sh := SmoothInitCS(beta, time_window_size)

	SHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		SHAnswer[c.key] = make([]AnswerSum2, 0)
	}
	var total_elapsed float64 = 0
	for t := int64(0); t < time_window_size*2; t++ {
		var total_insert float64 = 0
		for _, c := range cases {
			start_insert := time.Now()
			sh.Update(c.vec[t].T, c.key, c.vec[t].F)
			elapsed_insert := time.Since(start_insert)
			total_insert += float64(elapsed_insert.Microseconds())
			if t >= t2 {
				start := time.Now()
				merged_bucket := sh.QueryIntervalMergeCS(t-t2, t-t1)
				count := merged_bucket.EstimateStringCount(c.key)
				sum := merged_bucket.EstimateStringSum(c.key)
				sum2 := merged_bucket.EstimateStringSum2(c.key)
				elapsed := time.Since(start)
				total_elapsed += float64(elapsed.Microseconds())
				SHAnswer[c.key] = append(SHAnswer[c.key], AnswerSum2{count: count, sum: sum, sum2: sum2, time: float64(elapsed.Microseconds())})
			}
		}
		if t%10000 == 0 {
			// fmt.Fprintf(w, "At %d time\n", t)
			w.Flush()
		}
	}
	fmt.Fprintf(w, "============Start comparing answers!=================\n")
	for _, c := range cases {
		fmt.Fprintf(w, "%s error_count, error_sum, error_sum2\n", c.key)
		w.Flush()
		var (
			total_error_count float64 = 0
			total_error_sum   float64 = 0
			total_error_sum2  float64 = 0
			total_time        float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ {
			count_err := AbsFloat64(GroundTruth[c.key][i].count-SHAnswer[c.key][i].count) / GroundTruth[c.key][i].count
			sum_err := AbsFloat64(GroundTruth[c.key][i].sum-SHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			sum2_err := AbsFloat64(GroundTruth[c.key][i].sum2-SHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2
			time := SHAnswer[c.key][i].time
			fmt.Fprintf(w, "%f,%f,%f,%f,%f\n", count_err, sum_err, sum2_err, GroundTruth[c.key][i].time, SHAnswer[c.key][i].time)
			if i%100 == 0 {
				w.Flush()
			}
			total_error_count += count_err
			total_error_sum += sum_err
			total_error_sum2 += sum2_err
			total_time += time
		}
		fmt.Fprintf(w, "Average error and time: avg_count_error: %f%%, avg_sum_error: %f%%, avg_sum2_error: %f%%, avg_time: %f(us)\n", total_error_count/float64(len(GroundTruth[c.key]))*100, total_error_sum/float64(len(GroundTruth[c.key]))*100, total_error_sum2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

func TestSmoothHistogramCS(t *testing.T) {
	constructInputTimeSeries()
	fmt.Println("finished construct input time series")
	runtime.GOMAXPROCS(40)
	beta_input := []float64{1.0, 0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, beta := range beta_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 40 {
					break
				}
			}
			testname := fmt.Sprintf("SHCS_%d_%f", time_window_size, beta)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(beta float64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				fmt.Fprintf(w, "=================>Testing beta=%f, window_size=%d===================>\n", beta, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthSUM2(time_window_size, w)
				funcSmoothHistogramCS(beta, time_window_size, GroundTruth, w)
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done beta=%f, window_size=%d===================>\n", beta, time_window_size)
				wg.Done()
			}(beta, time_window_size)
		}
	}
	wg.Wait()
}
*/
