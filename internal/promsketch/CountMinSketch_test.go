package promsketch

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestCountMinSketch(t *testing.T) {
	fmt.Println("Hello TestCountMinSketch")
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
		{"hello", -30},
		{"world", 10},
		{"hello", 10},
	}

	expected := []struct {
		key string
		cnt float64
	}{
		{"notfound", 1},
		{"hello", -17},
		{"count", 3},
		{"min", 6},
		{"world", 20},
		{"cheatcheat", 10},
		{"tigger", 34},
		{"flow", 9},
		{"miss", 4},
	}

	seed1 := make([]uint32, CM_ROW_NO)
	rand.Seed(time.Now().UnixNano())
	for r := 0; r < CM_ROW_NO; r++ {
		seed1[r] = rand.Uint32()
	}

	s, _ := NewCountMinSketch(CM_ROW_NO, CM_COL_NO, seed1)

	for _, c := range cases {
		s.CMProcessing(c.key, c.cnt)
	}

	for i, c := range expected {
		got := s.EstimateStringSum(c.key)
		// fmt.Println("key = ", c.key, "cm got = ", got)
		if c.cnt != got {
			t.Logf("case %d '%s' got %f, expect %f", i, c.key, got, c.cnt)
		}
	}
}
