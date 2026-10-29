package pipelinecontract

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidGraphIgnoresLayoutAndKeepsExplicitOrder(t *testing.T) {
	original := validGraph()
	moved := validGraph()
	moved.Layout.Nodes[0].Y = 900
	moved.Layout.Nodes[1].Y = 1

	first, err := ExecutionPlan(original)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	second, err := ExecutionPlan(moved)
	if err != nil {
		t.Fatalf("moved plan: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("layout changed the plan:\n%#v\n%#v", first, second)
	}

	want := Plan{
		TelegrafVersion:  SupportedTelegrafVersion,
		Inputs:           []string{"cpu-total", "cpu-per", "mem-main", "disk-local"},
		ProcessorsBefore: []string{"rename-field", "rename-host"},
		Aggregators: []AggregatorPlan{
			{ID: "stats-cpu", Period: "30s", Delay: "100ms", Grace: "1s", DropOriginal: true},
			{ID: "stats-mem", Period: "30s", Delay: "0s", Grace: "0s", DropOriginal: false},
		},
		ProcessorsAfter: []string{"rename-field", "rename-host"},
		Outputs:         []string{"out-file", "out-influx"},
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("plan = %#v, want %#v", first, want)
	}
	if first.ProcessorsBefore[0] == "rename-host" {
		t.Fatal("processor plan followed slice order instead of processor_order")
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := Parse(encoded)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	roundTrip, err := ExecutionPlan(parsed)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if !reflect.DeepEqual(roundTrip, want) {
		t.Fatalf("round trip plan = %#v", roundTrip)
	}
}

func TestParseRejectsVisualRouting(t *testing.T) {
	for _, body := range []string{
		`{"schema_version":1,"telegraf_version":"1.40.1","mode":"graph","edges":[]}`,
		`{"schema_version":1,"telegraf_version":"1.40.1","mode":"graph","graph":{"edges":[]}}`,
	} {
		_, err := Parse([]byte(body))
		if codeOf(err) != CodeInvalidDocument {
			t.Fatalf("body %s code = %q, err = %v", body, codeOf(err), err)
		}
		if !strings.Contains(err.Error(), "edges") {
			t.Fatalf("error %q should name the rejected field", err)
		}
	}
}

func TestRejectsUnsupportedVersionsPluginsAndOptions(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Document)
		code string
	}{
		{"schema", func(doc *Document) { doc.SchemaVersion = 2 }, CodeUnsupportedVersion},
		{"version", func(doc *Document) { doc.TelegrafVersion = "1.40.0" }, CodeUnsupportedVersion},
		{"floating", func(doc *Document) { doc.TelegrafVersion = "latest" }, CodeUnsupportedVersion},
		{"tag spelling", func(doc *Document) { doc.TelegrafVersion = "v1.40.1" }, CodeUnsupportedVersion},
		{"plugin", func(doc *Document) { doc.Graph.Instances[0].Plugin = "net" }, CodeUnsupportedPlugin},
		{"kind", func(doc *Document) { doc.Graph.Instances[0].Kind = KindOutput }, CodeUnsupportedPlugin},
		{"option", func(doc *Document) {
			doc.Graph.Instances[0].Options["energy"] = BoolOption(true)
		}, CodeUnsupportedOption},
		{"processor order field", func(doc *Document) {
			doc.Graph.Instances[4].Options["order"] = IntegerOption(1)
		}, CodeUnsupportedOption},
		{"variance", func(doc *Document) {
			doc.Graph.Instances[6].Options["stats"] = StringListOption("variance")
		}, CodeInvalidOption},
		{"data format", func(doc *Document) {
			doc.Graph.Instances[8].Options["data_format"] = StringOption("prometheus")
		}, CodeInvalidOption},
		{"headers", func(doc *Document) {
			doc.Graph.Instances[9].Options["http_headers"] = StringMapOption(map[string]string{"X": "y"})
		}, CodeUnsupportedOption},
		{"duplicate", func(doc *Document) { doc.Graph.Instances[1].ID = "cpu-total" }, CodeDuplicateInstance},
		{"order missing", func(doc *Document) { doc.Graph.ProcessorOrder = []string{"rename-field"} }, CodeInvalidOrder},
		{"order includes input", func(doc *Document) {
			doc.Graph.ProcessorOrder = []string{"cpu-total", "rename-field"}
		}, CodeInvalidOrder},
		{"both skips", func(doc *Document) {
			doc.Graph.SkipProcessorsBeforeAggregators = BoolPtr(true)
			doc.Graph.SkipProcessorsAfterAggregators = BoolPtr(true)
		}, CodeInvalidDocument},
		{"implicit skip", func(doc *Document) { doc.Graph.SkipProcessorsAfterAggregators = nil }, CodeInvalidDocument},
		{"glob", func(doc *Document) {
			doc.Graph.Instances[6].Filter.NamePass = []string{"cpu[0-9]"}
		}, CodeUnsupportedPattern},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := validGraph()
			tc.edit(&doc)
			err := Validate(doc)
			if codeOf(err) != tc.code {
				t.Fatalf("code = %q, err = %v", codeOf(err), err)
			}
		})
	}
}

