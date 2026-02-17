package promsketch

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OneOfOne/xxhash"
)

func TestUnivSketch(t *testing.T) {
	fmt.Println("Hello TestUnivSketch")

	cases := []struct {
		key string
		cnt int64
	}{
		{"notfound", 1},
		{"hello", 1},
		{"count", 3},
		{"min", 4},
		{"world", 10},
		{"cheatcheat", 3},
		{"cheatcheat", 7},
		{"min", 2},
		{"hello", 2},
		{"tigger", 34},
		{"flow", 9},
		{"miss", 4},
		{"hello", 30},
		{"world", 10},
		{"hello", 10},
		{"mom", 1},
	}

	/*
		expected := []struct {
			key	string
			cnt	float64
		}{
			{"notfound", 1},
			{"hello", 43},
			{"count", 3},
			{"min", 6},
			{"world", 20},
			{"cheatcheat", 10},
			{"tigger", 34},
			{"flow", 9},
			{"miss", 4},
		}
	*/

	seed1 := make([]uint32, 5)
	seed2 := make([]uint32, 5)
	rand.Seed(time.Now().UnixNano())
	for r := 0; r < 5; r++ {
		seed1[r] = rand.Uint32()
		seed2[r] = rand.Uint32()
	}
	seed3 := rand.Uint32()

	t_now := time.Now()
	s, _ := NewUnivSketch(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS, seed1, seed2, seed3, -1)
	since := time.Since(t_now)
	fmt.Println("univmon creation time =", since)
	fmt.Println(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS)

	// fmt.Println("successfully init universal sketch")

	for _, c := range cases {
		hash := xxhash.ChecksumString64S(c.key, uint64(seed3))
		bottom_layer_num := findBottomLayerNum(hash, CS_LVLS)

		pos, sign := s.cs_layers[0].position_and_sign([]byte(c.key))

		s.univmon_processing(c.key, c.cnt, bottom_layer_num, &pos, &sign)
		/*
			fmt.Println("insert", c.key, c.cnt)

			for _, item := range s.HH_layers[0].topK.heap {
				fmt.Println("!!", item.key, item.count)
			}
			fmt.Println("----------------")
		*/
	}

	got := s.calcL1()

	if got != 131 {
		t.Logf("got %f, expect %d", got, 131)
	}

	card := s.calcCard()

	if card != 10 {
		t.Logf("got card %f, expect %d", card, 10)
	}

	s.Free()
	t_now = time.Now()
	s1, _ := NewUnivSketch(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS, seed1, seed2, seed3, -1)
	since = time.Since(t_now)
	fmt.Println("univmon creation time =", since)
	fmt.Println(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS)

	for _, c := range cases {
		hash := xxhash.ChecksumString64S(c.key, uint64(seed3))
		bottom_layer_num := findBottomLayerNum(hash, CS_LVLS)
		pos, sign := s.cs_layers[0].position_and_sign([]byte(c.key))
		s1.univmon_processing(c.key, c.cnt, bottom_layer_num, &pos, &sign)
		/*
			fmt.Println("insert", c.key, c.cnt)
			for _, item := range s.HH_layers[0].topK.heap {
				fmt.Println("!!", item.key, item.count)
			}
			fmt.Println("----------------")
		*/
	}

	got = s1.calcL1()

	if got != 131 {
		t.Logf("got %f, expect %d", got, 131)
	}

	card = s1.calcCard()

	if card != 10 {
		t.Logf("got card %f, expect %d", card, 10)
	}

	s1.Free()
	t_now = time.Now()
	s2, _ := NewUnivSketch(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS, seed1, seed2, seed3, -1)
	since = time.Since(t_now)
	fmt.Println("univmon creation time =", since)
	fmt.Println(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS)

	for _, c := range cases {
		hash := xxhash.ChecksumString64S(c.key, uint64(seed3))
		bottom_layer_num := findBottomLayerNum(hash, CS_LVLS)
		pos, sign := s.cs_layers[0].position_and_sign([]byte(c.key))
		s2.univmon_processing(c.key, c.cnt, bottom_layer_num, &pos, &sign)
		/*
			fmt.Println("insert", c.key, c.cnt)
			for _, item := range s.HH_layers[0].topK.heap {
				fmt.Println("!!", item.key, item.count)
			}
			fmt.Println("----------------")
		*/
	}

	got = s2.calcL1()

	if got != 131 {
		t.Logf("got %f, expect %d", got, 131)
	}

	card = s2.calcCard()
	if card != 10 {
		t.Logf("got card %f, expect %d", card, 10)
	}
}

func readCAIDADebug() {
	for i := 0; i < 1; i++ {
		f, err := os.Open("./testdata/caida_input.txt")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		t := int64(0)
		vec := make(Vector, 0)
		for scanner.Scan() {
			splits := strings.Fields(scanner.Text())
			T := t
			F, _ := strconv.ParseFloat(strings.TrimSpace(splits[0]), 64)
			vec = append(vec, Sample{T: T, F: F})
			t += 1
		}
		tmp := TestCase{
			key: "caida_input",
			vec: vec,
		}
		cases = append(cases, tmp)
	}
}

