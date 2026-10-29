package pipelinecontract

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const previewSecretStore = "toolkit"

var secretStoreName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

var previewInstanceComment = regexp.MustCompile(`^# toolkit\.instance = "([a-z][a-z0-9-]{0,62})"$`)

var previewHeader = regexp.MustCompile(`^\[\[(inputs|processors|aggregators|outputs)\.([a-z0-9_]+)(?:\.([a-z_]+))?\]\]$`)

// Generate renders a validated graph document as Telegraf 1.40.1 TOML.
// Manual mode is not converted. Layout coordinates are omitted. A secret
// option becomes a toolkit secret-store reference, never a credential value.
func Generate(doc Document) (string, error) {
	if err := Validate(doc); err != nil {
		return "", err
	}
	if doc.Mode != ModeGraph {
		return "", contractError(CodeManualNotExecutable, "manual TOML is stored and is not generated from the graph")
	}
	graph := doc.Graph
	instances, err := orderedInstances(graph)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# telegraf-toolkit graph preview\n")
	b.WriteString("# schema_version = 1\n")
	b.WriteString("# telegraf_version = \"1.40.1\"\n")
	b.WriteString("# layout coordinates are omitted\n")
	b.WriteString("# secret values are omitted; credential options are secret-store references\n")
	b.WriteString("# this preview is not deployed\n")
	b.WriteString("\n[agent]\n")
	writeBool(&b, "skip_processors_before_aggregators", *graph.SkipProcessorsBeforeAggregators)
	writeBool(&b, "skip_processors_after_aggregators", *graph.SkipProcessorsAfterAggregators)
	if len(graph.GlobalTags) > 0 {
		b.WriteString("\n[global_tags]\n")
		keys := sortedKeys(graph.GlobalTags)
		for _, key := range keys {
			if _, err := fmt.Fprintf(&b, "  %s = %s\n", key, quoteTOML(graph.GlobalTags[key])); err != nil {
				return "", err
			}
		}
	}
	for _, instance := range instances {
		if err := writeInstance(&b, instance); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// ParsePreview reads TOML produced by Generate back into a graph document.
// It is not a general Telegraf config importer and does not accept manual mode.
func ParsePreview(toml string) (Document, error) {
	lines := strings.Split(strings.ReplaceAll(toml, "\r\n", "\n"), "\n")
	// A trailing newline produces a final empty line. Drop one trailing empty.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	cursor := 0
	skipBlankAndHeader := func() {
		for cursor < len(lines) {
			line := lines[cursor]
			if line == "" || (strings.HasPrefix(line, "#") && !previewInstanceComment.MatchString(line)) {
				cursor++
				continue
			}
			return
		}
	}
	skipBlankAndHeader()
	if cursor >= len(lines) || lines[cursor] != "[agent]" {
		return Document{}, parseError("preview TOML is missing the agent table")
	}
	cursor++
	doc := Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeGraph,
		Graph: &Graph{
			Instances:      []Instance{},
			ProcessorOrder: []string{},
		},
	}
	flags := map[string]*bool{}
	for cursor < len(lines) {
		line := strings.TrimSpace(lines[cursor])
		if line == "" || strings.HasPrefix(lines[cursor], "#") || strings.HasPrefix(lines[cursor], "[") {
			break
		}
		key, raw, ok := splitKey(lines[cursor])
		if !ok {
			return Document{}, parseError("preview TOML has a line the graph preview cannot parse")
		}
		value, err := parseScalar(raw)
		if err != nil {
			return Document{}, err
		}
		parsed, ok := value.(bool)
		if !ok {
			return Document{}, parseError("preview agent value is not a boolean")
		}
		if _, exists := flags[key]; exists {
			return Document{}, parseError("preview agent table repeats a key")
		}
		flags[key] = BoolPtr(parsed)
		cursor++
	}
	for key := range flags {
		if key != "skip_processors_before_aggregators" && key != "skip_processors_after_aggregators" {
			return Document{}, parseError("preview agent table has an unexpected key")
		}
	}
	if flags["skip_processors_before_aggregators"] == nil || flags["skip_processors_after_aggregators"] == nil {
		return Document{}, parseError("preview agent table is missing processor pass flags")
	}
	doc.Graph.SkipProcessorsBeforeAggregators = flags["skip_processors_before_aggregators"]
	doc.Graph.SkipProcessorsAfterAggregators = flags["skip_processors_after_aggregators"]
	skipBlankAndHeader()
	if cursor < len(lines) && lines[cursor] == "[global_tags]" {
		cursor++
		tags := map[string]string{}
		for cursor < len(lines) {
			if lines[cursor] == "" || strings.HasPrefix(lines[cursor], "#") || strings.HasPrefix(lines[cursor], "[") {
				break
			}
			key, raw, ok := splitKey(lines[cursor])
			if !ok {
				return Document{}, parseError("preview TOML has a line the graph preview cannot parse")
			}
			value, err := parseScalar(raw)
			if err != nil {
				return Document{}, err
			}
			text, ok := value.(string)
			if !ok {
				return Document{}, parseError("preview global tag value is not a string")
			}
			tags[key] = text
			cursor++
		}
		doc.Graph.GlobalTags = tags
	}
	for cursor < len(lines) {
		skipBlankAndHeader()
		if cursor >= len(lines) {
			break
		}
		match := previewInstanceComment.FindStringSubmatch(lines[cursor])
		if match == nil {
			return Document{}, parseError("preview TOML is missing an instance marker")
		}
		id := match[1]
		cursor++
		skipBlankAndHeader()
		if cursor >= len(lines) {
			return Document{}, parseError("preview instance is missing a plugin table")
		}
		header := previewHeader.FindStringSubmatch(lines[cursor])
		if header == nil || header[3] != "" {
			return Document{}, parseError("preview instance is missing a plugin table")
		}
		kind, err := kindFromHeader(header[1])
		if err != nil {
			return Document{}, err
		}
		cursor++
		instance := Instance{ID: id, Kind: kind, Plugin: header[2], Options: map[string]OptionValue{}}
		for cursor < len(lines) {
			if lines[cursor] == "" {
				cursor++
				continue
			}
			if previewInstanceComment.MatchString(lines[cursor]) {
				break
			}
			if strings.HasPrefix(lines[cursor], "#") {
				cursor++
				continue
			}
			nested := previewHeader.FindStringSubmatch(strings.TrimSpace(lines[cursor]))
			if nested != nil && nested[3] == "replace" {
				rule, next, err := parseReplace(lines, cursor+1)
				if err != nil {
					return Document{}, err
				}
				current := instance.Options["replace"]
				if current.Kind == "" {
					current = RenameOption()
				}
				current.Replaces = append(current.Replaces, rule)
				instance.Options["replace"] = current
				cursor = next
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(lines[cursor]), "[") {
				break
			}
			key, raw, ok := splitKey(lines[cursor])
			if !ok {
				return Document{}, parseError("preview TOML has a line the graph preview cannot parse")
			}
			if err := assignField(&instance, key, raw); err != nil {
				return Document{}, err
			}
			cursor++
		}
		if len(instance.Options) == 0 {
			instance.Options = nil
		}
		doc.Graph.Instances = append(doc.Graph.Instances, instance)
		if instance.Kind == KindProcessor {
			doc.Graph.ProcessorOrder = append(doc.Graph.ProcessorOrder, instance.ID)
		}
	}
	if err := Validate(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func orderedInstances(graph *Graph) ([]Instance, error) {
	processors := map[string]Instance{}
	var inputs, aggregators, outputs []Instance
	for _, instance := range graph.Instances {
		switch instance.Kind {
		case KindInput:
			inputs = append(inputs, instance)
		case KindProcessor:
			processors[instance.ID] = instance
		case KindAggregator:
			aggregators = append(aggregators, instance)
		case KindOutput:
			outputs = append(outputs, instance)
		default:
			return nil, contractError(CodeInvalidDocument, "plugin kind is not supported")
		}
	}
	ordered := make([]Instance, 0, len(graph.Instances))
	ordered = append(ordered, inputs...)
	for _, id := range graph.ProcessorOrder {
		ordered = append(ordered, processors[id])
	}
	ordered = append(ordered, aggregators...)
	ordered = append(ordered, outputs...)
	return ordered, nil
}

func writeInstance(b *strings.Builder, instance Instance) error {
	b.WriteString("\n# toolkit.instance = " + quoteTOML(instance.ID) + "\n")
	header, err := tablePrefix(instance.Kind)
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "[[%s.%s]]\n", header, instance.Plugin)
	spec, ok := Lookup(instance.Kind, instance.Plugin)
	if !ok {
		return contractError(CodeUnsupportedPlugin, "plugin is not in the verified Telegraf 1.40.1 catalog")
	}
	for _, option := range spec.Options {
		value, exists := instance.Options[option.Key]
		if !exists {
			continue
		}
		if err := writeOption(b, header, instance.Plugin, option, value); err != nil {
			return err
		}
	}
	writeFilter(b, instance.Filter)
	return nil
}

func writeOption(b *strings.Builder, header, plugin string, option OptionSpec, value OptionValue) error {
	switch option.Kind {
	case KindBool:
		writeBool(b, option.Key, *value.Bool)
	case KindString:
		fmt.Fprintf(b, "  %s = %s\n", option.Key, quoteTOML(*value.String))
	case KindInteger:
		fmt.Fprintf(b, "  %s = %d\n", option.Key, *value.Integer)
	case KindDuration:
		fmt.Fprintf(b, "  %s = %s\n", option.Key, quoteTOML(*value.Duration))
	case KindStringList:
		fmt.Fprintf(b, "  %s = %s\n", option.Key, quoteList(value.Strings))
	case KindStringMap:
		fmt.Fprintf(b, "  %s = %s\n", option.Key, quoteStringMap(value.StringMap))
	case KindSecretRef:
		ref := *value.SecretRef
		if !secretStoreName.MatchString(ref) {
			return contractError(CodeInvalidOption, "credential reference cannot be encoded as a Telegraf 1.40.1 secret-store reference")
		}
		fmt.Fprintf(b, "  %s = %s\n", option.Key, quoteTOML("@{"+previewSecretStore+":"+ref+"}"))
	case KindRenameReplace:
		for _, rule := range value.Replaces {
			fmt.Fprintf(b, "  [[%s.%s.replace]]\n", header, plugin)
			switch {
			case rule.Measurement != "":
				fmt.Fprintf(b, "    measurement = %s\n", quoteTOML(rule.Measurement))
			case rule.Tag != "":
				fmt.Fprintf(b, "    tag = %s\n", quoteTOML(rule.Tag))
			case rule.Field != "":
				fmt.Fprintf(b, "    field = %s\n", quoteTOML(rule.Field))
			}
			fmt.Fprintf(b, "    dest = %s\n", quoteTOML(rule.Dest))
		}
	default:
		return contractError(CodeInvalidOption, "option kind is not supported")
	}
	return nil
}

func writeFilter(b *strings.Builder, filter MetricFilter) {
	writeStringList(b, "namepass", filter.NamePass)
	writeStringList(b, "namedrop", filter.NameDrop)
	if filter.TagPass != nil {
		fmt.Fprintf(b, "  tagpass = %s\n", quotePatternMap(filter.TagPass))
	}
	if filter.TagDrop != nil {
		fmt.Fprintf(b, "  tagdrop = %s\n", quotePatternMap(filter.TagDrop))
	}
	writeStringList(b, "fieldinclude", filter.FieldInclude)
	writeStringList(b, "fieldexclude", filter.FieldExclude)
	writeStringList(b, "taginclude", filter.TagInclude)
	writeStringList(b, "tagexclude", filter.TagExclude)
}

func writeStringList(b *strings.Builder, key string, values []string) {
	if values == nil {
		return
	}
	fmt.Fprintf(b, "  %s = %s\n", key, quoteList(values))
}

func writeBool(b *strings.Builder, key string, value bool) {
	text := "false"
	if value {
		text = "true"
	}
	fmt.Fprintf(b, "  %s = %s\n", key, text)
}

func quoteList(values []string) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = quoteTOML(value)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func quoteStringMap(values map[string]string) string {
	keys := sortedKeys(values)
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = key + " = " + quoteTOML(values[key])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func quotePatternMap(values map[string][]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = key + " = " + quoteList(values[key])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quoteTOML(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func tablePrefix(kind PluginKind) (string, error) {
	switch kind {
	case KindInput:
		return "inputs", nil
	case KindProcessor:
		return "processors", nil
	case KindAggregator:
		return "aggregators", nil
	case KindOutput:
		return "outputs", nil
	default:
		return "", contractError(CodeInvalidDocument, "plugin kind is not supported")
	}
}

func kindFromHeader(prefix string) (PluginKind, error) {
	switch prefix {
	case "inputs":
		return KindInput, nil
	case "processors":
		return KindProcessor, nil
	case "aggregators":
		return KindAggregator, nil
	case "outputs":
		return KindOutput, nil
	default:
		return "", parseError("preview plugin table is not a supported kind")
	}
}

func splitKey(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	key, raw, ok := strings.Cut(trimmed, " = ")
	if !ok || key == "" || strings.Contains(key, " ") {
		return "", "", false
	}
	return key, raw, true
}

func parseReplace(lines []string, start int) (RenameReplacement, int, error) {
	rule := RenameReplacement{}
	sources := 0
	cursor := start
	for cursor < len(lines) {
		if lines[cursor] == "" || strings.HasPrefix(strings.TrimSpace(lines[cursor]), "[") || strings.HasPrefix(lines[cursor], "#") {
			break
		}
		key, raw, ok := splitKey(lines[cursor])
		if !ok {
			return RenameReplacement{}, 0, parseError("preview TOML has a line the graph preview cannot parse")
		}
		value, err := parseScalar(raw)
		if err != nil {
			return RenameReplacement{}, 0, err
		}
		text, ok := value.(string)
		if !ok {
			return RenameReplacement{}, 0, parseError("preview rename value is not a string")
		}
		switch key {
		case "measurement":
			rule.Measurement = text
			sources++
		case "tag":
			rule.Tag = text
			sources++
		case "field":
			rule.Field = text
			sources++
		case "dest":
			rule.Dest = text
		default:
			return RenameReplacement{}, 0, contractError(CodeUnsupportedOption, "option is not in the verified catalog")
		}
		cursor++
	}
	if sources != 1 || rule.Dest == "" {
		return RenameReplacement{}, 0, contractError(CodeInvalidOption, "rename rule needs one source and dest")
	}
	return rule, cursor, nil
}

func assignField(instance *Instance, key, raw string) error {
	switch key {
	case "namepass":
		values, err := expectStringList(raw)
		if err != nil {
			return err
		}
		instance.Filter.NamePass = values
		return nil
	case "namedrop":
		values, err := expectStringList(raw)
		if err != nil {
			return err
		}
		instance.Filter.NameDrop = values
		return nil
	case "fieldinclude":
		values, err := expectStringList(raw)
		if err != nil {
			return err
		}
		instance.Filter.FieldInclude = values
		return nil
	case "fieldexclude":
		values, err := expectStringList(raw)
		if err != nil {
			return err
		}
		instance.Filter.FieldExclude = values
		return nil
	case "taginclude":
		values, err := expectStringList(raw)
		if err != nil {
			return err
		}
		instance.Filter.TagInclude = values
		return nil
	case "tagexclude":
		values, err := expectStringList(raw)
		if err != nil {
			return err
		}
		instance.Filter.TagExclude = values
		return nil
	case "tagpass":
		values, err := expectPatternMap(raw)
		if err != nil {
			return err
		}
		instance.Filter.TagPass = values
		return nil
	case "tagdrop":
		values, err := expectPatternMap(raw)
		if err != nil {
			return err
		}
		instance.Filter.TagDrop = values
		return nil
	}
	spec, ok := Lookup(instance.Kind, instance.Plugin)
	if !ok {
		return contractError(CodeUnsupportedPlugin, "plugin is not in the verified Telegraf 1.40.1 catalog")
	}
	var option OptionSpec
	found := false
	for _, candidate := range spec.Options {
		if candidate.Key == key {
			option = candidate
			found = true
			break
		}
	}
	if !found {
		return contractError(CodeUnsupportedOption, "option is not in the verified catalog")
	}
	value, err := parseScalar(raw)
	if err != nil {
		return err
	}
	switch option.Kind {
	case KindBool:
		parsed, ok := value.(bool)
		if !ok {
			return contractError(CodeInvalidOption, "option kind does not match the catalog")
		}
		instance.Options[key] = BoolOption(parsed)
	case KindString:
		parsed, ok := value.(string)
		if !ok {
			return contractError(CodeInvalidOption, "option kind does not match the catalog")
		}
		instance.Options[key] = StringOption(parsed)
	case KindInteger:
		parsed, ok := value.(int64)
		if !ok {
			return contractError(CodeInvalidOption, "option kind does not match the catalog")
		}
		instance.Options[key] = IntegerOption(parsed)
	case KindDuration:
		parsed, ok := value.(string)
		if !ok {
			return contractError(CodeInvalidOption, "option kind does not match the catalog")
		}
		instance.Options[key] = DurationOption(parsed)
	case KindStringList:
		parsed, err := expectStringList(raw)
		if err != nil {
			return err
		}
		instance.Options[key] = StringListOption(parsed...)
	case KindStringMap:
		parsed, err := expectStringMap(raw)
		if err != nil {
			return err
		}
		instance.Options[key] = StringMapOption(parsed)
	case KindSecretRef:
		parsed, ok := value.(string)
		if !ok {
			return contractError(CodeSecretMaterial, "secret option accepts only a credential reference")
		}
		ref, ok := secretReference(parsed)
		if !ok {
			return contractError(CodeSecretMaterial, "secret option accepts only a credential reference")
		}
		instance.Options[key] = SecretRefOption(ref)
	default:
		return contractError(CodeUnsupportedOption, "option is not in the verified catalog")
	}
	return nil
}

func secretReference(value string) (string, bool) {
	prefix := "@{" + previewSecretStore + ":"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "}") {
		return "", false
	}
	ref := strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}")
	if !secretStoreName.MatchString(ref) {
		return "", false
	}
	return ref, true
}

func expectStringList(raw string) ([]string, error) {
	value, err := parseScalar(raw)
	if err != nil {
		return nil, err
	}
	parsed, ok := value.([]string)
	if !ok {
		return nil, contractError(CodeInvalidOption, "option kind does not match the catalog")
	}
	return parsed, nil
}

func expectStringMap(raw string) (map[string]string, error) {
	value, err := parseScalar(raw)
	if err != nil {
		return nil, err
	}
	parsed, ok := value.(map[string]string)
	if !ok {
		return nil, contractError(CodeInvalidOption, "option kind does not match the catalog")
	}
	return parsed, nil
}

func expectPatternMap(raw string) (map[string][]string, error) {
	value, err := parseScalar(raw)
	if err != nil {
		return nil, err
	}
	parsed, ok := value.(map[string][]string)
	if !ok {
		return nil, contractError(CodeInvalidFilter, "tag filter key is invalid")
	}
	return parsed, nil
}

func parseScalar(raw string) (any, error) {
	switch raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if strings.HasPrefix(raw, "\"") {
		return unquoteTOML(raw)
	}
	if strings.HasPrefix(raw, "[") {
		return parseStringArray(raw)
	}
	if strings.HasPrefix(raw, "{") {
		return parseInlineTable(raw)
	}
	if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return parsed, nil
	}
	return nil, parseError("preview TOML has a value the graph preview cannot parse")
}

func parseStringArray(raw string) ([]string, error) {
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, parseError("preview TOML has a value the graph preview cannot parse")
	}
	body := strings.TrimSpace(raw[1 : len(raw)-1])
	if body == "" {
		return []string{}, nil
	}
	parts, err := splitTop(body)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(parts))
	for i, part := range parts {
		text, err := unquoteTOML(part)
		if err != nil {
			return nil, err
		}
		out[i] = text
	}
	return out, nil
}

