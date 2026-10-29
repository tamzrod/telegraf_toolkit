package pipelinecontract

// OptionSpec is one plugin option verified from the Telegraf 1.40.1 tree.
// Specs carry no sample secret and no default that would be written into a
// document when the user omits the option.
type OptionSpec struct {
	Key               string
	Kind              ValueKind
	Required          bool
	Allowed           []string
	MinInteger        *int64
	AllowZeroDuration bool
	URL               bool
	NonEmpty          bool
}

// PluginSpec is one supported plugin in the focused 1.40.1 catalog.
type PluginSpec struct {
	Kind    PluginKind
	Name    string
	Options []OptionSpec
	Sources []string
}

// SharedSources are the Telegraf 1.40.1 files that define execution order,
// filtering and aggregator behavior for every plugin in the catalog.
var SharedSources = []string{
	"docs/CONFIGURATION.md",
	"models/filter.go",
	"filter/filter.go",
	"agent/agent.go",
	"config/config.go",
}

// BasicStatsNames are the stats strings accepted by
// plugins/aggregators/basicstats/basicstats.go parseStats at v1.40.1.
// The sample comment lists "variance"; the parser name for that statistic
// is "s2". Unrecognized names are not accepted here.
var BasicStatsNames = []string{
	"count",
	"diff",
	"first",
	"interval",
	"last",
	"max",
	"mean",
	"min",
	"non_negative_diff",
	"non_negative_rate",
	"percent_change",
	"rate",
	"s2",
	"stdev",
	"sum",
}

var catalogByKey map[pluginKey]PluginSpec

type pluginKey struct {
	kind PluginKind
	name string
}

func init() {
	catalogByKey = make(map[pluginKey]PluginSpec, len(catalogPlugins))
	for _, spec := range catalogPlugins {
		key := pluginKey{spec.Kind, spec.Name}
		if _, exists := catalogByKey[key]; exists {
			panic("duplicate pipeline catalog plugin " + string(spec.Kind) + "." + spec.Name)
		}
		seen := make(map[string]struct{}, len(spec.Options))
		for _, option := range spec.Options {
			if _, exists := seen[option.Key]; exists {
				panic("duplicate option " + option.Key + " on " + spec.Name)
			}
			seen[option.Key] = struct{}{}
		}
		catalogByKey[key] = spec
	}
}

// Lookup returns a copy of the verified spec for a plugin kind and name.
func Lookup(kind PluginKind, name string) (PluginSpec, bool) {
	spec, ok := catalogByKey[pluginKey{kind, name}]
	if !ok {
		return PluginSpec{}, false
	}
	return copySpec(spec), true
}

// SupportedPlugins returns copies of the catalog in definition order.
func SupportedPlugins() []PluginSpec {
	out := make([]PluginSpec, len(catalogPlugins))
	for i, spec := range catalogPlugins {
		out[i] = copySpec(spec)
	}
	return out
}

func copySpec(spec PluginSpec) PluginSpec {
	spec.Options = append([]OptionSpec(nil), spec.Options...)
	spec.Sources = append([]string(nil), spec.Sources...)
	for i := range spec.Options {
		spec.Options[i].Allowed = append([]string(nil), spec.Options[i].Allowed...)
		if spec.Options[i].MinInteger != nil {
			value := *spec.Options[i].MinInteger
			spec.Options[i].MinInteger = &value
		}
	}
	return spec
}

func optionMap(spec PluginSpec) map[string]OptionSpec {
	out := make(map[string]OptionSpec, len(spec.Options))
	for _, option := range spec.Options {
		out[option.Key] = option
	}
	return out
}

func intMin(v int64) *int64 {
	return &v
}

func withCommon(common []OptionSpec, specific ...OptionSpec) []OptionSpec {
	out := make([]OptionSpec, 0, len(common)+len(specific))
	out = append(out, common...)
	out = append(out, specific...)
	return out
}

var inputCommon = []OptionSpec{
	{Key: "alias", Kind: KindString, NonEmpty: true},
	{Key: "interval", Kind: KindDuration},
	{Key: "name_override", Kind: KindString, NonEmpty: true},
	{Key: "name_prefix", Kind: KindString, NonEmpty: true},
	{Key: "name_suffix", Kind: KindString, NonEmpty: true},
	{Key: "tags", Kind: KindStringMap},
}

var processorCommon = []OptionSpec{
	{Key: "alias", Kind: KindString, NonEmpty: true},
}

