package pipelinecontract

// Effect is how one plugin filter treats one metric.
type Effect string

const (
	// EffectDelivered means the plugin handles the metric after modifiers.
	EffectDelivered Effect = "delivered"
	// EffectBypassed means a processor or aggregator skips the metric and
	// leaves it unchanged for the next stage.
	EffectBypassed Effect = "bypassed"
	// EffectDropped means an input or output does not emit the metric, or
	// modifiers removed every field.
	EffectDropped Effect = "dropped"
)

// Metric is the filter fixture shape. Field values are not stored.
type Metric struct {
	Name   string
	Tags   map[string]string
	Fields map[string]struct{}
}

// AggregateRoute is how one metric meets every aggregator.
//
// Telegraf 1.40.1 offers the metric to each aggregator. A filter miss does
// not stop later aggregators. drop_original on any aggregator that selected
// the metric drops the original, including when modifiers removed its fields.
type AggregateRoute struct {
	Consumed         []string
	DroppedBy        []string
	ForwardsOriginal bool
}

const globUnsupported = "{}[]!\\"

// Evaluate applies one plugin filter using the v1.40.1 selector order:
// namepass, namedrop, tagpass, then tagdrop. Modifiers run only after the
// selectors accept the metric. tagpass matches when any listed tag key has a
// value matching any of its patterns.
func Evaluate(kind PluginKind, filter MetricFilter, metric Metric) (Effect, Metric, error) {
	if err := validateFilter(filter); err != nil {
		return "", Metric{}, err
	}
	selected, err := selectorsPass(filter, metric)
	if err != nil {
		return "", Metric{}, err
	}
	if !selected {
		if kind == KindProcessor || kind == KindAggregator {
			return EffectBypassed, cloneMetric(metric), nil
		}
		return EffectDropped, cloneMetric(metric), nil
	}
	modified, err := applyModifiers(filter, metric)
	if err != nil {
		return "", Metric{}, err
	}
	if len(modified.Fields) == 0 {
		return EffectDropped, modified, nil
	}
	return EffectDelivered, modified, nil
}

// RouteAggregators applies aggregator fan-out to metric.
// instances must already be aggregator instances in declaration order.
func RouteAggregators(instances []Instance, metric Metric) (AggregateRoute, error) {
	route := AggregateRoute{ForwardsOriginal: true}
	for _, instance := range instances {
		if instance.Kind != KindAggregator {
			return AggregateRoute{}, contractError(CodeInvalidDocument, "route expects aggregator instances")
		}
		effect, _, err := Evaluate(KindAggregator, instance.Filter, metric)
		if err != nil {
			return AggregateRoute{}, err
		}
		if effect == EffectBypassed {
			continue
		}
		drop, err := dropOriginal(instance)
		if err != nil {
			return AggregateRoute{}, err
		}
		if drop {
			route.DroppedBy = append(route.DroppedBy, instance.ID)
			route.ForwardsOriginal = false
		}
		if effect == EffectDelivered {
			route.Consumed = append(route.Consumed, instance.ID)
		}
	}
	return route, nil
}

func dropOriginal(instance Instance) (bool, error) {
	value, ok := instance.Options["drop_original"]
	if !ok || value.Kind != KindBool || value.Bool == nil {
		return false, contractError(CodeInvalidOption, "drop_original must be explicit")
	}
	return *value.Bool, nil
}

func selectorsPass(filter MetricFilter, metric Metric) (bool, error) {
	namePass, err := anyPattern(filter.NamePass, metric.Name)
	if err != nil {
		return false, err
	}
	if len(filter.NamePass) > 0 && !namePass {
		return false, nil
	}
	nameDrop, err := anyPattern(filter.NameDrop, metric.Name)
	if err != nil {
		return false, err
	}
	if nameDrop {
		return false, nil
	}
	if filter.TagPass != nil {
		matched, err := tagTableMatches(filter.TagPass, metric.Tags)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	if filter.TagDrop != nil {
		matched, err := tagTableMatches(filter.TagDrop, metric.Tags)
		if err != nil {
			return false, err
		}
		if matched {
			return false, nil
		}
	}
	return true, nil
}

func applyModifiers(filter MetricFilter, metric Metric) (Metric, error) {
	out := cloneMetric(metric)
	for key := range out.Fields {
		keep, err := modifierKeeps(filter.FieldInclude, filter.FieldExclude, key)
		if err != nil {
			return Metric{}, err
		}
		if !keep {
			delete(out.Fields, key)
		}
	}
	for key := range out.Tags {
		keep, err := modifierKeeps(filter.TagInclude, filter.TagExclude, key)
		if err != nil {
			return Metric{}, err
		}
		if !keep {
			delete(out.Tags, key)
		}
	}
	return out, nil
}

func modifierKeeps(include, exclude []string, key string) (bool, error) {
	if len(include) > 0 {
		matched, err := anyPattern(include, key)
		if err != nil || !matched {
			return false, err
		}
	}
	if len(exclude) > 0 {
		matched, err := anyPattern(exclude, key)
		if err != nil || matched {
			return false, err
		}
	}
	return true, nil
}

func tagTableMatches(table map[string][]string, tags map[string]string) (bool, error) {
	for key, patterns := range table {
		value, ok := tags[key]
		if !ok {
			continue
		}
		matched, err := anyPattern(patterns, value)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func anyPattern(patterns []string, value string) (bool, error) {
	for _, pattern := range patterns {
		matched, err := globMatch(pattern, value)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func globMatch(pattern, value string) (bool, error) {
	if stringsContainsUnsupported(pattern) {
		return false, contractError(CodeUnsupportedPattern, "filter pattern uses an unsupported glob")
	}
	return matchRunes([]rune(pattern), []rune(value)), nil
}

func stringsContainsUnsupported(pattern string) bool {
	for _, r := range pattern {
		switch r {
		case '{', '}', '[', ']', '!', '\\':
			return true
		}
	}
	return false
}

func matchRunes(pattern, value []rune) bool {
	for {
		if len(pattern) == 0 {
			return len(value) == 0
		}
		if pattern[0] == '*' {
			pattern = pattern[1:]
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(value); i++ {
				if matchRunes(pattern, value[i:]) {
					return true
				}
			}
			return false
		}
		if len(value) == 0 {
			return false
		}
		if pattern[0] != '?' && pattern[0] != value[0] {
			return false
		}
		pattern = pattern[1:]
		value = value[1:]
	}
}

func cloneMetric(metric Metric) Metric {
	out := Metric{Name: metric.Name, Tags: map[string]string{}, Fields: map[string]struct{}{}}
	for key, value := range metric.Tags {
		out.Tags[key] = value
	}
	for key := range metric.Fields {
		out.Fields[key] = struct{}{}
	}
	return out
}