func parseInlineTable(raw string) (any, error) {
	if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return nil, parseError("preview TOML has a value the graph preview cannot parse")
	}
	body := strings.TrimSpace(raw[1 : len(raw)-1])
	if body == "" {
		return map[string]string{}, nil
	}
	parts, err := splitTop(body)
	if err != nil {
		return nil, err
	}
	stringsOnly := true
	for _, part := range parts {
		_, value, ok := splitKey(part)
		if !ok {
			return nil, parseError("preview TOML has a value the graph preview cannot parse")
		}
		if strings.HasPrefix(value, "[") {
			stringsOnly = false
		}
	}
	if stringsOnly {
		out := map[string]string{}
		for _, part := range parts {
			key, value, ok := splitKey(part)
			if !ok {
				return nil, parseError("preview TOML has a value the graph preview cannot parse")
			}
			text, err := unquoteTOML(value)
			if err != nil {
				return nil, err
			}
			out[key] = text
		}
		return out, nil
	}
	out := map[string][]string{}
	for _, part := range parts {
		key, value, ok := splitKey(part)
		if !ok {
			return nil, parseError("preview TOML has a value the graph preview cannot parse")
		}
		list, err := parseStringArray(value)
		if err != nil {
			return nil, err
		}
		out[key] = list
	}
	return out, nil
}

