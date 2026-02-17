package promsketch

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCSHeap(t *testing.T) {
	seed1 := make([]uint32, CS_ROW_NO_Univ_ELEPHANT)
	seed2 := make([]uint32, CS_ROW_NO_Univ_ELEPHANT)
	rand.Seed(time.Now().UnixNano())
	for r := 0; r < CS_ROW_NO_Univ_ELEPHANT; r++ {
		seed1[r] = rand.Uint32()
		seed2[r] = rand.Uint32()
	}

	t_now := time.Now()
	cs, err := NewCountSketch(CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, seed1, seed2)
	since := time.Since(t_now)
	t.Log(since)
	require.NoError(t, err)
	var s float64 = 2
	var v float64 = 1
	var RAND *rand.Rand = rand.New(rand.NewSource(time.Now().Unix()))
	z := rand.NewZipf(RAND, s, v, uint64(value_scale))
	vec := make(Vector, 0)

	t2 := 1000000
	for t := 0; t < t2; t++ {
		value := float64(z.Uint64())
		// TODO: DDSketch and KLL only works with positive float64 currently
		vec = append(vec, Sample{T: int64(t), F: value})
	}
	l1Map := make(map[float64]float64)
	for t := 0; t < t2; t++ {
		if _, ok := l1Map[vec[t].F]; ok {
			l1Map[vec[t].F] += 1
		} else {
			l1Map[vec[t].F] = 1
		}

	}

	var l1 float64 = 0.0
	var l2 float64 = 0.0
	var entropynorm float64 = 0.0
	start := time.Now()
	for _, value := range l1Map {
		l1 += value
		entropynorm += value * math.Log(value) / math.Log(2)
		l2 += value * value
	}
	elapsed := time.Since(start)
	fmt.Println("total baseline query time=", elapsed)
	// entropy := math.Log(float64(t2))/math.Log(2) - entropynorm/float64(t2)
	l2 = math.Sqrt(l2)
	// card := float64(len(l1Map))

	for t := 0; t < t2; t++ {
		cs.UpdateString(strconv.FormatFloat(vec[t].F, 'f', -1, 64), 1)
	}
	start = time.Now()
	l2_cs := cs.cs_l2()
	l1_cs := int64(0)
	for _, item := range cs.topK.heap {
		l1_cs += item.count
	}
	elapsed = time.Since(start)
	fmt.Println("total CS query time=", elapsed)
	l2_err := AbsFloat64(l2_cs-l2) / l2
	fmt.Println("l2 err:", l2_err*100, "%")
	l1_err := AbsFloat64(float64(l1_cs)-l1) / l1
	fmt.Println("l1 err:", l1_err*100, "%")
}

func TestCountSketch(t *testing.T) {
	fmt.Println("Hello TestCountSketch")
	cases := []struct {
		key string
		cnt float64
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
	}

	seed1 := make([]uint32, 5)
	seed2 := make([]uint32, 5)
	rand.Seed(time.Now().UnixNano())
	for r := 0; r < 3; r++ {
		seed1[r] = rand.Uint32()
		seed2[r] = rand.Uint32()
	}

	s, err := NewCountSketch(CS_ROW_NO_Univ_ELEPHANT, CS_COL_NO_Univ_ELEPHANT, seed1, seed2)
	require.NoError(t, err)

	for _, c := range cases {
		s.UpdateString(c.key, c.cnt)
	}

	err = s.FreeCountSketch()
	require.NoError(t, err)
}
