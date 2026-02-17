package promsketch

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUnivPool(t *testing.T) {
	t_now := time.Now()
	ehu := ExpoInitUniv(100, 100000)
	since := time.Since(t_now)
	fmt.Println("expo init univ total time =", since)
	fmt.Println("expo init univ each time =", since/time.Duration(UnivPoolCAP))

	for t_idx := int64(0); t_idx < 1000; t_idx++ {
		value := float64(0)
		for {
			value = rand.NormFloat64()*stdDev + stdMean
			if value >= 0 && value <= value_scale {
				break
			}
		}

		t_now := time.Now()
		ehu.Update(t_idx, value) // insert data to univmon for current timeseries
		since := time.Since(t_now)
		fmt.Println("ehu insertion time =", since)
	}
}

type TestCase struct {
	key string
	vec Vector
}

type AnswerUniv struct {
	card    float64 // distinct_over_time of a timeseries
	l1      float64 // l1 norm_over_time
	l2      float64 // l2 norm_over_time
	entropy float64 // entropy_over_time
	time    float64 // query time
	memory  float64
}

type AnswerQuantile struct {
	quantiles []float64
	time      float64 // query time
	memory    float64
}

type AnswerSum2 struct {
	count  float64
	sum    float64
	sum2   float64
	time   float64 // query time
	memory float64 // eh memory usage, KB
}

var (
	cases                     = make([]TestCase, 0)
	total_time_series int     = 1
	value_scale       float64 = 50000
	stdDev            float64 = 10000
	stdMean           float64 = 50000
	test_times        int     = 5
	time_range        int64   = 200000000
)