func splitTop(body string) ([]string, error) {
	var parts []string
	start := 0
	depth := 0
	inString := false
	escaped := false
	for index, r := range body {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(body[start:index]))
				start = index + 1
			}
		}
	}
	if inString || depth != 0 {
		return nil, parseError("preview TOML has a value the graph preview cannot parse")
	}
	parts = append(parts, strings.TrimSpace(body[start:]))
	return parts, nil
}

func unquoteTOML(raw string) (string, error) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", parseError("preview TOML has a value the graph preview cannot parse")
	}
	var b strings.Builder
	escaped := false
	for index := 1; index < len(raw)-1; {
		r, size := utf8.DecodeRuneInString(raw[index:])
		index += size
		if escaped {
			switch r {
			case '\\', '"':
				b.WriteRune(r)
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if index+3 >= len(raw)-1 {
					return "", parseError("preview TOML has a value the graph preview cannot parse")
				}
				parsed, err := strconv.ParseUint(raw[index:index+4], 16, 32)
				if err != nil {
					return "", parseError("preview TOML has a value the graph preview cannot parse")
				}
				b.WriteRune(rune(parsed))
				index += 4
			default:
				return "", parseError("preview TOML has a value the graph preview cannot parse")
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		return "", parseError("preview TOML has a value the graph preview cannot parse")
	}
	return b.String(), nil
}