func TestUnivCAIDA(t *testing.T) {
	query_window_size := int64(1000000)

	// constructInputTimeSeriesZipf()
	readCAIDADebug()

	seed1 := make([]uint32, 5)
	seed2 := make([]uint32, 5)
	rand.Seed(time.Now().UnixNano())
	for r := 0; r < 5; r++ {
		seed1[r] = rand.Uint32()
		seed2[r] = rand.Uint32()
	}
	seed3 := rand.Uint32()
	univ, _ := NewUnivSketch(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS, seed1, seed2, seed3, -1)
	values := make([]float64, 0)

	for t := int64(0); t < query_window_size; t++ {
		values = append(values, cases[0].vec[t].F)
	}
	gt_distinct, gt_l1, gt_entropy, gt_l2 := gsum(values)

	now := time.Now()
	for t := int64(0); t < query_window_size; t++ {
		key := strconv.FormatFloat(cases[0].vec[t].F, 'f', -1, 64)
		hash := xxhash.ChecksumString64S(key, uint64(univ.seed))
		bottom_layer_num := findBottomLayerNum(hash, CS_LVLS)

		pos, sign := univ.cs_layers[0].position_and_sign([]byte(key))
		univ.univmon_processing(key, 1, bottom_layer_num, &pos, &sign)
	}
	since := time.Since(now)
	fmt.Println("UnivMon update time per item:", float64(since.Microseconds())/float64(query_window_size), "us")

	// univ.PrintHHlayers()

	count := float64(query_window_size)
	distinct := univ.calcCard()
	l1 := univ.calcL1()
	l2 := univ.calcL2()
	entropy := univ.calcEntropy()

	univ_optimized, _ := NewUnivSketchPyramid(TOPK_SIZE, CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, CS_LVLS, seed1, seed2, seed3, -1)
	now = time.Now()
	for t := int64(0); t < query_window_size; t++ {
		key := strconv.FormatFloat(cases[0].vec[t].F, 'f', -1, 64)
		hash := xxhash.ChecksumString64S(key, uint64(univ.seed))
		bottom_layer_num := findBottomLayerNum(hash, CS_LVLS)

		pos, sign := univ_optimized.cs_layers[0].position_and_sign([]byte(key))
		univ_optimized.univmon_processing_optimized(key, 1, bottom_layer_num, &pos, &sign)
	}
	since = time.Since(now)
	fmt.Println("Optimized UnivMon update time per item:", float64(since.Microseconds())/float64(query_window_size), "us")

	// univ_optimized.PrintHHlayers()
	count_op := float64(query_window_size)
	distinct_op := univ_optimized.calcCard()
	l1_op := univ_optimized.calcL1()
	l2_op := univ_optimized.calcL2()
	entropy_op := univ_optimized.calcEntropy()

	fmt.Println("native univ:")
	fmt.Println("approx:", distinct, l1, entropy, l2, count)
	fmt.Println("gt:", gt_distinct, gt_l1, gt_entropy, gt_l2, 1000000)

	rel_err := AbsFloat64(gt_distinct-distinct) / (gt_distinct) * 100
	fmt.Println("distinct err:", rel_err)

	rel_err = AbsFloat64(gt_l1-l1) / (gt_l1) * 100
	fmt.Println("l1 err:", rel_err)

	rel_err = AbsFloat64(gt_entropy-entropy) / (gt_entropy) * 100
	fmt.Println("entropy err:", rel_err)

	rel_err = AbsFloat64(gt_l2-l2) / (gt_l2) * 100
	fmt.Println("l2 err:", rel_err)
	fmt.Println(univ.GetMemoryKB())

	fmt.Println("optimized univ:")
	fmt.Println("approx:", distinct_op, l1_op, entropy_op, l2_op, count_op)
	fmt.Println("gt:", gt_distinct, gt_l1, gt_entropy, gt_l2, 1000000)

	rel_err_op := AbsFloat64(gt_distinct-distinct_op) / (gt_distinct) * 100
	fmt.Println("distinct err:", rel_err_op)

	rel_err_op = AbsFloat64(gt_l1-l1_op) / (gt_l1) * 100
	fmt.Println("l1 err:", rel_err_op)

	rel_err_op = AbsFloat64(gt_entropy-entropy_op) / (gt_entropy) * 100
	fmt.Println("entropy err:", rel_err_op)

	rel_err_op = AbsFloat64(gt_l2-l2_op) / (gt_l2) * 100
	fmt.Println("l2 err:", rel_err_op)
	fmt.Println(univ_optimized.GetMemoryKBPyramid(), "KB")
}
