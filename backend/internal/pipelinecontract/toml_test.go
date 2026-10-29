package pipelinecontract

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratePreviewMatchesFixtureAndRoundTrips(t *testing.T) {
	doc := previewDocument()
	doc.Layout = &Layout{Nodes: []LayoutNode{{InstanceID: "cpu-1", X: 12345, Y: 67890}}}
	first, err := Generate(doc)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := Generate(doc)
	if err != nil {
		t.Fatalf("generate again: %v", err)
	}
	if first != second {
		t.Fatal("generate is not deterministic")
	}
	want, err := os.ReadFile("../../../ui/src/features/pipelines/graph-preview.toml")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if first != string(want) {
		t.Fatalf("preview =\n%s\nwant =\n%s", first, want)
	}
	if strings.Contains(first, "12345") || strings.Contains(first, "67890") || strings.Contains(first, "layout =") {
		t.Fatal("layout coordinates leaked into TOML")
	}
	parsed, err := ParsePreview(first)
	if err != nil {
		t.Fatalf("parse preview: %v", err)
	}
	if parsed.Layout != nil {
		t.Fatal("preview restored layout")
	}
	again, err := Generate(parsed)
	if err != nil || again != first {
		t.Fatalf("round trip changed the preview: %v\n%s", err, again)
	}
	if strings.Join(parsed.Graph.ProcessorOrder, ",") != "rename-2,rename-1" {
		t.Fatalf("processor order = %v", parsed.Graph.ProcessorOrder)
	}
}

func TestGenerateAggregatorPreviewRoundTrips(t *testing.T) {
	doc := Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeGraph,
		Graph: &Graph{
			SkipProcessorsBeforeAggregators: BoolPtr(false),
			SkipProcessorsAfterAggregators:  BoolPtr(false),
			ProcessorOrder:                  []string{"rename-1"},
			Instances: []Instance{
				{ID: "cpu-1", Kind: KindInput, Plugin: "cpu", Options: map[string]OptionValue{
					"percpu": BoolOption(false),
				}},
				{ID: "rename-1", Kind: KindProcessor, Plugin: "rename", Options: map[string]OptionValue{
					"replace": RenameOption(RenameReplacement{Tag: "cpu", Dest: "core"}),
				}},
				{ID: "stats-1", Kind: KindAggregator, Plugin: "basicstats", Options: map[string]OptionValue{
					"period":        DurationOption("30s"),
					"delay":         DurationOption("100ms"),
					"grace":         DurationOption("0s"),
					"drop_original": BoolOption(false),
					"stats":         StringListOption("mean", "s2"),
				}},
				{ID: "file-1", Kind: KindOutput, Plugin: "file", Options: map[string]OptionValue{
					"files":       StringListOption("stdout"),
					"data_format": StringOption("influx"),
				}},
			},
		},
	}
	const want = `# telegraf-toolkit graph preview
# schema_version = 1
# telegraf_version = "1.40.1"
# layout coordinates are omitted
# secret values are omitted; credential options are secret-store references
# this preview is not deployed

[agent]
  skip_processors_before_aggregators = false
  skip_processors_after_aggregators = false

# toolkit.instance = "cpu-1"
[[inputs.cpu]]
  percpu = false

# toolkit.instance = "rename-1"
[[processors.rename]]
  [[processors.rename.replace]]
    tag = "cpu"
    dest = "core"

# toolkit.instance = "stats-1"
[[aggregators.basicstats]]
  period = "30s"
  delay = "100ms"
  grace = "0s"
  drop_original = false
  stats = ["mean", "s2"]

# toolkit.instance = "file-1"
[[outputs.file]]
  files = ["stdout"]
  data_format = "influx"
`
	got, err := Generate(doc)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if got != want {
		t.Fatalf("preview =\n%s", got)
	}
	parsed, err := ParsePreview(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	again, err := Generate(parsed)
	if err != nil || again != got {
		t.Fatalf("round trip: %v", err)
	}
}

