package promsketch

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestQueryTimeSampling(t *testing.T) {
	readGoogle2019()
	// readPowerDataset()
	total_length := int64(20000000)
	// sliding_window_sizes := []int64{10000, 100000, 1000000, 10000000}
	sliding_window_sizes := []int64{1000000}

	for test_case := 0; test_case < 5; test_case++ {
		filename := "query_time/google2019_20M_avg_sampling_" + strconv.Itoa(test_case) + ".txt"
		fmt.Println(filename)
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w := bufio.NewWriter(f)

		for _, query_window_size := range sliding_window_sizes {
			if query_window_size > total_length {
				break
			}

			cost_query_interval_sampling_avg := int64(query_window_size / 10)
			t1 := make([]int64, 0)
			t2 := make([]int64, 0)
			t1 = append(t1, query_window_size/3)
			t2 = append(t2, query_window_size/3*2)

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

			fmt.Fprintln(w, "t1:", t1)
			fmt.Fprintln(w, "t2:", t2)

			fmt.Fprintln(w, "sliding window size:", query_window_size)
			sampling_rate := []float64{0.01, 0.05, 0.1, 0.2, 0.3}
			// sampling_rate := []float64{0.1}

			for _, rate := range sampling_rate {
				fmt.Fprintln(w, "sampling", rate)

				query_time := make([]float64, len(t1))
				total_query := make([]int64, len(t1))
				gt_query_time := make([]float64, len(t1))
				insert_compute := 0.0
				sampling_size := int(float64(query_window_size) * rate)
				avg_error := make([]float64, len(t1))
				err2 := make([]float64, len(t1))
				total_err_query := make([]float64, len(t1))

				sampling_instance := NewUniformSampling(query_window_size, rate, sampling_size)

				for j := 0; j < len(t1); j++ {
					query_time[j] = 0
					total_query[j] = 0
					total_err_query[j] = 0
					gt_query_time[j] = 0
					avg_error[j] = 0
					err2[j] = 0
				}

				for t := int64(0); t < total_length; t++ {

					start := time.Now()
					sampling_instance.Insert(t, cases[0].vec[t].F)
					elapsed := time.Since(start)
					insert_compute += float64(elapsed.Microseconds())

					if t == total_length-1 || (t >= query_window_size-1 && (t+1)%cost_query_interval_sampling_avg == 0) {
						for j := range len(t1) {

							start_t := t1[j] + t - query_window_size + 1
							end_t := t2[j] + t - query_window_size + 1
							start := time.Now()
							sampling_avg := sampling_instance.QueryAvg(start_t, end_t)
							elapsed := time.Since(start)

							query_time[j] += float64(elapsed.Microseconds())
							total_query[j] += 1

							start = time.Now()
							values := make([]float64, 0)
							for t := start_t; t <= end_t; t++ {
								values = append(values, cases[0].vec[t].F)
							}
							gt_avg := sum(values) / float64(len(values))
							// gt_avg := (sum2(values)/float64(len(values)) - math.Pow(sum(values)/float64(len(values)), 2))
							elapsed1 := time.Since(start)
							gt_query_time[j] += float64(elapsed1.Microseconds())
							// fmt.Println(start_t, end_t, t, t2[j]-t1[j]+1, len(sampling_instance.Arr), elapsed.Microseconds(), elapsed1.Microseconds())
							// fmt.Println(sampling_instance.GetMinTime(), sampling_instance.GetMaxTime())
							// fmt.Fprintln(w, "sampling err:", AbsFloat64(gt_avg-sampling_avg)/gt_avg*100, "window size:", t2[j]-t1[j]+1)

							err := AbsFloat64(gt_avg-sampling_avg) / gt_avg * 100
							if !math.IsNaN(err) {
								avg_error[j] += err
								err2[j] += math.Pow(err, 2)
								total_err_query[j] += 1
							}
						}
					}

				}

				for j := 0; j < len(t1); j++ {
					fmt.Fprintln(w, "sampling err:", avg_error[j]/float64(total_err_query[j]), "window size=", t2[j]-t1[j]+1)
					stdvar := err2[j]/float64(total_err_query[j]) - math.Pow(avg_error[j]/float64(total_err_query[j]), 2)
					stdvar = math.Sqrt(stdvar)
					fmt.Fprintln(w, "sampling stdvar:", stdvar, "window size=", t2[j]-t1[j]+1)
				}

				total_query_compute := 0.0
				total_query_gt_compute := 0.0
				for j := 0; j < len(t1); j++ {
					fmt.Fprintln(w, "sampling estimate query time=", query_time[j]/float64(total_query[j]), "us", "gt query time=", gt_query_time[j]/float64(total_query[j]), "us",
						"window size=", t2[j]-t1[j]+1)
					total_query_compute += query_time[j]
					total_query_gt_compute += gt_query_time[j]
				}

				update_time := float64(insert_compute) / float64(total_length)
				fmt.Fprintln(w, "sampling insert compute:", insert_compute, "us")
				fmt.Fprintln(w, "sampling update time per item:", update_time, "us")
				fmt.Fprintln(w, "sampling query compute:", total_query_compute, "us")
				fmt.Fprintln(w, "gt query compute:", total_query_gt_compute, "us")
				fmt.Fprintln(w, "sampling memory:", sampling_instance.GetMemory(), "KB")
				fmt.Fprintln(w, "gt memory:", float64(query_window_size)*8/1024, "KB")
				w.Flush()
			}
		}
	}
}
