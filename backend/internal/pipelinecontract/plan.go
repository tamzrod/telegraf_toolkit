package pipelinecontract

// Plan is the Telegraf 1.40.1 execution shape of a graph document.
//
// Inputs are independent sources. Outputs are independent writers. Processor
// order is the explicit processor_order list, applied before aggregators and
// again after them unless that pass is skipped. Aggregators are fan-out in
// declaration order, not a canvas path. The plan has no coordinates.
type Plan struct {
	TelegrafVersion  string
	Inputs           []string
	ProcessorsBefore []string
	Aggregators      []AggregatorPlan
	ProcessorsAfter  []string
	Outputs          []string
}

// AggregatorPlan records the aggregator settings that change delivery.
// Period, delay and grace are the user's strings. The time window is not
// simulated, because membership depends on agent start time.
type AggregatorPlan struct {
	ID           string
	Period       string
	Delay        string
	Grace        string
	DropOriginal bool
}

// ExecutionPlan validates doc and returns its execution plan.
// Manual mode stays authoritative and is not converted into instances.
func ExecutionPlan(doc Document) (Plan, error) {
	if err := Validate(doc); err != nil {
		return Plan{}, err
	}
	if doc.Mode == ModeManual {
		return Plan{}, contractError(CodeManualNotExecutable, "manual mode is authoritative and is not converted to a graph")
	}
	return planGraph(doc.Graph), nil
}

func planGraph(graph *Graph) Plan {
	plan := Plan{TelegrafVersion: SupportedTelegrafVersion}
	for _, instance := range graph.Instances {
		switch instance.Kind {
		case KindInput:
			plan.Inputs = append(plan.Inputs, instance.ID)
		case KindAggregator:
			plan.Aggregators = append(plan.Aggregators, aggregatorPlan(instance))
		case KindOutput:
			plan.Outputs = append(plan.Outputs, instance.ID)
		}
	}
	order := append([]string(nil), graph.ProcessorOrder...)
	if !*graph.SkipProcessorsBeforeAggregators {
		plan.ProcessorsBefore = append([]string(nil), order...)
	}
	// config.go builds a second processor instance list. agent.go starts that
	// list only when at least one aggregator exists and the after flag is false.
	if len(plan.Aggregators) > 0 && !*graph.SkipProcessorsAfterAggregators {
		plan.ProcessorsAfter = append([]string(nil), order...)
	}
	return plan
}

func aggregatorPlan(instance Instance) AggregatorPlan {
	return AggregatorPlan{
		ID:           instance.ID,
		Period:       durationString(instance, "period"),
		Delay:        durationString(instance, "delay"),
		Grace:        durationString(instance, "grace"),
		DropOriginal: *instance.Options["drop_original"].Bool,
	}
}

func durationString(instance Instance, key string) string {
	return *instance.Options[key].Duration
}