func TestGenerateUsesProcessorOrderAndSecretReference(t *testing.T) {
	doc := validGraph()
	doc.Graph.Instances[9].Options["token"] = SecretRefOption("cred_influx_v2")
	out, err := Generate(doc)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	field := strings.Index(out, `# toolkit.instance = "rename-field"`)
	host := strings.Index(out, `# toolkit.instance = "rename-host"`)
	if field < 0 || host < 0 || field > host {
		t.Fatal("processor tables followed slice order instead of processor_order")
	}
	if !strings.Contains(out, `token = "@{toolkit:cred_influx_v2}"`) {
		t.Fatal("secret reference was not encoded")
	}
	if strings.Contains(out, "cred-influx") || strings.Contains(out, `token = ""`) {
		t.Fatal("generated TOML contained a rejected credential form")
	}
	if strings.Contains(out, "namepass = [\"cpu\", \"percpu_usage\"]") == false {
		t.Fatal("aggregator filter was omitted")
	}
	parsed, err := ParsePreview(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	again, err := Generate(parsed)
	if err != nil || again != out {
		t.Fatalf("rich round trip changed the preview: %v", err)
	}
}

func TestGenerateRejectsUnencodableReferenceWithoutEchoingIt(t *testing.T) {
	literal := "cred-placeholder-literal"
	doc := validGraph()
	doc.Graph.Instances[9].Options["token"] = SecretRefOption(literal)
	_, err := Generate(doc)
	if codeOf(err) != CodeInvalidOption || CheckClassOf(err) != CheckConfig {
		t.Fatalf("class/code = %s %s (%v)", CheckClassOf(err), codeOf(err), err)
	}
	if strings.Contains(err.Error(), literal) {
		t.Fatal("unencodable reference was echoed")
	}
}

func TestGenerateDoesNotConvertManualMode(t *testing.T) {
	doc := Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeManual,
		Manual:          &Manual{TOML: "[agent]\n"},
	}
	_, err := Generate(doc)
	if codeOf(err) != CodeManualNotExecutable || CheckClassOf(err) != CheckConfig {
		t.Fatalf("manual generate = %s %v", CheckClassOf(err), err)
	}
}

func TestCheckClasses(t *testing.T) {
	_, err := Parse([]byte("{"))
	if CheckClassOf(err) != CheckParse {
		t.Fatalf("json class = %s (%v)", CheckClassOf(err), err)
	}
	_, err = Parse([]byte(`{"schema_version":1,"telegraf_version":"1.40.1","mode":"graph","edges":[]}`))
	if CheckClassOf(err) != CheckParse || !strings.Contains(err.Error(), "edges") {
		t.Fatalf("unknown field class = %s (%v)", CheckClassOf(err), err)
	}
	doc := previewDocument()
	doc.Graph.Instances[0].Plugin = "kafka"
	_, err = Generate(doc)
	if codeOf(err) != CodeUnsupportedPlugin || CheckClassOf(err) != CheckSupport {
		t.Fatalf("support class = %s %v", CheckClassOf(err), err)
	}
	doc = previewDocument()
	both := true
	doc.Graph.SkipProcessorsBeforeAggregators = &both
	doc.Graph.SkipProcessorsAfterAggregators = &both
	_, err = Generate(doc)
	if CheckClassOf(err) != CheckConfig {
		t.Fatalf("config class = %s (%v)", CheckClassOf(err), err)
	}
	secret := "placeholder-literal"
	leaky := strings.Join([]string{
		"[agent]",
		"  skip_processors_before_aggregators = false",
		"  skip_processors_after_aggregators = false",
		"",
		"# toolkit.instance = \"influx-1\"",
		"[[outputs.influxdb_v2]]",
		"  urls = [\"http://127.0.0.1:8086\"]",
		"  token = \"" + secret + "\"",
		"  organization = \"toolkit\"",
		"  bucket = \"metrics\"",
		"",
	}, "\n")
	_, err = ParsePreview(leaky)
	if codeOf(err) != CodeSecretMaterial || strings.Contains(err.Error(), secret) {
		t.Fatalf("secret parse = %v", err)
	}
	_, err = ParsePreview("not toml")
	if CheckClassOf(err) != CheckParse {
		t.Fatalf("bad preview class = %s (%v)", CheckClassOf(err), err)
	}
}

func previewDocument() Document {
	return Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeGraph,
		Graph: &Graph{
			SkipProcessorsBeforeAggregators: BoolPtr(false),
			SkipProcessorsAfterAggregators:  BoolPtr(false),
			ProcessorOrder:                  []string{"rename-2", "rename-1"},
			Instances: []Instance{
				{ID: "cpu-1", Kind: KindInput, Plugin: "cpu", Options: map[string]OptionValue{
					"alias": StringOption("edge-host"),
				}},
				{ID: "rename-1", Kind: KindProcessor, Plugin: "rename", Options: map[string]OptionValue{
					"replace": RenameOption(RenameReplacement{Tag: "hostname", Dest: "host"}),
				}},
				{ID: "rename-2", Kind: KindProcessor, Plugin: "rename"},
				{ID: "file-1", Kind: KindOutput, Plugin: "file", Options: map[string]OptionValue{
					"files":       StringListOption("stdout"),
					"data_format": StringOption("influx"),
				}},
			},
		},
	}
}