func readGoogleClusterData2009() {
	filename := []string{"./testdata/google-cluster-data-1.csv"} // 3535030 rows
	lines := int64(0)
	vec := make(Vector, 0)
	for i := 0; i < len(filename); i++ {
		f, err := os.Open(filename[i])
		if err != nil {
			panic(err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		first_line := true
		for scanner.Scan() {
			if first_line {
				first_line = false
				continue
			}
			splits := strings.Split(scanner.Text(), " ")
			F, _ := strconv.ParseFloat(strings.TrimSpace(splits[5]), 64) // memory field
			vec = append(vec, Sample{T: int64(lines), F: F})
			lines += 1
		}
	}
	key := "google-cluster"
	tmp := TestCase{
		key: key,
		vec: vec,
	}
	cases = append(cases, tmp)
}

func TestReadGoogleClusterData(t *testing.T) {
	readGoogleClusterData2009()
}

func readPowerDataset() {
	f, err := os.Open("./testdata/household_power_consumption.txt")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lines := 0
	vec := make(Vector, 0)
	for scanner.Scan() {
		lines++
		splits := strings.Split(scanner.Text(), ";")
		if lines == 1 {
			continue
		}
		F, _ := strconv.ParseFloat(strings.TrimSpace(splits[2]), 64) // Global_active_power field
		vec = append(vec, Sample{T: int64(lines), F: F})
	}
	key := "power"
	tmp := TestCase{
		key: key,
		vec: vec,
	}
	cases = append(cases, tmp)
}

func TestReadPowerDataset(t *testing.T) {
	readPowerDataset()
}

func constructInputTimeSeriesZipf() {
	// Construct input timeseries
	for i := 0; i < total_time_series; i++ {
		key := "machine" + strconv.Itoa(i)
		vec := make(Vector, 0)
		var s float64 = 1.01
		var v float64 = 1
		var RAND *rand.Rand = rand.New(rand.NewSource(time.Now().Unix()))
		z := rand.NewZipf(RAND, s, v, uint64(value_scale))
		for t := int64(0); t < time_range; t++ {
			value := float64(z.Uint64()) + 1 // avoid 0 value
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

func constructInputTimeSeriesUniformFloat64() { // X~U(0, 100000)
	// Construct input timeseries
	for i := 0; i < total_time_series; i++ {
		key := "machine" + strconv.Itoa(i)
		vec := make(Vector, 0)
		for t := int64(0); t < time_range; t++ {
			value := rand.Float64()*value_scale + 0.00001 // avoid 0 value

			// TODO: DDSketch and KLL only works with positive float64 currently
			vec = append(vec, Sample{T: t, F: float64(value)})
		}

		tmp := TestCase{
			key: key,
			vec: vec,
		}
		cases = append(cases, tmp)
	}
}

func constructInputTimeSeriesUniformInt64() { // X~U(0, 100000)
	// Construct input timeseries
	for i := 0; i < total_time_series; i++ {
		key := "machine" + strconv.Itoa(i)
		vec := make(Vector, 0)
		for t := int64(0); t < time_range; t++ {
			// value := rand.Uint64()%uint64(value_scale) + 1
			value := rand.Intn(int(value_scale)) + 1 // avoid 0 value
			// TODO: DDSketch and KLL only works with positive float64 currently
			vec = append(vec, Sample{T: t, F: float64(value)})
		}

		tmp := TestCase{
			key: key,
			vec: vec,
		}
		cases = append(cases, tmp)
	}
}

func calcGroundTruthUnivTimeSeriesZipf() {
	runtime.GOMAXPROCS(40)

	key := "machine" + strconv.Itoa(0)
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

	var wg sync.WaitGroup
	time_window_sizes := []int64{1000000, 10000000, 100000000}
	for _, time_window_size := range time_window_sizes {
		wg.Add(1)
		go func(time_window_size int64) {
			defer wg.Done()
			gt_file, err := os.OpenFile("./testdata/zipf/"+testname+"_univ_groundtruth_"+strconv.FormatInt(time_window_size, 10)+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer gt_file.Close()
			w := bufio.NewWriter(gt_file)

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
					fmt.Fprintf(w, "%d %f %f %f %f\n", t-1, float64(len(l1Map)), l1, entropynorm, l2)
				}
				w.Flush()
				if t%1000 == 0 {
					fmt.Println("time=", t)
				}
			}
		}(time_window_size)
	}
	wg.Wait()
}

func constructInputTimeSeriesOutputZipf() {
	runtime.GOMAXPROCS(40)
	// Construct input timeseries
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for i := 0; i < 1; i++ {
		wg.Add(1)
		atomic.AddUint64(&ops, add)
		for {
			if ops < 30 {
				break
			}
		}
		go func(i int) {
			f, err := os.OpenFile("./testdata/zipf.txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			w := bufio.NewWriter(f)
			vec := make(Vector, 0)
			var s float64 = 1.01
			var v float64 = 1
			var RAND *rand.Rand = rand.New(rand.NewSource(time.Now().Unix()))
			z := rand.NewZipf(RAND, s, v, uint64(value_scale))
			for t := int64(0); t < 10000001; t++ {
				value := float64(z.Uint64()) + 1 // avoid 0 value
				vec = append(vec, Sample{T: t, F: value})
				fmt.Fprintf(w, "%d %f\n", t, value)
				if t%100000 == 0 {
					fmt.Println("generate data t=", t)
				}
			}
			tmp := TestCase{
				key: "zipf",
				vec: vec,
			}
			cases = append(cases, tmp)
			w.Flush()
			atomic.AddUint64(&ops, -add)
			wg.Done()
		}(i)
	}
	wg.Wait()
	fmt.Println("Done generating data")
}

func constructInputTimeSeriesOutputNormal() {
	runtime.GOMAXPROCS(40)
	// Construct input timeseries
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for i := 0; i < total_time_series; i++ {
		wg.Add(1)
		atomic.AddUint64(&ops, add)
		for {
			if ops < 30 {
				break
			}
		}
		go func(i int) {
			key := "machine" + strconv.Itoa(i)
			testname := key
			f, err := os.OpenFile("./testdata/normal/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			w := bufio.NewWriter(f)
			// stdMean_f := rand.Float64() * value_scale
			// stdDev_f := rand.Float64() * value_scale / 2
			for t := int64(0); t < time_range; t++ {
				value := float64(0)
				for {
					value = rand.NormFloat64()*stdDev + stdMean
					if value >= 0 && value <= value_scale {
						break
					}
				}
				fmt.Fprintf(w, "%d %f\n", t, value)
			}
			w.Flush()
			atomic.AddUint64(&ops, -add)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestConstructInputSeriesNormal(t *testing.T) {
	constructInputTimeSeriesOutputNormal()
}

func TestConstructInputSeriesZipf(t *testing.T) {
	constructInputTimeSeriesOutputZipf()
	calcGroundTruthUnivTimeSeriesZipf()
}

func calcGroundTruthSUM2(time_window_size int64, w *bufio.Writer) map[string]([]AnswerSum2) {
	fmt.Fprintf(w, "=================>Calculating Smooth Histogram Sum2 GroundTruth===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)

	// Ground Truth
	GroundTruth := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		GroundTruth[c.key] = make([]AnswerSum2, 0)
	}
	for t := t2; t < time_window_size*2; t++ {
		for _, c := range cases {
			start := time.Now()
			count := float64(0)
			sum := float64(0)
			sum2 := float64(0)
			for tt := t - t2; tt < t-t1; tt++ {
				count += 1
				sum += c.vec[tt].F
				sum2 += c.vec[tt].F * c.vec[tt].F
			}

			GroundTruth[c.key] = append(GroundTruth[c.key], AnswerSum2{count: count,
				sum:  sum,
				sum2: sum2,
				time: float64(time.Since(start).Microseconds())})

		}
	}
	return GroundTruth
}

func calcGroundTruthSUM2SubWindow(time_window_size int64, w *bufio.Writer, start_t, end_t int64) map[string]([]AnswerSum2) {
	fmt.Fprintf(w, "=================>Calculating Smooth Histogram Sum2 GroundTruth===================>\n")
	w.Flush()
	t1 := int64(start_t)
	t2 := int64(end_t)

	// Ground Truth
	GroundTruth := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		GroundTruth[c.key] = make([]AnswerSum2, 0)
	}
	for t := time_window_size; t < time_window_size*2; t++ {
		for _, c := range cases {
			start := time.Now()
			count := float64(0)
			sum := float64(0)
			sum2 := float64(0)
			for tt := t - t2; tt < t-t1; tt++ {
				count += 1
				sum += c.vec[tt].F
				sum2 += c.vec[tt].F * c.vec[tt].F
			}

			GroundTruth[c.key] = append(GroundTruth[c.key], AnswerSum2{count: count,
				sum:  sum,
				sum2: sum2,
				time: float64(time.Since(start).Microseconds())})

		}
	}
	return GroundTruth
}

func calcGroundTruthQuantile(time_window_size int64, phis []float64, w *bufio.Writer) map[string]([]AnswerQuantile) {
	fmt.Fprintf(w, "=================>Calculating Expo Histogram Quantile GroundTruth===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)

	var total_insert_gt int64 = 0
	var total_query_gt int64 = 0
	var total_queries float64 = 0
	var total_inserts float64 = 0

	// Ground Truth
	GroundTruth := make(map[string]([]AnswerQuantile))
	for _, c := range cases {
		GroundTruth[c.key] = make([]AnswerQuantile, 0)
	}
	for t := t2; t < time_window_size*2; t++ {
		for _, c := range cases {
			vec := make([]float64, 0)
			for tt := t - t2; tt < t-t1; tt++ {
				start_insert_gt := time.Now()
				vec = append(vec, c.vec[tt].F)
				elapsed_insert_gt := time.Since(start_insert_gt)
				total_insert_gt += elapsed_insert_gt.Microseconds()
				total_inserts += 1
			}
			start_query_gt := time.Now()
			res := AnswerQuantile{quantiles: quantiles(phis, vec), time: float64(time.Since(start_query_gt).Microseconds())}
			GroundTruth[c.key] = append(GroundTruth[c.key], res)
			elapsed_query_gt := time.Since(start_query_gt)
			total_query_gt += elapsed_query_gt.Microseconds()
			total_queries += 1
		}
	}
	fmt.Fprintf(w, "avg of read quantile ground truth (us): %f\n", float64(total_insert_gt)/total_inserts)
	fmt.Fprintf(w, "avg of query quantile ground truth (us):%f\n", float64(total_query_gt)/total_queries)
	return GroundTruth
}

func calcGroundTruthQuantileSubWindow(time_window_size int64, phis []float64, w *bufio.Writer, start_t, end_t int64) map[string]([]AnswerQuantile) {
	fmt.Fprintf(w, "=================>Calculating Expo Histogram Quantile GroundTruth===================>\n")
	w.Flush()
	t1 := int64(start_t)
	t2 := int64(end_t)

	var total_insert_gt int64 = 0
	var total_query_gt int64 = 0
	var total_queries float64 = 0
	var total_inserts float64 = 0

	// Ground Truth
	GroundTruth := make(map[string]([]AnswerQuantile))
	for _, c := range cases {
		GroundTruth[c.key] = make([]AnswerQuantile, 0)
	}
	for t := time_window_size; t < time_window_size*2; t++ {
		for _, c := range cases {
			vec := make([]float64, 0)
			for tt := t - t2; tt < t-t1; tt++ {
				start_insert_gt := time.Now()
				vec = append(vec, c.vec[tt].F)
				elapsed_insert_gt := time.Since(start_insert_gt)
				total_insert_gt += elapsed_insert_gt.Microseconds()
				total_inserts += 1
			}
			start_query_gt := time.Now()
			res := AnswerQuantile{quantiles: quantiles(phis, vec), time: float64(time.Since(start_query_gt).Microseconds())}
			GroundTruth[c.key] = append(GroundTruth[c.key], res)
			elapsed_query_gt := time.Since(start_query_gt)
			total_query_gt += elapsed_query_gt.Microseconds()
			total_queries += 1
		}
	}
	fmt.Fprintf(w, "avg of read quantile ground truth (us): %f\n", float64(total_insert_gt)/total_inserts)
	fmt.Fprintf(w, "avg of query quantile ground truth (us):%f\n", float64(total_query_gt)/total_queries)
	return GroundTruth
}

func funcExpoHistogramUniv(k int64, time_window_size int64, GroundTruth map[string]([]AnswerUniv), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with ExpoHistogram UnivMon===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)
	m := float64(t2 - t1)

	/*
		for _, c := range cases {
			fmt.Println(c.key)
			for _, ans := range GroundTruth[c.key] {
				fmt.Println(ans.card, ans.sum, ans.l2, ans.entropy)
			}
		}
	*/

	// one EH+Univ for one timeseries
	ehs := make(map[string](*ExpoHistogramUniv))
	for _, c := range cases {
		ehs[c.key] = ExpoInitUniv(k, time_window_size)
	}

	EHAnswer := make(map[string]([]AnswerUniv))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerUniv, 0)
	}

	// for debugging just one query
	// raw_data := make([]string, 0)

	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			ehs[c.key].Update(c.vec[t].T, c.vec[t].F) // insert data to univmon for current timeseries
			if t >= t2 {
				start := time.Now()
				merged_univ, err := ehs[c.key].QueryIntervalMergeUniv(t-t2, t-t1, t)
				if err == nil {
					card := merged_univ.calcCard()
					elapsed := time.Since(start)
					l1 := merged_univ.calcL1()
					l2 := merged_univ.calcL2()
					l2 = math.Sqrt(l2)
					entropynorm := merged_univ.calcEntropy()
					entropy := math.Log(m)/math.Log(2) - entropynorm/m
					EHAnswer[c.key] = append(EHAnswer[c.key], AnswerUniv{card: card, l1: l1, l2: l2, entropy: entropy, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
					// fmt.Fprintf(w, "debug entropy: %f %f %f\n", GroundTruth[c.key][len(EHAnswer[c.key])-1].entropy-EHAnswer[c.key][len(EHAnswer[c.key])-1].entropy, GroundTruth[c.key][len(EHAnswer[c.key])-1].entropy, EHAnswer[c.key][len(EHAnswer[c.key])-1].entropy)
					// fmt.Fprintf(w, "debug error_entropy= %f\n", AbsFloat64(GroundTruth[c.key][len(EHAnswer[c.key])-1].entropy-EHAnswer[c.key][len(EHAnswer[c.key])-1].entropy)/GroundTruth[c.key][len(EHAnswer[c.key])-1].entropy)
				} else {
					EHAnswer[c.key] = append(EHAnswer[c.key], AnswerUniv{card: -1, l1: -1, l2: -1, entropy: -1, time: -1, memory: -1})
				}
			}
			/*
				if t < t1 {
					raw_data = append(raw_data, strconv.FormatFloat(c.vec[t].F, 'f', -1, 64))
				}
			*/
		}
		/*
			if t >= t2 {
				break
			}
		*/
	}

	// for debugging just one query
	/*
		fmt.Println("raw_data =", raw_data)
		cs_seed1 := make([]uint32, CS_ROW_NO)
		cs_seed2 := make([]uint32, CS_ROW_NO)
		rand.Seed(time.Now().UnixNano())
		for i := 0; i < CS_ROW_NO; i++ {
			cs_seed1[i] = rand.Uint32()
			cs_seed2[i] = rand.Uint32()
		}
		test_univ, _ := NewUnivSketch(TOPK_SIZE, CS_ROW_NO, CS_COL_NO, CS_LVLS, cs_seed1, cs_seed2)
		for _, key := range raw_data {
			test_univ.univmon_processing(key, 1)
		}
		test_card := test_univ.calcCard()
		test_entropynorm := test_univ.calcEntropy()
		test_l1 := test_univ.calcL1()
		test_l2 := test_univ.calcL2()
		fmt.Println("test_card =", test_card)
		fmt.Println("test_entropynorm =", test_entropynorm)
		fmt.Println("test_l1 =", test_l1)
		fmt.Println("test_l2 =", test_l2)
	*/

	fmt.Fprintf(w, "============Start comparing answers!=================\n")

	for _, c := range cases {
		fmt.Fprintf(w, "%s error_card, error_l1, error_l2, error_entropy, query_time(us)\n", c.key)
		w.Flush()
		// assert.Equal(t, len(GroundTruth[c.key]), len(EHAnswer[c.key]), "the answer length should be the same.")

		var (
			total_error_card    float64 = 0
			total_error_l1      float64 = 0
			total_error_l2      float64 = 0
			total_error_entropy float64 = 0
			total_time          float64 = 0
			total_memory        float64 = 0
		)
		for i := 0; i < len(GroundTruth[c.key]); i++ { // time dimension
			/*
				fmt.Println("card:", GroundTruth[c.key][i].card, EHAnswer[c.key][i].card, AbsFloat64(GroundTruth[c.key][i].card-EHAnswer[c.key][i].card)/GroundTruth[c.key][i].card)
				fmt.Println("l1:", GroundTruth[c.key][i].l1, EHAnswer[c.key][i].l1, AbsFloat64(GroundTruth[c.key][i].l1-EHAnswer[c.key][i].l1)/GroundTruth[c.key][i].l1)
				fmt.Println("l2:", GroundTruth[c.key][i].l2, EHAnswer[c.key][i].l2, AbsFloat64(GroundTruth[c.key][i].l2-EHAnswer[c.key][i].l2)/GroundTruth[c.key][i].l2)
				fmt.Println("entropy:", GroundTruth[c.key][i].entropy, EHAnswer[c.key][i].entropy, AbsFloat64(GroundTruth[c.key][i].entropy-EHAnswer[c.key][i].entropy)/GroundTruth[c.key][i].entropy)
			*/
			error_card := AbsFloat64(GroundTruth[c.key][i].card-EHAnswer[c.key][i].card) / GroundTruth[c.key][i].card
			error_l1 := AbsFloat64(GroundTruth[c.key][i].l1-EHAnswer[c.key][i].l1) / GroundTruth[c.key][i].l1
			error_l2 := AbsFloat64(GroundTruth[c.key][i].l2-EHAnswer[c.key][i].l2) / GroundTruth[c.key][i].l2
			error_entropy := AbsFloat64(GroundTruth[c.key][i].entropy-EHAnswer[c.key][i].entropy) / GroundTruth[c.key][i].entropy

			// fmt.Fprintf(w, "debug entropy: %f %f %f\n", GroundTruth[c.key][i].entropy-EHAnswer[c.key][i].entropy, GroundTruth[c.key][i].entropy, EHAnswer[c.key][i].entropy)

			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory
			if i%100 == 0 {
				// fmt.Fprintf(w, "%f,%f,%f,%f,%f\n", error_card, error_l1, error_l2, error_entropy, time)
				w.Flush()
			}
			total_error_card += error_card
			total_error_l1 += error_l1
			total_error_l2 += error_l2
			total_error_entropy += error_entropy
			total_time += time
			total_memory += memory
			/*
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].card-EHAnswer[c.key][i].card)/GroundTruth[c.key][i].card < 0.05 {
						return true
					} else {
						return false
					}
				}, "Card error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].l1-EHAnswer[c.key][i].l1)/GroundTruth[c.key][i].l1 < 0.05 {
						return true
					} else {
						return false
					}
				}, "L1 error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].l2-EHAnswer[c.key][i].l2)/GroundTruth[c.key][i].l2 < 0.05 {
						return true
					} else {
						return false
					}
				}, "L2 error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].entropy-EHAnswer[c.key][i].entropy)/GroundTruth[c.key][i].entropy < 0.05 {
						return true
					} else {
						return false
					}
				}, "Entropy error too large")
			*/
		}

		fmt.Fprintf(w, "Average error, time, and memory: avg_card_error: %f%%, avg_l1_error: %f%%, avg_l2_error: %f%%, avg_entropy_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_card/float64(len(GroundTruth[c.key]))*100, total_error_l1/float64(len(GroundTruth[c.key]))*100, total_error_l2/float64(len(GroundTruth[c.key]))*100, total_error_entropy/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

func TestExpoHistogramUniv(t *testing.T) {
	runtime.GOMAXPROCS(40)

	constructInputTimeSeriesUniv()
	fmt.Println("finished construct input time series")

	ehk_input := []int64{10, 20, 50, 100, 200, 500, 1000} // error = sqrt(C_f/k), C_f=2, 44%, 31%, 20%, 14%, 10%, 6.3%, 4.4%
	// time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	time_window_size_input := []int64{10000, 100000, 1000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, k := range ehk_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 30 {
					break
				}
			}
			testname := fmt.Sprintf("EHUniv_%d_%d", time_window_size, k)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(k int64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				w := bufio.NewWriter(f)
				fmt.Printf("=================>Testing k=%d, window_size=%d===================>\n", k, time_window_size)
				fmt.Fprintf(w, "=================>Testing k=%d, window_size=%d===================>\n", k, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthUniv(time_window_size, w)
				funcExpoHistogramUniv(k, time_window_size, GroundTruth, w)
				w.Flush()
				atomic.AddUint64(&ops, -add)
				fmt.Printf("=================>Done k=%d, window_size=%d===================>\n", k, time_window_size)
				wg.Done()
			}(k, time_window_size)
		}
	}
	wg.Wait()
}

func funcExpoHistogramKLL(k int64, kll_k int, time_window_size int64, phis []float64, GroundTruth map[string]([]AnswerQuantile), w *bufio.Writer) {
	fmt.Fprintf(w, "===================Estimating Expo Histogram KLL=======================\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)

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

	var total_insert_ehkll int64 = 0
	var total_query_ehkll int64 = 0
	var total_queries float64 = 0
	var total_inserts float64 = 0
	var total_mem float64 = 0

	for test_time := 0; test_time < test_times; test_time++ {
		// one EH+KLL for one timeseries
		ehs := make(map[string](*ExpoHistogramKLL))
		for _, c := range cases {
			ehs[c.key] = ExpoInitKLL(k, kll_k, time_window_size)
		}

		EHAnswer := make(map[string]([]AnswerQuantile))
		for _, c := range cases {
			EHAnswer[c.key] = make([]AnswerQuantile, 0)
		}
		for t := int64(0); t < time_window_size*2; t++ {
			for _, c := range cases {
				start_insert_ehkll := time.Now()
				ehs[c.key].Update(c.vec[t].T, c.vec[t].F)
				elapsed_insert_ehkll := time.Since(start_insert_ehkll)
				total_insert_ehkll += elapsed_insert_ehkll.Microseconds()
				total_inserts += 1
				if t >= t2 {
					start_query_ehkll := time.Now()
					merged_kll := ehs[c.key].QueryIntervalMergeKLL(t-t2, t-t1)
					cdf := merged_kll.CDF()
					q_values := make([]float64, 0)
					for _, phi := range phis {
						q_values = append(q_values, cdf.Query(phi))
					}
					elapsed_query_ehkll := time.Since(start_query_ehkll)
					total_query_ehkll += elapsed_query_ehkll.Microseconds()
					total_queries += 1
					EHAnswer[c.key] = append(EHAnswer[c.key], AnswerQuantile{quantiles: q_values, time: float64(elapsed_query_ehkll.Microseconds()), memory: ehs[c.key].GetMemory()})
					total_mem += ehs[c.key].GetMemory()
				}
			}
		}

		fmt.Fprintf(w, "===============Test %v=================\n", test_time)
		fmt.Fprintf(w, "queries window size: %d (data points)\n", time_window_size)
		w.Flush()
		/*
			for _, c := range cases {
				s_count, bucketsizes := ehs[c.key].get_memory()
				fmt.Fprintf(w, "final ehkll memory: %s : {s_count: %d, bucketsizes: %d \n", c.key, s_count, bucketsizes)
				total_memory := float64(0)
				for _, size := range bucketsizes {
					total_memory += 256 + math.Log(float64(size)/256)/math.Log(2)
				}
				fmt.Fprintf(w, "total_memory (KB): %f\n", total_memory*64/8/1024)
			}
			w.Flush()
		*/

		for _, c := range cases {
			fmt.Fprintf(w, "%s, quantile_errors, query_time(us)\n", c.key)

			for i := 0; i < len(GroundTruth[c.key]); i++ {
				// fmt.Println("** quantile error:", GroundTruth[c.key][i].quantile, EHAnswer[c.key][i].quantile, AbsFloat64(GroundTruth[c.key][i].quantile - EHAnswer[c.key][i].quantile) / GroundTruth[c.key][i].quantile)
				error_quantile := make([]float64, 0)
				for idx := range phis {
					error_quantile = append(error_quantile, AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
					avg_error[c.key][i][idx] += AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx]) / GroundTruth[c.key][i].quantiles[idx]
					max_error[c.key][i][idx] = MaxFloat64(max_error[c.key][i][idx], AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
				}
				// fmt.Fprintf(w, "quantile errors: %v\n", error_quantile)
				w.Flush()
			}
		}
	}

	fmt.Fprintf(w, "===============Summary over %v tests=================\n", test_times)
	fmt.Fprintf(w, "-----------------avg_error-----------------\n")
	avg_error_total := float64(0.0)
	max_error_total := float64(0.0)
	total_avg_count := float64(0.0)
	for _, c := range cases {
		for i := 0; i < len(GroundTruth[c.key]); i++ {
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
	fmt.Fprintf(w, "avg of insert ehkll (us): %f\n", float64(total_insert_ehkll)/total_inserts)
	fmt.Fprintf(w, "avg of query ehkll (us): %f\n", float64(total_query_ehkll)/total_queries)
	fmt.Fprintf(w, "avg of memory ehdd (KB): %f\n", total_mem/total_queries)
	w.Flush()
}

// test quantile_over_time
func TestExpoHistogramKLL(t *testing.T) {
	// constructInputTimeSeriesUniformFloat64()
	// constructInputTimeSeriesZipf()
	// constructInputTimeSeriesUniformFloat64()
	readPowerDataset()
	fmt.Println("finished construct input time series")
	runtime.GOMAXPROCS(40)

	ehk_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	kllk_input := []int{64, 128, 256, 512, 1024}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	// phis_input := []float64{0, 0.1, 0.2, 0.25, 0.3, 0.4, 0.5, 0.6, 0.7, 0.75, 0.8, 0.9, 0.99, 1}
	phis_input := []float64{0.5}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1

	for _, time_window_size := range time_window_size_input {
		for _, k := range ehk_input {
			for _, kll_k := range kllk_input {
				atomic.AddUint64(&ops, add)
				for {
					if ops < 35 {
						break
					}
				}
				testname := fmt.Sprintf("EHKLL_%d_%d_%d_0.5_power", time_window_size, k, kll_k)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(k int64, time_window_size int64, kll_k int) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					w := bufio.NewWriter(f)
					fmt.Printf("=================>Testing eh_k=%d, window_size=%d, kll_k=%d===================>\n", k, time_window_size, kll_k)
					fmt.Fprintf(w, "=================>Testing eh_k=%d, window_size=%d, kll_k=%d===================>\n", k, time_window_size, kll_k)
					w.Flush()
					GroundTruth := calcGroundTruthQuantile(time_window_size, phis_input, w)
					funcExpoHistogramKLL(k, kll_k, time_window_size, phis_input, GroundTruth, w)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done eh_k=%d, window_size=%d, kll_k=%d===================>\n", k, time_window_size, kll_k)
					wg.Done()
				}(k, time_window_size, kll_k)
			}
		}
	}
	wg.Wait()
}

func funcExpoHistogramCount(k int64, time_window_size int64, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer) {
	fmt.Fprintf(w, "=================>Estimating with ExpoHistogram Count===================>\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)

	// one EH+Accumulator for one timeseries
	ehs := make(map[string](*ExpoHistogramCount))
	for _, c := range cases {
		ehs[c.key] = ExpoInitCount(k, time_window_size)
	}

	EHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerSum2, 0)
	}
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			ehs[c.key].Update(c.vec[t].T, c.vec[t].F)
			if t >= t2 {
				start := time.Now()
				merged_bucket, err := ehs[c.key].QueryIntervalMergeCount(t-t2, t-t1)
				if err != nil {
					continue
				}
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerSum2{count: float64(merged_bucket.count), sum: merged_bucket.sum, sum2: merged_bucket.sum2, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
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
			/*
				fmt.Println("count:", GroundTruth[c.key][i].count, EHAnswer[c.key][i].count, AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count)/GroundTruth[c.key][i].count)
				fmt.Println("sum:", GroundTruth[c.key][i].sum, EHAnswer[c.key][i].sum, AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum)/GroundTruth[c.key][i].sum)
				fmt.Println("sum2:", GroundTruth[c.key][i].sum2, EHAnswer[c.key][i].sum2, AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2)/GroundTruth[c.key][i].sum2)
			*/
			error_count := AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count) / GroundTruth[c.key][i].count
			error_sum := AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			error_sum2 := AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2
			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			if i%100 == 0 {
				// fmt.Fprintf(w, "%f,%f,%f,%f\n", error_count, error_sum, error_sum2, time)
				w.Flush()
			}
			total_error_count += error_count
			total_error_sum += error_sum
			total_error_sum2 += error_sum2
			total_time += time
			total_memory += memory
			/*
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count)/GroundTruth[c.key][i].count <= 0.05 {
						return true
					} else {
						return false
					}
				}, "count error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum)/GroundTruth[c.key][i].sum <= 0.05 {
						return true
					} else {
						return false
					}
				}, "sum error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2)/GroundTruth[c.key][i].sum2 <= 0.05 {
						return true
					} else {
						return false
					}
				}, "sum2 error too large")
			*/
		}
		fmt.Fprintf(w, "Average error, time, memory: avg_count_error: %f%%, avg_sum_error: %f%%, avg_sum2_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_count/float64(len(GroundTruth[c.key]))*100, total_error_sum/float64(len(GroundTruth[c.key]))*100, total_error_sum2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

// test count_over_time, sum_over_time, sum2_over_time
func TestExpoHistogramCount(t *testing.T) {
	runtime.GOMAXPROCS(40)
	constructInputTimeSeriesZipf()
	fmt.Println("finished construct input time series")

	ehk_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, k := range ehk_input {
			atomic.AddUint64(&ops, add)
			for {
				if ops < 40 {
					break
				}
			}
			testname := fmt.Sprintf("EHCount_%d_%d", time_window_size, k)
			f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			wg.Add(1)
			go func(k int64, time_window_size int64) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				fmt.Printf("=================>Testing k=%d, window_size=%d===================>\n", k, time_window_size)
				w := bufio.NewWriter(f)
				fmt.Fprintf(w, "=================>Testing k=%d, window_size=%d===================>\n", k, time_window_size)
				w.Flush()
				GroundTruth := calcGroundTruthSUM2(time_window_size, w)
				funcExpoHistogramCount(k, time_window_size, GroundTruth, w)
				w.Flush()
				atomic.AddUint64(&ops, -add)
				wg.Done()
			}(k, time_window_size)
		}
	}
	wg.Wait()
}

func funcExpoHistogramCountSubWindow(k int64, time_window_size int64, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "=================>Estimating with ExpoHistogram Count===================>\n")
	w.Flush()
	t1 := int64(start_t)
	t2 := int64(end_t)

	// one EH+Accumulator for one timeseries
	ehs := make(map[string](*ExpoHistogramCount))
	for _, c := range cases {
		ehs[c.key] = ExpoInitCount(k, time_window_size)
	}

	EHAnswer := make(map[string]([]AnswerSum2))
	for _, c := range cases {
		EHAnswer[c.key] = make([]AnswerSum2, 0)
	}
	for t := int64(0); t < time_window_size*2; t++ {
		for _, c := range cases {
			ehs[c.key].Update(c.vec[t].T, c.vec[t].F)
			if t >= time_window_size {
				start := time.Now()
				merged_bucket, err := ehs[c.key].QueryIntervalMergeCount(t-t2, t-t1)
				if err != nil {
					continue
				}
				elapsed := time.Since(start)
				EHAnswer[c.key] = append(EHAnswer[c.key], AnswerSum2{count: float64(merged_bucket.count), sum: merged_bucket.sum, sum2: merged_bucket.sum2, time: float64(elapsed.Microseconds()), memory: ehs[c.key].GetMemory()})
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
			/*
				fmt.Println("count:", GroundTruth[c.key][i].count, EHAnswer[c.key][i].count, AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count)/GroundTruth[c.key][i].count)
				fmt.Println("sum:", GroundTruth[c.key][i].sum, EHAnswer[c.key][i].sum, AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum)/GroundTruth[c.key][i].sum)
				fmt.Println("sum2:", GroundTruth[c.key][i].sum2, EHAnswer[c.key][i].sum2, AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2)/GroundTruth[c.key][i].sum2)
			*/
			error_count := AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count) / GroundTruth[c.key][i].count
			error_sum := AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
			error_sum2 := AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2
			time := EHAnswer[c.key][i].time
			memory := EHAnswer[c.key][i].memory

			if i%100 == 0 {
				// fmt.Fprintf(w, "%f,%f,%f,%f\n", error_count, error_sum, error_sum2, time)
				w.Flush()
			}
			total_error_count += error_count
			total_error_sum += error_sum
			total_error_sum2 += error_sum2
			total_time += time
			total_memory += memory
			/*
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count)/GroundTruth[c.key][i].count <= 0.05 {
						return true
					} else {
						return false
					}
				}, "count error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum)/GroundTruth[c.key][i].sum <= 0.05 {
						return true
					} else {
						return false
					}
				}, "sum error too large")
				assert.Condition(t, func() bool {
					if AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2)/GroundTruth[c.key][i].sum2 <= 0.05 {
						return true
					} else {
						return false
					}
				}, "sum2 error too large")
			*/
		}
		fmt.Fprintf(w, "Average error, time, memory: avg_count_error: %f%%, avg_sum_error: %f%%, avg_sum2_error: %f%%, avg_time: %f(us), avg_memory: %f(KB)\n", total_error_count/float64(len(GroundTruth[c.key]))*100, total_error_sum/float64(len(GroundTruth[c.key]))*100, total_error_sum2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])), total_memory/float64(len(GroundTruth[c.key])))
		w.Flush()
	}
	w.Flush()
}

// test count_over_time
func TestExpoHistogramCountSubWindow(t *testing.T) {
	runtime.GOMAXPROCS(40)
	constructInputTimeSeriesZipf()
	fmt.Println("finished construct input time series")

	ehk_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	time_window_size_input := []int64{1000000}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1
	for _, time_window_size := range time_window_size_input {
		for _, subwindow_size := range subwindow_size_input {
			for _, k := range ehk_input {
				atomic.AddUint64(&ops, add)
				for {
					if ops < 30 {
						break
					}
				}
				testname := fmt.Sprintf("EHCount_%d_%d_%d_%d", time_window_size, k, time_window_size-subwindow_size.start, time_window_size-subwindow_size.end)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(k int64, time_window_size int64, start_t, end_t int64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					fmt.Printf("=================>Testing k=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", k, time_window_size, subwindow_size.start, subwindow_size.end)
					w := bufio.NewWriter(f)
					fmt.Fprintf(w, "=================>Testing k=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", k, time_window_size, subwindow_size.start, subwindow_size.end)
					w.Flush()
					GroundTruth := calcGroundTruthSUM2SubWindow(time_window_size, w, start_t, end_t)
					funcExpoHistogramCountSubWindow(k, time_window_size, GroundTruth, w, start_t, end_t)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done k=%d, window_size=%d, start_t=%d, end_t=%d===================>\n", k, time_window_size, subwindow_size.start, subwindow_size.end)
					wg.Done()
				}(k, time_window_size, subwindow_size.start, subwindow_size.end)
			}
		}
	}
	wg.Wait()
}

/*
	func funcExpoHistogramCS(k int64, time_window_size int64, GroundTruth map[string]([]AnswerSum2), w *bufio.Writer) {
		fmt.Fprintf(w, "=================>Estimating with ExpoHistogram Count===================>\n")
		w.Flush()
		t1 := int64(0)
		t2 := int64(time_window_size)

		// one EH+Accumulator for one timeseries
		ehs := ExpoInitCountCS(k, time_window_size)

		EHAnswer := make(map[string]([]AnswerSum2))
		for _, c := range cases {
			EHAnswer[c.key] = make([]AnswerSum2, 0)
		}
		for t := int64(0); t < time_window_size*2; t++ {
			for _, c := range cases {
				ehs.Update(c.vec[t].T, c.key, c.vec[t].F)
				if t >= t2 {
					start := time.Now()
					merged_bucket := ehs.QueryIntervalMergeCount(t-t2, t-t1)
					count := merged_bucket.EstimateStringCount(c.key)
					sum := merged_bucket.EstimateStringSum(c.key)
					sum2 := merged_bucket.EstimateStringSum2(c.key)
					elapsed := time.Since(start)
					EHAnswer[c.key] = append(EHAnswer[c.key], AnswerSum2{count: float64(count), sum: sum, sum2: sum2, time: float64(elapsed.Microseconds()), memory: ehs.GetMemory()})
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
			)
			for i := 0; i < len(GroundTruth[c.key]); i++ {

				error_count := AbsFloat64(GroundTruth[c.key][i].count-EHAnswer[c.key][i].count) / GroundTruth[c.key][i].count
				error_sum := AbsFloat64(GroundTruth[c.key][i].sum-EHAnswer[c.key][i].sum) / GroundTruth[c.key][i].sum
				error_sum2 := AbsFloat64(GroundTruth[c.key][i].sum2-EHAnswer[c.key][i].sum2) / GroundTruth[c.key][i].sum2
				time := EHAnswer[c.key][i].time
				if i%100 == 0 {
					// fmt.Fprintf(w, "%f,%f,%f,%f\n", error_count, error_sum, error_sum2, time)
					w.Flush()
				}
				total_error_count += error_count
				total_error_sum += error_sum
				total_error_sum2 += error_sum2
				total_time += time
			}
			fmt.Fprintf(w, "Average error and time: avg_count_error: %f%%, avg_sum_error: %f%%, avg_sum2_error: %f%%, avg_time: %f(us)\n", total_error_count/float64(len(GroundTruth[c.key]))*100, total_error_sum/float64(len(GroundTruth[c.key]))*100, total_error_sum2/float64(len(GroundTruth[c.key]))*100, total_time/float64(len(GroundTruth[c.key])))
			w.Flush()
		}
		w.Flush()
	}

// test count_over_time, sum_over_time, sum2_over_time

	func TestExpoHistogramCS(t *testing.T) {
		runtime.GOMAXPROCS(40)
		constructInputTimeSeries()
		fmt.Println("finished construct input time series")

		ehk_input := []int64{10, 20, 50, 100, 200, 500, 1000}
		time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
		var wg sync.WaitGroup
		var ops uint64 = 0
		var add uint64 = 1
		for _, time_window_size := range time_window_size_input {
			for _, k := range ehk_input {
				atomic.AddUint64(&ops, add)
				for {
					if ops < 40 {
						break
					}
				}
				testname := fmt.Sprintf("EHCount_%d_%d", time_window_size, k)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(k int64, time_window_size int64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					w := bufio.NewWriter(f)
					fmt.Printf("=================>Testing k=%d, window_size=%d===================>\n", k, time_window_size)
					fmt.Fprintf(w, "=================>Testing k=%d, window_size=%d===================>\n", k, time_window_size)
					w.Flush()
					GroundTruth := calcGroundTruthSUM2(time_window_size, w)
					funcExpoHistogramCS(k, time_window_size, GroundTruth, w)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					wg.Done()
				}(k, time_window_size)
			}
		}
		wg.Wait()
	}
*/

func funcExpoHistogramDD(k int64, dd_acc float64, time_window_size int64, phis []float64, GroundTruth map[string]([]AnswerQuantile), w *bufio.Writer) {
	fmt.Fprintf(w, "===================Estimating Expo Histogram DDSketch=======================\n")
	w.Flush()
	t1 := int64(0)
	t2 := int64(time_window_size)

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

	var total_insert_ehdd int64 = 0
	var total_query_ehdd int64 = 0
	var total_queries float64 = 0
	var total_inserts float64 = 0
	var total_mem float64 = 0

	for test_time := 0; test_time < test_times; test_time++ {
		// one EH+dd for one timeseries
		ehs := make(map[string](*ExpoHistogramDD))
		for _, c := range cases {
			ehs[c.key] = ExpoInitDD(k, time_window_size, dd_acc)
		}

		EHAnswer := make(map[string]([]AnswerQuantile))
		for _, c := range cases {
			EHAnswer[c.key] = make([]AnswerQuantile, 0)
		}
		for t := int64(0); t < time_window_size*2; t++ {
			for _, c := range cases {
				start_insert_ehdd := time.Now()
				ehs[c.key].Update(c.vec[t].T, c.vec[t].F)
				elapsed_insert_ehdd := time.Since(start_insert_ehdd)
				total_insert_ehdd += elapsed_insert_ehdd.Microseconds()
				total_inserts += 1
				if t >= t2 {
					start_query_ehdd := time.Now()
					merged_dd := ehs[c.key].QueryIntervalMergeDD(t-t2, t-t1)
					q_values, _ := merged_dd.GetValuesAtQuantiles(phis)
					elapsed_query_ehdd := time.Since(start_query_ehdd)
					total_query_ehdd += elapsed_query_ehdd.Microseconds()
					total_queries += 1
					EHAnswer[c.key] = append(EHAnswer[c.key], AnswerQuantile{quantiles: q_values, time: float64(elapsed_query_ehdd.Microseconds()), memory: ehs[c.key].GetMemory()})
					total_mem += ehs[c.key].GetMemory()
				}
			}
		}

		fmt.Fprintf(w, "===============Test %v=================\n", test_time)
		fmt.Fprintf(w, "queries window size: %d (data points)\n", time_window_size)
		w.Flush()
		/*
			for _, c := range cases {
				s_count, bucketsizes := ehs[c.key].get_memory()
				fmt.Fprintf(w, "final ehdd memory: %s : {s_count: %d, bucketsizes: %d \n", c.key, s_count, bucketsizes)
				total_memory := float64(0)
				for _, size := range bucketsizes {
					total_memory += 256 + math.Log(float64(size)/256)/math.Log(2)
				}
				fmt.Fprintf(w, "total_memory (KB): %f\n", total_memory*64/8/1024)
			}
			w.Flush()
		*/

		for _, c := range cases {
			fmt.Fprintf(w, "%s, quantile_errors, query_time(us)\n", c.key)

			for i := 0; i < len(GroundTruth[c.key]); i++ {
				// fmt.Println("** quantile error:", GroundTruth[c.key][i].quantile, EHAnswer[c.key][i].quantile, AbsFloat64(GroundTruth[c.key][i].quantile - EHAnswer[c.key][i].quantile) / GroundTruth[c.key][i].quantile)
				error_quantile := make([]float64, 0)
				for idx := range phis {
					error_quantile = append(error_quantile, AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
					avg_error[c.key][i][idx] += AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx]) / GroundTruth[c.key][i].quantiles[idx]
					// fmt.Println(GroundTruth[c.key][i].quantiles[idx], EHAnswer[c.key][i].quantiles[idx], avg_error[c.key][i][idx])
					max_error[c.key][i][idx] = MaxFloat64(max_error[c.key][i][idx], AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
				}
				// fmt.Fprintf(w, "quantile errors: %v\n", error_quantile)
				w.Flush()
			}
		}
	}

	fmt.Fprintf(w, "===============Summary over %v tests=================\n", test_times)
	fmt.Fprintf(w, "-----------------avg_error-----------------\n")
	avg_error_total := float64(0.0)
	max_error_total := float64(0.0)
	total_avg_count := float64(0.0)
	for _, c := range cases {
		for i := 0; i < len(GroundTruth[c.key]); i++ {
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
	fmt.Fprintf(w, "avg of insert ehdd (us): %f\n", float64(total_insert_ehdd)/total_inserts)
	fmt.Fprintf(w, "avg of query ehdd (us): %f\n", float64(total_query_ehdd)/total_queries)
	fmt.Fprintf(w, "avg of memory ehdd (KB): %f\n", total_mem/total_queries)
	w.Flush()
}

// test quantile_over_time using DDSketch
func TestExpoHistogramDD(t *testing.T) {
	constructInputTimeSeriesUniformFloat64()
	fmt.Println("finished construct input time series")
	runtime.GOMAXPROCS(40)

	ehdd_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	dd_acc_input := []float64{0.1, 0.05, 0.02, 0.01, 0.001}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	// phis_input := []float64{0.1, 0.2, 0.25, 0.3, 0.4, 0.5, 0.6, 0.7, 0.75, 0.8, 0.9, 0.99}
	phis_input := []float64{0.9}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1

	for _, time_window_size := range time_window_size_input {
		for _, k := range ehdd_input {
			for _, dd_acc := range dd_acc_input {
				atomic.AddUint64(&ops, add)
				for {
					if ops < 20 {
						break
					}
				}
				testname := fmt.Sprintf("EHDD_%d_%d_%f_0.5_uniform", time_window_size, k, dd_acc)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(k int64, time_window_size int64, dd_acc float64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					w := bufio.NewWriter(f)
					fmt.Printf("=================>Testing eh_k=%d, window_size=%d, dd_acc=%f===================>\n", k, time_window_size, dd_acc)
					fmt.Fprintf(w, "=================>Testing eh_k=%d, window_size=%d, dd_acc=%f===================>\n", k, time_window_size, dd_acc)
					w.Flush()
					GroundTruth := calcGroundTruthQuantile(time_window_size, phis_input, w)
					funcExpoHistogramDD(k, dd_acc, time_window_size, phis_input, GroundTruth, w)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done eh_k=%d, window_size=%d, dd_acc=%f===================>\n", k, time_window_size, dd_acc)
					wg.Done()
				}(k, time_window_size, dd_acc)
			}
		}
	}
	wg.Wait()
}

func TestExpoHistogramMinMax(t *testing.T) {
	constructInputTimeSeriesZipf()
	fmt.Println("finished construct input time series")
	runtime.GOMAXPROCS(40)

	ehdd_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	dd_acc_input := []float64{0.1, 0.05, 0.02, 0.01, 0.001}
	time_window_size_input := []int64{100, 1000, 10000, 100000, 1000000}
	phis_input := []float64{0, 1}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1

	for _, time_window_size := range time_window_size_input {
		for _, k := range ehdd_input {
			for _, dd_acc := range dd_acc_input {
				atomic.AddUint64(&ops, add)
				for {
					if ops < 40 {
						break
					}
				}
				testname := fmt.Sprintf("EHDD_%d_%d_%f", time_window_size, k, dd_acc)
				f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					panic(err)
				}
				defer f.Close()
				wg.Add(1)
				go func(k int64, time_window_size int64, dd_acc float64) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					w := bufio.NewWriter(f)
					fmt.Printf("=================>Testing eh_k=%d, window_size=%d, dd_acc=%f===================>\n", k, time_window_size, dd_acc)
					fmt.Fprintf(w, "=================>Testing eh_k=%d, window_size=%d, dd_acc=%f===================>\n", k, time_window_size, dd_acc)
					w.Flush()
					GroundTruth := calcGroundTruthQuantile(time_window_size, phis_input, w)
					funcExpoHistogramDD(k, dd_acc, time_window_size, phis_input, GroundTruth, w)
					w.Flush()
					atomic.AddUint64(&ops, -add)
					fmt.Printf("=================>Done eh_k=%d, window_size=%d, dd_acc=%f===================>\n", k, time_window_size, dd_acc)
					wg.Done()
				}(k, time_window_size, dd_acc)
			}
		}
	}
	wg.Wait()
}

func funcExpoHistogramKLLSubWindow(k int64, kll_k int, time_window_size int64, phis []float64, GroundTruth map[string]([]AnswerQuantile), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "===================Estimating Expo Histogram KLL=======================\n")
	w.Flush()
	t1 := int64(start_t)
	t2 := int64(end_t)

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

	var total_insert_ehkll int64 = 0
	var total_query_ehkll int64 = 0
	var total_queries float64 = 0
	var total_inserts float64 = 0
	var total_mem float64 = 0

	for test_time := 0; test_time < test_times; test_time++ {
		// one EH+dd for one timeseries
		ehs := make(map[string](*ExpoHistogramKLL))
		for _, c := range cases {
			ehs[c.key] = ExpoInitKLL(k, kll_k, time_window_size)
		}

		EHAnswer := make(map[string]([]AnswerQuantile))
		for _, c := range cases {
			EHAnswer[c.key] = make([]AnswerQuantile, 0)
		}
		for t := int64(0); t < time_window_size*2; t++ {
			for _, c := range cases {
				start_insert_ehkll := time.Now()
				ehs[c.key].Update(c.vec[t].T, c.vec[t].F)
				elapsed_insert_ehkll := time.Since(start_insert_ehkll)
				total_insert_ehkll += elapsed_insert_ehkll.Microseconds()
				total_inserts += 1
				if t >= time_window_size {
					start_query_ehkll := time.Now()
					merged_kll := ehs[c.key].QueryIntervalMergeKLL(t-t2, t-t1)
					cdf := merged_kll.CDF()
					q_values := make([]float64, 0)
					for _, phi := range phis {
						q_values = append(q_values, cdf.Query(phi))
					}
					elapsed_query_ehkll := time.Since(start_query_ehkll)
					total_query_ehkll += elapsed_query_ehkll.Microseconds()
					total_queries += 1
					EHAnswer[c.key] = append(EHAnswer[c.key], AnswerQuantile{quantiles: q_values, time: float64(elapsed_query_ehkll.Microseconds()), memory: ehs[c.key].GetMemory()})
					total_mem += ehs[c.key].GetMemory()
				}
			}
		}

		fmt.Fprintf(w, "===============Test %v=================\n", test_time)
		fmt.Fprintf(w, "queries window size: %d (data points)\n", time_window_size)
		w.Flush()

		for _, c := range cases {
			fmt.Fprintf(w, "%s, quantile_errors, query_time(us)\n", c.key)

			for i := 0; i < len(GroundTruth[c.key]); i++ {
				// fmt.Println("** quantile error:", GroundTruth[c.key][i].quantile, EHAnswer[c.key][i].quantile, AbsFloat64(GroundTruth[c.key][i].quantile - EHAnswer[c.key][i].quantile) / GroundTruth[c.key][i].quantile)
				error_quantile := make([]float64, 0)
				for idx := range phis {
					error_quantile = append(error_quantile, AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
					avg_error[c.key][i][idx] += AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx]) / GroundTruth[c.key][i].quantiles[idx]
					max_error[c.key][i][idx] = MaxFloat64(max_error[c.key][i][idx], AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
				}
				// fmt.Fprintf(w, "quantile errors: %v\n", error_quantile)
				w.Flush()
			}
		}
	}

	fmt.Fprintf(w, "===============Summary over %v tests=================\n", test_times)
	fmt.Fprintf(w, "-----------------avg_error-----------------\n")
	avg_error_total := float64(0.0)
	max_error_total := float64(0.0)
	total_avg_count := float64(0.0)
	for _, c := range cases {
		for i := 0; i < len(GroundTruth[c.key]); i++ {
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
	fmt.Fprintf(w, "avg of insert ehdd (us): %f\n", float64(total_insert_ehkll)/total_inserts)
	fmt.Fprintf(w, "avg of query ehdd (us): %f\n", float64(total_query_ehkll)/total_queries)
	fmt.Fprintf(w, "avg of memory ehdd (KB): %f\n", total_mem/total_queries)
	w.Flush()
}

func TestExpoHistogramKLLSubWindow(t *testing.T) {
	// constructInputTimeSeriesZipf()
	readPowerDataset()
	// constructInputTimeSeriesUniformFloat64()
	// readGoogleClusterData2009()
	fmt.Println("finished construct input time series")
	runtime.GOMAXPROCS(40)

	ehk_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	kllk_input := []int{64, 128, 256, 512, 1024}
	time_window_size_input := []int64{1000000}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	phis_input := []float64{0.5}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1

	for _, time_window_size := range time_window_size_input {
		for _, subwindow_size := range subwindow_size_input {
			for _, k := range ehk_input {
				for _, kll_k := range kllk_input {
					atomic.AddUint64(&ops, add)
					for {
						if ops < 35 {
							break
						}
					}
					testname := fmt.Sprintf("EHKLL_%d_%d_%d_%d_%d_0.9_power", time_window_size, k, kll_k, time_window_size-subwindow_size.end, time_window_size-subwindow_size.start)
					f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
					if err != nil {
						panic(err)
					}
					defer f.Close()
					wg.Add(1)
					go func(k int64, time_window_size int64, kll_k int, start_t int64, end_t int64) {
						runtime.LockOSThread()
						defer runtime.UnlockOSThread()
						w := bufio.NewWriter(f)
						fmt.Printf("=================>Testing eh_k=%d, window_size=%d, kll_k=%d, start_t=%d, end_t=%d===================>\n", k, time_window_size, kll_k, start_t, end_t)
						fmt.Fprintf(w, "=================>Testing eh_k=%d, window_size=%d, kll_k=%d, start_t=%d, end_t=%d===================>\n", k, time_window_size, kll_k, start_t, end_t)
						w.Flush()
						GroundTruth := calcGroundTruthQuantileSubWindow(time_window_size, phis_input, w, start_t, end_t)
						funcExpoHistogramKLLSubWindow(k, kll_k, time_window_size, phis_input, GroundTruth, w, start_t, end_t)
						w.Flush()
						atomic.AddUint64(&ops, -add)
						fmt.Printf("=================>Done eh_k=%d, window_size=%d, kll_k=%d, start_t=%d, end_t=%d===================>\n", k, time_window_size, kll_k, start_t, end_t)
						wg.Done()
					}(k, time_window_size, kll_k, subwindow_size.start, subwindow_size.end)
				}
			}
		}
	}
	wg.Wait()
}

func funcExpoHistogramDDSubWindow(k int64, dd_acc float64, time_window_size int64, phis []float64, GroundTruth map[string]([]AnswerQuantile), w *bufio.Writer, start_t, end_t int64) {
	fmt.Fprintf(w, "===================Estimating Expo Histogram DDSketch=======================\n")
	w.Flush()
	t1 := int64(start_t)
	t2 := int64(end_t)

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

	var total_insert_ehdd int64 = 0
	var total_query_ehdd int64 = 0
	var total_queries float64 = 0
	var total_inserts float64 = 0
	var total_mem float64 = 0

	for test_time := 0; test_time < test_times; test_time++ {
		// one EH+dd for one timeseries
		ehs := make(map[string](*ExpoHistogramDD))
		for _, c := range cases {
			ehs[c.key] = ExpoInitDD(k, time_window_size, dd_acc)
		}

		EHAnswer := make(map[string]([]AnswerQuantile))
		for _, c := range cases {
			EHAnswer[c.key] = make([]AnswerQuantile, 0)
		}
		for t := int64(0); t < time_window_size*2; t++ {
			for _, c := range cases {
				start_insert_ehdd := time.Now()
				ehs[c.key].Update(c.vec[t].T, c.vec[t].F)
				elapsed_insert_ehdd := time.Since(start_insert_ehdd)
				total_insert_ehdd += elapsed_insert_ehdd.Microseconds()
				total_inserts += 1
				if t >= time_window_size {
					start_query_ehdd := time.Now()
					merged_dd := ehs[c.key].QueryIntervalMergeDD(t-t2, t-t1)
					q_values, _ := merged_dd.GetValuesAtQuantiles(phis)
					elapsed_query_ehdd := time.Since(start_query_ehdd)
					total_query_ehdd += elapsed_query_ehdd.Microseconds()
					total_queries += 1
					EHAnswer[c.key] = append(EHAnswer[c.key], AnswerQuantile{quantiles: q_values, time: float64(elapsed_query_ehdd.Microseconds()), memory: ehs[c.key].GetMemory()})
					total_mem += ehs[c.key].GetMemory()
				}
			}
		}

		fmt.Fprintf(w, "===============Test %v=================\n", test_time)
		fmt.Fprintf(w, "queries window size: %d (data points)\n", time_window_size)
		w.Flush()
		/*
			for _, c := range cases {
				s_count, bucketsizes := ehs[c.key].get_memory()
				fmt.Fprintf(w, "final ehdd memory: %s : {s_count: %d, bucketsizes: %d \n", c.key, s_count, bucketsizes)
				total_memory := float64(0)
				for _, size := range bucketsizes {
					total_memory += 256 + math.Log(float64(size)/256)/math.Log(2)
				}
				fmt.Fprintf(w, "total_memory (KB): %f\n", total_memory*64/8/1024)
			}
			w.Flush()
		*/

		for _, c := range cases {
			fmt.Fprintf(w, "%s, quantile_errors, query_time(us)\n", c.key)

			for i := 0; i < len(GroundTruth[c.key]); i++ {
				// fmt.Println("** quantile error:", GroundTruth[c.key][i].quantile, EHAnswer[c.key][i].quantile, AbsFloat64(GroundTruth[c.key][i].quantile - EHAnswer[c.key][i].quantile) / GroundTruth[c.key][i].quantile)
				error_quantile := make([]float64, 0)
				for idx := range phis {
					error_quantile = append(error_quantile, AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
					avg_error[c.key][i][idx] += AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx]) / GroundTruth[c.key][i].quantiles[idx]
					max_error[c.key][i][idx] = MaxFloat64(max_error[c.key][i][idx], AbsFloat64(GroundTruth[c.key][i].quantiles[idx]-EHAnswer[c.key][i].quantiles[idx])/GroundTruth[c.key][i].quantiles[idx])
				}
				// fmt.Fprintf(w, "quantile errors: %v\n", error_quantile)
				w.Flush()
			}
		}
	}

	fmt.Fprintf(w, "===============Summary over %v tests=================\n", test_times)
	fmt.Fprintf(w, "-----------------avg_error-----------------\n")
	avg_error_total := float64(0.0)
	max_error_total := float64(0.0)
	total_avg_count := float64(0.0)
	for _, c := range cases {
		for i := 0; i < len(GroundTruth[c.key]); i++ {
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
	fmt.Fprintf(w, "max of avg_error: %f%%\n", avg_error_total/total_avg_count*100)
	fmt.Fprintf(w, "max of max_error: %f%%\n", max_error_total*100)
	fmt.Fprintf(w, "avg of insert ehdd (us): %f\n", float64(total_insert_ehdd)/total_inserts)
	fmt.Fprintf(w, "avg of query ehdd (us): %f\n", float64(total_query_ehdd)/total_queries)
	fmt.Fprintf(w, "avg of memory ehdd (KB): %f\n", total_mem/total_queries)
	w.Flush()
}

type Pair struct {
	start, end int64
}

// test quantile_over_time using DDSketch
func TestExpoHistogramDDSubWindow(t *testing.T) {
	// constructInputTimeSeriesZipf()
	// readPowerDataset()
	// constructInputTimeSeriesUniformFloat64()
	readGoogleClusterData2009()
	fmt.Println("finished construct input time series")
	runtime.GOMAXPROCS(40)

	ehdd_input := []int64{10, 20, 50, 100, 200, 500, 1000}
	dd_acc_input := []float64{0.1, 0.05, 0.02, 0.01, 0.001}
	time_window_size_input := []int64{1000000}
	subwindow_size_input := []Pair{{333333, 666666}, {0, 100000}, {0, 200000}, {0, 300000}, {0, 400000}, {0, 500000}, {0, 600000}, {0, 700000}, {0, 800000}, {0, 900000}}
	phis_input := []float64{0.9}
	var wg sync.WaitGroup
	var ops uint64 = 0
	var add uint64 = 1

	for _, time_window_size := range time_window_size_input {
		for _, subwindow_size := range subwindow_size_input {
			for _, k := range ehdd_input {
				for _, dd_acc := range dd_acc_input {
					atomic.AddUint64(&ops, add)
					for {
						if ops < 20 {
							break
						}
					}
					testname := fmt.Sprintf("EHDD_%d_%d_%f_%d_%d_0.9_google", time_window_size, k, dd_acc, time_window_size-subwindow_size.end, time_window_size-subwindow_size.start)
					f, err := os.OpenFile("./microbenchmark_results/"+testname+".txt", os.O_WRONLY|os.O_CREATE, 0666)
					if err != nil {
						panic(err)
					}
					defer f.Close()
					wg.Add(1)
					go func(k int64, time_window_size int64, dd_acc float64, start_t int64, end_t int64) {
						runtime.LockOSThread()
						defer runtime.UnlockOSThread()
						w := bufio.NewWriter(f)
						fmt.Printf("=================>Testing eh_k=%d, window_size=%d, dd_acc=%f, start_t=%d, end_t=%d===================>\n", k, time_window_size, dd_acc, start_t, end_t)
						fmt.Fprintf(w, "=================>Testing eh_k=%d, window_size=%d, dd_acc=%f, start_t=%d, end_t=%d===================>\n", k, time_window_size, dd_acc, start_t, end_t)
						w.Flush()
						GroundTruth := calcGroundTruthQuantileSubWindow(time_window_size, phis_input, w, start_t, end_t)
						funcExpoHistogramDDSubWindow(k, dd_acc, time_window_size, phis_input, GroundTruth, w, start_t, end_t)
						w.Flush()
						atomic.AddUint64(&ops, -add)
						fmt.Printf("=================>Done eh_k=%d, window_size=%d, dd_acc=%f, start_t=%d, end_t=%d===================>\n", k, time_window_size, dd_acc, start_t, end_t)
						wg.Done()
					}(k, time_window_size, dd_acc, subwindow_size.start, subwindow_size.end)
				}
			}
		}
	}
	wg.Wait()
}