func TestSecretReferencesRejectLiterals(t *testing.T) {
	const literal = "placeholder-literal"
	doc := validGraph()
	doc.Graph.Instances[9].Options["token"] = StringOption(literal)
	err := Validate(doc)
	if codeOf(err) != CodeSecretMaterial {
		t.Fatalf("code = %q, err = %v", codeOf(err), err)
	}
	if strings.Contains(err.Error(), literal) {
		t.Fatalf("error echoed the literal: %v", err)
	}

	doc = validGraph()
	doc.Graph.Instances[9].Options["urls"] = StringListOption("http://user:" + literal + "@127.0.0.1:8086")
	err = Validate(doc)
	if codeOf(err) != CodeSecretMaterial {
		t.Fatalf("url code = %q, err = %v", codeOf(err), err)
	}
	if strings.Contains(err.Error(), literal) {
		t.Fatalf("url error echoed the literal: %v", err)
	}

	doc = validGraph()
	doc.Mode = ModeManual
	doc.Graph = nil
	doc.Manual = &Manual{TOML: "[[outputs.influxdb_v2]]\ntoken = \"" + literal + "\"\n"}
	err = Validate(doc)
	if codeOf(err) != CodeSecretMaterial {
		t.Fatalf("manual code = %q, err = %v", codeOf(err), err)
	}
	if strings.Contains(err.Error(), literal) {
		t.Fatalf("manual error echoed the literal: %v", err)
	}

	spec, ok := Lookup(KindOutput, "influxdb_v2")
	if !ok {
		t.Fatal("missing influxdb_v2 spec")
	}
	var token OptionSpec
	for _, option := range spec.Options {
		if option.Key == "token" {
			token = option
		}
	}
	if token.Kind != KindSecretRef || !token.Required {
		t.Fatalf("token spec = %#v", token)
	}
}

func TestManualModeKeepsSecretStoreReferences(t *testing.T) {
	doc := Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeManual,
		Manual:          &Manual{TOML: "[[outputs.file]]\n  token = \"@{toolkit:cred_writer}\"\n"},
	}
	if err := Validate(doc); err != nil {
		t.Fatalf("secret-store reference in manual TOML: %v", err)
	}
}

func TestOmittedOptionsStayOmitted(t *testing.T) {
	doc := validGraph()
	disk := Instance{ID: "disk-plain", Kind: KindInput, Plugin: "disk"}
	doc.Graph.Instances = append(doc.Graph.Instances, disk)
	if err := Validate(doc); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, present := disk.Options["ignore_fs"]; present {
		t.Fatal("ignore_fs was inferred")
	}
	stats := doc.Graph.Instances[7]
	if _, present := stats.Options["stats"]; present {
		t.Fatal("basicstats stats default was inferred")
	}
}

