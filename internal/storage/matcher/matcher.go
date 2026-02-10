package matcher

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/promsketch/promsketch-dropin/internal/config"
)

// Matcher evaluates if a time series matches sketch target configuration
type Matcher struct {
	targets []*compiledTarget
}

// compiledTarget represents a compiled sketch target matcher
type compiledTarget struct {
	original  *config.SketchTarget
	matchType matchType
	// For exact match
	exactName string
	// For regex match
	nameRegex *regexp.Regexp
	// For label matchers
	labelMatchers []*labelMatcher
	// For wildcard
	isWildcard bool
}

type matchType int

const (
	matchExact matchType = iota
	matchRegex
	matchLabels
	matchWildcard
)

// labelMatcher represents a single label matcher
type labelMatcher struct {
	name      string
	matchType labelMatchType
	value     string
	regex     *regexp.Regexp
}

type labelMatchType int

const (
	labelMatchEqual labelMatchType = iota
	labelMatchNotEqual
	labelMatchRegex
	labelMatchNotRegex
)

// NewMatcher creates a new matcher from sketch targets configuration
func NewMatcher(targets []config.SketchTarget) (*Matcher, error) {
	compiled := make([]*compiledTarget, 0, len(targets))

	for i, target := range targets {
		ct, err := compileTarget(&target)
		if err != nil {
			return nil, fmt.Errorf("failed to compile target %d: %w", i, err)
		}
		compiled = append(compiled, ct)
	}

	return &Matcher{
		targets: compiled,
	}, nil
}

// compileTarget compiles a sketch target into a compiled matcher
func compileTarget(target *config.SketchTarget) (*compiledTarget, error) {
	ct := &compiledTarget{
		original: target,
	}

	match := strings.TrimSpace(target.Match)

	// Check for wildcard
	if match == "*" {
		ct.matchType = matchWildcard
		ct.isWildcard = true
		return ct, nil
	}

	// Check for label matchers: {...}
	if strings.HasPrefix(match, "{") && strings.HasSuffix(match, "}") {
		matchers, err := parseLabelMatchers(match)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label matchers: %w", err)
		}
		ct.matchType = matchLabels
		ct.labelMatchers = matchers
		return ct, nil
	}

	// Check if it's a regex pattern (contains regex metacharacters)
	if containsRegexMetachars(match) {
		regex, err := regexp.Compile("^" + match + "$")
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		ct.matchType = matchRegex
		ct.nameRegex = regex
		return ct, nil
	}

	// Exact match
	ct.matchType = matchExact
	ct.exactName = match
	return ct, nil
}

// containsRegexMetachars checks if a string contains regex metacharacters
func containsRegexMetachars(s string) bool {
	metachars := []string{".*", ".+", ".*", "[", "]", "(", ")", "^", "$", "|", "?", "+", "\\"}
	for _, mc := range metachars {
		if strings.Contains(s, mc) {
			return true
		}
	}
	return false
}

// parseLabelMatchers parses a PromQL-style label matcher expression
// Format: {__name__="metric_name", label1="value1", label2=~"regex.*"}
func parseLabelMatchers(expr string) ([]*labelMatcher, error) {
	// Remove surrounding braces
	expr = strings.TrimPrefix(expr, "{")
	expr = strings.TrimSuffix(expr, "}")
	expr = strings.TrimSpace(expr)

	if expr == "" {
		return nil, nil
	}

	// Split by comma (simple parser, doesn't handle quoted commas)
	parts := strings.Split(expr, ",")
	matchers := make([]*labelMatcher, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		matcher, err := parseSingleLabelMatcher(part)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, matcher)
	}

	return matchers, nil
}

// parseSingleLabelMatcher parses a single label matcher
// Formats: label="value", label!="value", label=~"regex", label!~"regex"
func parseSingleLabelMatcher(expr string) (*labelMatcher, error) {
	matcher := &labelMatcher{}

	// Check for regex matchers
	if strings.Contains(expr, "=~") {
		parts := strings.SplitN(expr, "=~", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid regex matcher: %s", expr)
		}
		matcher.name = strings.TrimSpace(parts[0])
		matcher.matchType = labelMatchRegex
		matcher.value = strings.Trim(strings.TrimSpace(parts[1]), `"`)

		regex, err := regexp.Compile(matcher.value)
		if err != nil {
			return nil, fmt.Errorf("invalid regex in matcher %s: %w", expr, err)
		}
		matcher.regex = regex
		return matcher, nil
	}

	if strings.Contains(expr, "!~") {
		parts := strings.SplitN(expr, "!~", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid negative regex matcher: %s", expr)
		}
		matcher.name = strings.TrimSpace(parts[0])
		matcher.matchType = labelMatchNotRegex
		matcher.value = strings.Trim(strings.TrimSpace(parts[1]), `"`)

		regex, err := regexp.Compile(matcher.value)
		if err != nil {
			return nil, fmt.Errorf("invalid regex in matcher %s: %w", expr, err)
		}
		matcher.regex = regex
		return matcher, nil
	}

	// Check for exact matchers
	if strings.Contains(expr, "!=") {
		parts := strings.SplitN(expr, "!=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid not-equal matcher: %s", expr)
		}
		matcher.name = strings.TrimSpace(parts[0])
		matcher.matchType = labelMatchNotEqual
		matcher.value = strings.Trim(strings.TrimSpace(parts[1]), `"`)
		return matcher, nil
	}

	if strings.Contains(expr, "=") {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid equal matcher: %s", expr)
		}
		matcher.name = strings.TrimSpace(parts[0])
		matcher.matchType = labelMatchEqual
		matcher.value = strings.Trim(strings.TrimSpace(parts[1]), `"`)
		return matcher, nil
	}

	return nil, fmt.Errorf("invalid label matcher format: %s", expr)
}

// Matches checks if a label set matches any of the configured sketch targets
func (m *Matcher) Matches(lbls labels.Labels) (*config.SketchTarget, bool) {
	for _, target := range m.targets {
		if m.matchesTarget(lbls, target) {
			return target.original, true
		}
	}
	return nil, false
}

// matchesTarget checks if labels match a single compiled target
func (m *Matcher) matchesTarget(lbls labels.Labels, target *compiledTarget) bool {
	switch target.matchType {
	case matchWildcard:
		return true

	case matchExact:
		metricName := lbls.Get(labels.MetricName)
		return metricName == target.exactName

	case matchRegex:
		metricName := lbls.Get(labels.MetricName)
		return target.nameRegex.MatchString(metricName)

	case matchLabels:
		return m.matchesLabels(lbls, target.labelMatchers)

	default:
		return false
	}
}

// matchesLabels checks if labels match all label matchers
func (m *Matcher) matchesLabels(lbls labels.Labels, matchers []*labelMatcher) bool {
	for _, matcher := range matchers {
		if !m.matchesSingleLabel(lbls, matcher) {
			return false
		}
	}
	return true
}

// matchesSingleLabel checks if labels match a single label matcher
func (m *Matcher) matchesSingleLabel(lbls labels.Labels, matcher *labelMatcher) bool {
	labelValue := lbls.Get(matcher.name)

	switch matcher.matchType {
	case labelMatchEqual:
		return labelValue == matcher.value

	case labelMatchNotEqual:
		return labelValue != matcher.value

	case labelMatchRegex:
		return matcher.regex.MatchString(labelValue)

	case labelMatchNotRegex:
		return !matcher.regex.MatchString(labelValue)

	default:
		return false
	}
}

// TargetCount returns the number of configured targets
func (m *Matcher) TargetCount() int {
	return len(m.targets)
}
