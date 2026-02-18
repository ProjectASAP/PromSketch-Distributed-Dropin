package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Pattern constants (matching Rust reference)
const (
	sinePeriodSecs    = 120.0 // 2 minute cycle
	stepPeriodSecs    = 30.0  // step changes every 30s
	linearWrapSecs    = 300.0 // linear resets every 5min
	sinePhaseVar      = 0.1   // phase offset per series
	sineAmpVar        = 0.2   // amplitude varies ±20%
	linearSlopeVar    = 0.1   // slope varies ±10%
	constantNumLevels = 10    // 10 distinct constant values
	noiseStddevFrac   = 0.1   // noise is 10% of valuescale
	spikeProbability  = 0.05  // 5% chance per scrape
	spikeMagnitude    = 5.0   // spike is 5x baseline
	stepNumLevels     = 4     // 4 discrete levels
)

var validPatterns = []string{"constant", "sine", "sine_noise", "linear_up", "step", "spiky"}

type exporter struct {
	metricName  string
	pattern     string
	valuescale  float64
	labelNames  []string
	labelCombos [][]string // each element is one label-value combination

	mu  sync.Mutex
	rng *rand.Rand
}

func main() {
	port := flag.Int("port", 9101, "HTTP port")
	metricName := flag.String("metric-name", "fake_metric", "Metric name")
	pattern := flag.String("pattern", "sine", "Data pattern: constant, sine, sine_noise, linear_up, step, spiky")
	valuescale := flag.Float64("valuescale", 100, "Max value magnitude")
	numLabels := flag.Int("num-labels", 2, "Number of labels")
	valuesPerLabel := flag.String("values-per-label", "3,5", "Comma-separated cardinality per label")
	labelNamesFlag := flag.String("label-names", "", "Comma-separated label names (default auto: label_0,label_1,...)")
	flag.Parse()

	// Validate pattern
	if !isValidPattern(*pattern) {
		fmt.Fprintf(os.Stderr, "invalid pattern %q; valid: %s\n", *pattern, strings.Join(validPatterns, ", "))
		os.Exit(1)
	}

	// Parse values-per-label
	vpl := parseValuesPerLabel(*valuesPerLabel, *numLabels)

	// Build label names
	names := buildLabelNames(*labelNamesFlag, *numLabels)

	// Build Cartesian product of label values
	combos := cartesianProduct(*numLabels, vpl)

	exp := &exporter{
		metricName:  *metricName,
		pattern:     *pattern,
		valuescale:  *valuescale,
		labelNames:  names,
		labelCombos: combos,
		rng:         rand.New(rand.NewPCG(0, 0)),
	}

	totalSeries := 1
	for _, v := range vpl {
		totalSeries *= v
	}
	fmt.Printf("fake-exporter: pattern=%s series=%d port=%d\n", *pattern, totalSeries, *port)

	http.HandleFunc("/metrics", exp.handleMetrics)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/metrics", http.StatusMovedPermanently)
	})

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func isValidPattern(p string) bool {
	for _, v := range validPatterns {
		if p == v {
			return true
		}
	}
	return false
}

func parseValuesPerLabel(s string, numLabels int) []int {
	parts := strings.Split(s, ",")
	vals := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			fmt.Fprintf(os.Stderr, "invalid values-per-label %q\n", p)
			os.Exit(1)
		}
		vals = append(vals, n)
	}
	if len(vals) == 1 {
		v := vals[0]
		vals = make([]int, numLabels)
		for i := range vals {
			vals[i] = v
		}
	}
	if len(vals) != numLabels {
		fmt.Fprintf(os.Stderr, "values-per-label count (%d) != num-labels (%d)\n", len(vals), numLabels)
		os.Exit(1)
	}
	return vals
}

func buildLabelNames(s string, numLabels int) []string {
	if s == "" {
		names := make([]string, numLabels)
		for i := range names {
			names[i] = fmt.Sprintf("label_%d", i)
		}
		return names
	}
	parts := strings.Split(s, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			names = append(names, p)
		}
	}
	if len(names) != numLabels {
		fmt.Fprintf(os.Stderr, "label-names count (%d) != num-labels (%d)\n", len(names), numLabels)
		os.Exit(1)
	}
	return names
}

