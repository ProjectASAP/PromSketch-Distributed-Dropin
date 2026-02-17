package promsketch

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func readZipf() {
	filename := "./testdata/zipf_ehuniv.txt"
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	vec := make(Vector, 0)
	lines := 0
	for scanner.Scan() {
		if lines == 10000000 {
			break
		}
		splits := strings.Split(scanner.Text(), " ")
		F, _ := strconv.ParseFloat(strings.TrimSpace(splits[1]), 64)
		T, _ := strconv.ParseFloat(strings.TrimSpace(splits[0]), 64)
		vec = append(vec, Sample{T: int64(T), F: F})
		lines += 1
	}
	key := "zipf"
	tmp := TestCase{
		key: key,
		vec: vec,
	}
	cases = append(cases, tmp)
}

// Test cost (compute + memory) and accuracy under sliding window
func TestCostAnalysisGSumOfflineApproximation(t *testing.T) {

	query_window_size := int64(1000000)
	total_length := int64(10000000)

	// Create a scenario
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

	fmt.Println("t1:", t1)
	fmt.Println("t2:", t2)

	/*
		ground_truth := make([][]gsum_ans, total_length)
		for t := 0; t < int(total_length); t++ {
			ground_truth[t] = make([]gsum_ans, 31)
		}
	*/

	readZipf()
	fmt.Println("Finished reading input timeseries.")
	/*
		filename := "./testdata/zipf_gsum_groundtruth.txt"
		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		lines := 0
		for scanner.Scan() {
			if lines == 0 {
				splits := strings.Split(scanner.Text(), " ")
				for i := 1; i < len(splits); i++ {
					time, _ := strconv.ParseInt(strings.TrimSpace(splits[i]), 10, 64)
					t1 = append(t1, int64(time))
				}
			}
			if lines == 1 {
				splits := strings.Split(scanner.Text(), " ")
				for i := 1; i < len(splits); i++ {
					time, _ := strconv.ParseInt(strings.TrimSpace(splits[i]), 10, 64)
					t2 = append(t2, time)
				}
			}
			if lines > 6 {
				splits := strings.Split(scanner.Text(), " ")
				time, _ := strconv.ParseInt(strings.TrimSpace(splits[0]), 10, 64)
				idx, _ := strconv.ParseInt(strings.TrimSpace(splits[1]), 10, 64)
				distinct, _ := strconv.ParseFloat(strings.TrimSpace(splits[2]), 64)
				l1, _ := strconv.ParseFloat(strings.TrimSpace(splits[3]), 64)
				entropy, _ := strconv.ParseFloat(strings.TrimSpace(splits[4]), 64)
				l2, _ := strconv.ParseFloat(strings.TrimSpace(splits[5]), 64)
				ground_truth[time][idx] = gsum_ans{distinct: distinct, l1: l1, entropy: entropy, l2: l2}
			}
			lines += 1
		}
		fmt.Fprintln(w,"Finished reading input timeseries and ground truth")
		fmt.Fprintln(w,"t1:", t1)
		fmt.Fprintln(w,"t2:", t2)
	*/

	for test_case := 0; test_case < 5; test_case += 1 {
		f, err := os.OpenFile("./testdata/zipf_gsum_sampling_shuniv_larger"+strconv.Itoa(test_case)+".txt", os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w := bufio.NewWriter(f)

		// Sampling baselines
		sampling_rate := []float64{0.001, 0.01, 0.05, 0.1, 0.2, 0.3}
		for _, rate := range sampling_rate {
			fmt.Fprintln(w, "sampling", rate)
			query_compute := 0.0
			insert_compute := 0.0

			sampling_size := int(float64(query_window_size) * rate)
			sampling_instance := NewUniformSampling(query_window_size, rate, sampling_size)
			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				sampling_instance.Insert(t, cases[0].vec[t].F)
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())

				if t == total_length-1 || (t >= query_window_size-1 && (t+1)%cost_query_interval_gsum == 0) {
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

						fmt.Fprintln(w, t, j, distinct, l1, entropy, l2)

						// distinct_rel_err := AbsFloat64(ground_truth[t][j].distinct-distinct) / (ground_truth[t][j].distinct) * 100
						// l1_rel_err := AbsFloat64(ground_truth[t][j].l1-l1) / (ground_truth[t][j].l1) * 100
						// entropy_rel_err := AbsFloat64(ground_truth[t][j].entropy-entropy) / (ground_truth[t][j].entropy) * 100
						// l2_rel_err := AbsFloat64(ground_truth[t][j].l2-l2) / (ground_truth[t][j].l2) * 100
						// fmt.Fprintln(w, t, j, distinct_rel_err, l1_rel_err, entropy_rel_err, l2_rel_err)
					}
				}
			}
			fmt.Fprintln(w, "insert compute:", insert_compute, "us")
			fmt.Fprintln(w, "query compute:", query_compute, "us")
			fmt.Fprintln(w, "total compute:", query_compute+insert_compute, "us")
			fmt.Fprintln(w, "memory:", sampling_instance.GetMemory(), "KB")
		}

		// PromSketch, SHUniv
		beta_input := []float64{0.7071, 0.5, 0.3535, 0.25, 0.177, 0.125, 0.0884, 0.0625, 0.044}
		for _, beta := range beta_input {
			fmt.Fprintln(w, "SHUniv", beta)
			// shu_distinct_error := make([]float64, 0)
			// shu_l1_error := make([]float64, 0)
			// shu_entropy_error := make([]float64, 0)
			// shu_l2_error := make([]float64, 0)
			shu := SmoothInitUnivMon(beta, query_window_size)

			total_compute := 0.0
			insert_compute := 0.0
			for t := int64(0); t < total_length; t++ {
				start := time.Now()
				shu.Update(t, strconv.FormatFloat(cases[0].vec[t].F, 'f', -1, 64))
				elapsed := time.Since(start)
				insert_compute += float64(elapsed.Microseconds())
				fmt.Fprintln(w, t)
				if t == total_length-1 || (t >= query_window_size-1 && (t+1)%cost_query_interval_gsum == 0) {
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

						fmt.Fprintln(w, t, j, distinct, l1, entropy, l2)

						/*
							values := make([]float64, 0)
							for tt := merged_univ.min_time; tt < merged_univ.max_time; tt++ {
								values = append(values, float64(cases[0].vec[tt].F))
							}
							gt_distinct, gt_l1, gt_entropy, gt_l2 := gsum(values)


								distinct_err := AbsFloat64(gt_distinct-distinct) / gt_distinct * 100
								l1_err := AbsFloat64(gt_l1-l1) / gt_l1 * 100
								entropy_err := AbsFloat64(gt_entropy-entropy) / gt_entropy * 100
								l2_err := AbsFloat64(gt_l2-l2) / gt_l2 * 100
								fmt.Fprintln(w,"in time range error:", distinct_err, l1_err, entropy_err, l2_err)
						*/

						// distinct_rel_err := AbsFloat64(ground_truth[t][j].distinct-distinct) / (ground_truth[t][j].distinct) * 100

						// l1_rel_err := AbsFloat64(ground_truth[t][j].l1-l1) / (ground_truth[t][j].l1) * 100

						// entropy_rel_err := AbsFloat64(ground_truth[t][j].entropy-entropy) / (ground_truth[t][j].entropy) * 100

						// l2_rel_err := AbsFloat64(ground_truth[t][j].l2-l2) / (ground_truth[t][j].l2) * 100

						// fmt.Fprintln(w,t, j, distinct_rel_err, l1_rel_err, entropy_rel_err, l2_rel_err)
					}
				}
			}
			// fmt.Fprintln(w,"distinct error:", shu_distinct_error)
			// fmt.Fprintln(w,"l1 error:", shu_l1_error)
			// fmt.Fprintln(w,"entropy error:", shu_entropy_error)
			// fmt.Fprintln(w,"l2 error:", shu_l2_error)

			fmt.Fprintln(w, "insert compute:", insert_compute)
			fmt.Fprintln(w, "query compute:", total_compute, "us")
			fmt.Fprintln(w, "total compute:", total_compute+insert_compute, "us")
			fmt.Fprintln(w, "memory:", shu.GetMemory(), "KB")
		}
	}
}