func TestAggregatorFanoutAndFilterFixtures(t *testing.T) {
	both := []Instance{
		aggregator("stats-a", true, "cpu"),
		aggregator("stats-b", false, "cpu"),
	}
	route, err := RouteAggregators(both, metric("cpu", nil, "usage_idle"))
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if !reflect.DeepEqual(route.Consumed, []string{"stats-a", "stats-b"}) {
		t.Fatalf("consumed = %#v", route.Consumed)
	}
	if !reflect.DeepEqual(route.DroppedBy, []string{"stats-a"}) || route.ForwardsOriginal {
		t.Fatalf("route = %#v", route)
	}

	split := []Instance{
		aggregator("stats-mem", true, "mem"),
		aggregator("stats-cpu", true, "cpu"),
	}
	route, err = RouteAggregators(split, metric("cpu", nil, "usage_idle"))
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if !reflect.DeepEqual(route.Consumed, []string{"stats-cpu"}) || route.ForwardsOriginal {
		t.Fatalf("split route = %#v", route)
	}

	miss, err := RouteAggregators(split, metric("disk", nil, "used"))
	if err != nil {
		t.Fatalf("miss: %v", err)
	}
	if len(miss.Consumed) != 0 || !miss.ForwardsOriginal {
		t.Fatalf("miss route = %#v", miss)
	}

	emptied := aggregator("stats-empty", true, "cpu")
	emptied.Filter.FieldExclude = []string{"*"}
	route, err = RouteAggregators([]Instance{emptied}, metric("cpu", nil, "usage_idle"))
	if err != nil {
		t.Fatalf("empty fields: %v", err)
	}
	if len(route.Consumed) != 0 || !reflect.DeepEqual(route.DroppedBy, []string{"stats-empty"}) {
		t.Fatalf("empty route = %#v", route)
	}

	filter := MetricFilter{
		TagPass: map[string][]string{
			"fstype": {"ext4", "xfs"},
			"path":   {"/opt", "/home*"},
		},
	}
	effect, _, err := Evaluate(KindInput, filter, metric("disk", map[string]string{"fstype": "ext4"}, "used"))
	if err != nil || effect != EffectDelivered {
		t.Fatalf("fstype pass effect = %s, err = %v", effect, err)
	}
	effect, _, err = Evaluate(KindInput, filter, metric("disk", map[string]string{"path": "/home/data"}, "used"))
	if err != nil || effect != EffectDelivered {
		t.Fatalf("path pass effect = %s, err = %v", effect, err)
	}
	effect, _, err = Evaluate(KindInput, filter, metric("disk", map[string]string{"fstype": "btrfs", "path": "/var"}, "used"))
	if err != nil || effect != EffectDropped {
		t.Fatalf("tag miss effect = %s, err = %v", effect, err)
	}

	cpuFilter := MetricFilter{
		TagPass: map[string][]string{"cpu": {"cpu0", "cpu1"}},
		TagDrop: map[string][]string{"cpu": {"cpu1"}},
	}
	effect, _, err = Evaluate(KindInput, cpuFilter, metric("cpu", map[string]string{"cpu": "cpu0"}, "usage_idle"))
	if err != nil || effect != EffectDelivered {
		t.Fatalf("cpu0 effect = %s, err = %v", effect, err)
	}
	effect, _, err = Evaluate(KindInput, cpuFilter, metric("cpu", map[string]string{"cpu": "cpu1"}, "usage_idle"))
	if err != nil || effect != EffectDropped {
		t.Fatalf("cpu1 effect = %s, err = %v", effect, err)
	}

	names := MetricFilter{NamePass: []string{"cpu*"}, NameDrop: []string{"cpu_time"}}
	effect, _, err = Evaluate(KindProcessor, names, metric("mem", nil, "used"))
	if err != nil || effect != EffectBypassed {
		t.Fatalf("processor bypass = %s, err = %v", effect, err)
	}
	effect, _, err = Evaluate(KindInput, names, metric("mem", nil, "used"))
	if err != nil || effect != EffectDropped {
		t.Fatalf("input drop = %s, err = %v", effect, err)
	}
	effect, _, err = Evaluate(KindOutput, names, metric("cpu_time", nil, "usage_idle"))
	if err != nil || effect != EffectDropped {
		t.Fatalf("namedrop = %s, err = %v", effect, err)
	}

	fields := MetricFilter{FieldInclude: []string{"usage_*"}, FieldExclude: []string{"usage_guest"}}
	effect, got, err := Evaluate(KindOutput, fields, metric("cpu", nil, "usage_idle", "usage_user", "usage_guest"))
	if err != nil || effect != EffectDelivered {
		t.Fatalf("fields effect = %s, err = %v", effect, err)
	}
	if !reflect.DeepEqual(got.Fields, fieldSet("usage_idle", "usage_user")) {
		t.Fatalf("fields = %#v", got.Fields)
	}

	_, err = globMatch("cpu[0-9]", "cpu1")
	if codeOf(err) != CodeUnsupportedPattern {
		t.Fatalf("glob code = %q, err = %v", codeOf(err), err)
	}
}