var aggregatorCommon = []OptionSpec{
	{Key: "alias", Kind: KindString, NonEmpty: true},
	{Key: "name_override", Kind: KindString, NonEmpty: true},
	{Key: "name_prefix", Kind: KindString, NonEmpty: true},
	{Key: "name_suffix", Kind: KindString, NonEmpty: true},
	{Key: "period", Kind: KindDuration, Required: true},
	{Key: "delay", Kind: KindDuration, Required: true, AllowZeroDuration: true},
	{Key: "grace", Kind: KindDuration, Required: true, AllowZeroDuration: true},
	{Key: "drop_original", Kind: KindBool, Required: true},
}

var outputCommon = []OptionSpec{
	{Key: "alias", Kind: KindString, NonEmpty: true},
	{Key: "name_override", Kind: KindString, NonEmpty: true},
	{Key: "name_prefix", Kind: KindString, NonEmpty: true},
	{Key: "name_suffix", Kind: KindString, NonEmpty: true},
}

var catalogPlugins = []PluginSpec{
	{
		Kind: KindInput, Name: "cpu",
		Sources: []string{"plugins/inputs/cpu/README.md"},
		Options: withCommon(inputCommon,
			OptionSpec{Key: "percpu", Kind: KindBool},
			OptionSpec{Key: "totalcpu", Kind: KindBool},
			OptionSpec{Key: "collect_cpu_time", Kind: KindBool},
			OptionSpec{Key: "report_active", Kind: KindBool},
			OptionSpec{Key: "core_tags", Kind: KindBool},
		),
	},
	{
		Kind: KindInput, Name: "mem",
		Sources: []string{"plugins/inputs/mem/README.md"},
		Options: withCommon(inputCommon,
			OptionSpec{Key: "collect_extended", Kind: KindBool},
		),
	},
	{
		Kind: KindInput, Name: "disk",
		Sources: []string{"plugins/inputs/disk/README.md"},
		Options: withCommon(inputCommon,
			OptionSpec{Key: "mount_points", Kind: KindStringList},
			OptionSpec{Key: "ignore_fs", Kind: KindStringList},
			OptionSpec{Key: "ignore_mount_opts", Kind: KindStringList},
		),
	},
	{
		Kind: KindProcessor, Name: "rename",
		Sources: []string{"plugins/processors/rename/README.md"},
		Options: withCommon(processorCommon,
			OptionSpec{Key: "replace", Kind: KindRenameReplace},
		),
	},
	{
		Kind: KindAggregator, Name: "basicstats",
		Sources: []string{
			"plugins/aggregators/basicstats/README.md",
			"plugins/aggregators/basicstats/basicstats.go",
		},
		Options: withCommon(aggregatorCommon,
			OptionSpec{Key: "stats", Kind: KindStringList, Allowed: BasicStatsNames},
		),
	},
	{
		Kind: KindOutput, Name: "file",
		Sources: []string{"plugins/outputs/file/README.md"},
		Options: withCommon(outputCommon,
			OptionSpec{Key: "files", Kind: KindStringList, Required: true},
			OptionSpec{Key: "use_batch_format", Kind: KindBool},
			OptionSpec{Key: "rotation_interval", Kind: KindDuration, AllowZeroDuration: true},
			OptionSpec{Key: "rotation_max_archives", Kind: KindInteger, MinInteger: intMin(-1)},
			OptionSpec{Key: "data_format", Kind: KindString, Required: true, NonEmpty: true, Allowed: []string{"influx"}},
		),
	},
	{
		Kind: KindOutput, Name: "influxdb_v2",
		Sources: []string{"plugins/outputs/influxdb_v2/README.md"},
		Options: withCommon(outputCommon,
			OptionSpec{Key: "urls", Kind: KindStringList, Required: true, URL: true},
			OptionSpec{Key: "token", Kind: KindSecretRef, Required: true},
			OptionSpec{Key: "organization", Kind: KindString, Required: true, NonEmpty: true},
			OptionSpec{Key: "bucket", Kind: KindString, Required: true, NonEmpty: true},
			OptionSpec{Key: "timeout", Kind: KindDuration},
			OptionSpec{Key: "content_encoding", Kind: KindString, Allowed: []string{"gzip", "identity"}},
			OptionSpec{Key: "influx_uint_support", Kind: KindBool},
			OptionSpec{Key: "concurrent_writes", Kind: KindInteger, MinInteger: intMin(1)},
		),
	},
}