func cartesianProduct(numLabels int, vpl []int) [][]string {
	// Build per-label value lists
	pools := make([][]string, numLabels)
	for i := 0; i < numLabels; i++ {
		pool := make([]string, vpl[i])
		for j := 0; j < vpl[i]; j++ {
			pool[j] = fmt.Sprintf("value_%d_%d", i, j)
		}
		pools[i] = pool
	}
	// Cartesian product
	result := [][]string{{}}
	for _, pool := range pools {
		var next [][]string
		for _, prefix := range result {
			for _, item := range pool {
				combo := make([]string, len(prefix)+1)
				copy(combo, prefix)
				combo[len(prefix)] = item
				next = append(next, combo)
			}
		}
		result = next
	}
	return result
}

func (e *exporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# HELP %s Fake metric with %s pattern\n", e.metricName, e.pattern))
	b.WriteString(fmt.Sprintf("# TYPE %s gauge\n", e.metricName))

	for seriesID, combo := range e.labelCombos {
		val := e.sample(seriesID)
		b.WriteString(e.metricName)
		b.WriteByte('{')
		for i, name := range e.labelNames {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(name)
			b.WriteString(`="`)
			b.WriteString(combo[i])
			b.WriteByte('"')
		}
		b.WriteString("} ")
		b.WriteString(strconv.FormatFloat(val, 'f', 6, 64))
		b.WriteByte('\n')
	}

	w.Write([]byte(b.String()))
}

func (e *exporter) sample(seriesID int) float64 {
	switch e.pattern {
	case "constant":
		return e.sampleConstant(seriesID)
	case "sine":
		return e.sampleSine(seriesID)
	case "sine_noise":
		return e.sampleSineNoise(seriesID)
	case "linear_up":
		return e.sampleLinearUp(seriesID)
	case "step":
		return e.sampleStep(seriesID)
	case "spiky":
		return e.sampleSpiky(seriesID)
	default:
		return 0
	}
}

func nowSecs() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

func (e *exporter) sampleConstant(seriesID int) float64 {
	level := seriesID % constantNumLevels
	base := e.valuescale / float64(constantNumLevels)
	return base * (float64(level) + 0.5)
}

func (e *exporter) sampleSine(seriesID int) float64 {
	t := nowSecs()
	phase := float64(seriesID%100) * sinePhaseVar
	ampMul := 1.0 + (float64(seriesID%5)-2.0)*sineAmpVar
	amplitude := (e.valuescale / 2.0) * ampMul
	offset := e.valuescale / 2.0
	angle := (2.0*math.Pi*t/sinePeriodSecs) + phase
	return offset + amplitude*math.Sin(angle)
}

func (e *exporter) sampleSineNoise(seriesID int) float64 {
	base := e.sampleSine(seriesID)
	stddev := e.valuescale * noiseStddevFrac
	e.mu.Lock()
	noise := e.rng.NormFloat64() * stddev
	e.mu.Unlock()
	v := base + noise
	if v < 0 {
		v = 0
	}
	return v
}

func (e *exporter) sampleLinearUp(seriesID int) float64 {
	t := nowSecs()
	slopeMul := 1.0 + float64(seriesID%10)*linearSlopeVar
	slope := (e.valuescale / linearWrapSecs) * slopeMul
	return math.Mod(t*slope, e.valuescale)
}

func (e *exporter) sampleStep(seriesID int) float64 {
	t := nowSecs()
	phaseOffset := float64(seriesID%stepNumLevels) * (stepPeriodSecs / float64(stepNumLevels))
	adjusted := t + phaseOffset
	stepIdx := int(adjusted/stepPeriodSecs) % stepNumLevels
	levelHeight := e.valuescale / float64(stepNumLevels)
	return levelHeight * (float64(stepIdx) + 0.5)
}

func (e *exporter) sampleSpiky(seriesID int) float64 {
	baseline := e.valuescale * 0.2 * (1.0 + float64(seriesID%5)*0.1)
	e.mu.Lock()
	r := e.rng.Float64()
	e.mu.Unlock()
	if r < spikeProbability {
		return baseline * spikeMagnitude
	}
	return baseline
}