func TestProcessorPassesFollowAggregatorPresence(t *testing.T) {
	doc := validGraph()
	doc.Graph.SkipProcessorsBeforeAggregators = BoolPtr(true)
	plan, err := ExecutionPlan(doc)
	if err != nil {
		t.Fatalf("skip before: %v", err)
	}
	if len(plan.ProcessorsBefore) != 0 || !reflect.DeepEqual(plan.ProcessorsAfter, []string{"rename-field", "rename-host"}) {
		t.Fatalf("skip before plan = %#v", plan)
	}

	doc = validGraph()
	doc.Graph.Instances = removeKind(doc.Graph.Instances, KindAggregator)
	plan, err = ExecutionPlan(doc)
	if err != nil {
		t.Fatalf("no aggregators: %v", err)
	}
	if !reflect.DeepEqual(plan.ProcessorsBefore, []string{"rename-field", "rename-host"}) || len(plan.ProcessorsAfter) != 0 || len(plan.Aggregators) != 0 {
		t.Fatalf("no-aggregator plan = %#v", plan)
	}
}

func TestManualModeIsNotConverted(t *testing.T) {
	doc := Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeManual,
		Manual:          &Manual{TOML: "[[inputs.cpu]]\npercpu = true\n"},
		Layout:          &Layout{Nodes: []LayoutNode{{InstanceID: "cpu-total", X: 1, Y: 2}}},
	}
	if err := Validate(doc); err != nil {
		t.Fatalf("validate: %v", err)
	}
	_, err := ExecutionPlan(doc)
	if codeOf(err) != CodeManualNotExecutable {
		t.Fatalf("plan code = %q, err = %v", codeOf(err), err)
	}

	doc.Graph = validGraph().Graph
	err = Validate(doc)
	if codeOf(err) != CodeModeConflict {
		t.Fatalf("conflict code = %q, err = %v", codeOf(err), err)
	}
}

func TestCatalogProvenance(t *testing.T) {
	body := string(mustReadRepo(t, filepath.Join("docs", "contracts", "pipeline.md")))
	for _, needle := range []string{
		UpstreamCommit,
		UpstreamTag,
		CatalogInspectedOn,
		"basicstats.go",
		"agent.go",
		"variance",
		"metricpass",
		"http_headers",
		"manual",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("provenance doc missing %q", needle)
		}
	}
	for _, spec := range SupportedPlugins() {
		if !strings.Contains(body, spec.Name) {
			t.Errorf("provenance doc missing plugin %s", spec.Name)
		}
	}
	stats, ok := Lookup(KindAggregator, "basicstats")
	if !ok {
		t.Fatal("missing basicstats")
	}
	var allowed []string
	for _, option := range stats.Options {
		if option.Key == "stats" {
			allowed = option.Allowed
		}
		if option.Key == "tags" {
			t.Fatal("basicstats tags option is not supported")
		}
	}
	if !reflect.DeepEqual(allowed, BasicStatsNames) {
		t.Fatalf("stats = %#v", allowed)
	}
	for _, name := range allowed {
		if name == "variance" {
			t.Fatal("variance is not a parser stat name")
		}
	}
	foundSource := false
	for _, source := range stats.Sources {
		if strings.HasSuffix(source, "basicstats.go") {
			foundSource = true
		}
	}
	if !foundSource {
		t.Fatalf("sources = %#v", stats.Sources)
	}
}

func TestRenameRuleShape(t *testing.T) {
	doc := validGraph()
	doc.Graph.Instances[4].Options["replace"] = RenameOption(RenameReplacement{Tag: "hostname", Field: "lower", Dest: "host"})
	if codeOf(Validate(doc)) != CodeInvalidOption {
		t.Fatalf("two sources: %v", Validate(doc))
	}
	doc = validGraph()
	doc.Graph.Instances[4].Options["replace"] = RenameOption(RenameReplacement{Dest: "host"})
	if codeOf(Validate(doc)) != CodeInvalidOption {
		t.Fatalf("no source: %v", Validate(doc))
	}
}

func validGraph() Document {
	return Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeGraph,
		Graph: &Graph{
			SkipProcessorsBeforeAggregators: BoolPtr(false),
			SkipProcessorsAfterAggregators:  BoolPtr(false),
			ProcessorOrder:                  []string{"rename-field", "rename-host"},
			Instances: []Instance{
				{ID: "cpu-total", Kind: KindInput, Plugin: "cpu", Options: map[string]OptionValue{
					"percpu":   BoolOption(false),
					"totalcpu": BoolOption(true),
				}},
				{ID: "cpu-per", Kind: KindInput, Plugin: "cpu", Options: map[string]OptionValue{
					"percpu":        BoolOption(true),
					"totalcpu":      BoolOption(false),
					"name_override": StringOption("percpu_usage"),
				}},
				{ID: "mem-main", Kind: KindInput, Plugin: "mem", Options: map[string]OptionValue{
					"collect_extended": BoolOption(false),
				}},
				{ID: "disk-local", Kind: KindInput, Plugin: "disk", Options: map[string]OptionValue{
					"ignore_fs": StringListOption("tmpfs", "devtmpfs"),
				}},
				{ID: "rename-host", Kind: KindProcessor, Plugin: "rename", Options: map[string]OptionValue{
					"replace": RenameOption(RenameReplacement{Tag: "hostname", Dest: "host"}),
				}},
				{ID: "rename-field", Kind: KindProcessor, Plugin: "rename", Options: map[string]OptionValue{
					"replace": RenameOption(RenameReplacement{Field: "lower", Dest: "min"}),
				}},
				{ID: "stats-cpu", Kind: KindAggregator, Plugin: "basicstats", Options: map[string]OptionValue{
					"period":        DurationOption("30s"),
					"delay":         DurationOption("100ms"),
					"grace":         DurationOption("1s"),
					"drop_original": BoolOption(true),
					"stats":         StringListOption("count", "min", "max", "mean", "s2", "stdev"),
				}, Filter: MetricFilter{NamePass: []string{"cpu", "percpu_usage"}}},
				{ID: "stats-mem", Kind: KindAggregator, Plugin: "basicstats", Options: map[string]OptionValue{
					"period":        DurationOption("30s"),
					"delay":         DurationOption("0s"),
					"grace":         DurationOption("0s"),
					"drop_original": BoolOption(false),
				}, Filter: MetricFilter{NamePass: []string{"mem"}}},
				{ID: "out-file", Kind: KindOutput, Plugin: "file", Options: map[string]OptionValue{
					"files":       StringListOption("stdout"),
					"data_format": StringOption("influx"),
				}},
				{ID: "out-influx", Kind: KindOutput, Plugin: "influxdb_v2", Options: map[string]OptionValue{
					"urls":         StringListOption("http://127.0.0.1:8086"),
					"token":        SecretRefOption("cred-influx-v2"),
					"organization": StringOption("toolkit"),
					"bucket":       StringOption("metrics"),
				}, Filter: MetricFilter{NameDrop: []string{"mem"}}},
			},
		},
		Layout: &Layout{Nodes: []LayoutNode{
			{InstanceID: "rename-host", X: 10, Y: 10},
			{InstanceID: "rename-field", X: 10, Y: 400},
		}},
	}
}

func aggregator(id string, drop bool, name string) Instance {
	return Instance{
		ID: id, Kind: KindAggregator, Plugin: "basicstats",
		Options: map[string]OptionValue{
			"period":        DurationOption("30s"),
			"delay":         DurationOption("0s"),
			"grace":         DurationOption("0s"),
			"drop_original": BoolOption(drop),
		},
		Filter: MetricFilter{NamePass: []string{name}},
	}
}

func metric(name string, tags map[string]string, fields ...string) Metric {
	return Metric{Name: name, Tags: tags, Fields: fieldSet(fields...)}
}

func fieldSet(keys ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out
}

func removeKind(instances []Instance, kind PluginKind) []Instance {
	var out []Instance
	for _, instance := range instances {
		if instance.Kind != kind {
			out = append(out, instance)
		}
	}
	return out
}

func codeOf(err error) string {
	var contract *Error
	if errors.As(err, &contract) {
		return contract.Code
	}
	return ""
}

func mustReadRepo(t *testing.T, rel string) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, rel)
		body, err := os.ReadFile(candidate)
		if err == nil {
			return body
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find %s", rel)
	return nil
}
