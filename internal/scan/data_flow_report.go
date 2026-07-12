package scan

import (
	"fmt"
	"strings"
)

func RenderDataFlowsReport(records []DataFlowRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Data Flows\n\nKnown mappings are evidence-backed. Missing transformations are shown as gaps rather than inferred facts. Review `coverage.md` before treating an empty result as absence.\n\n")
	if len(records) == 0 {
		b.WriteString("- none generated\n")
		return b.String()
	}
	for _, record := range records {
		b.WriteString(fmt.Sprintf("## %s\n\n", record.Route))
		for _, node := range record.Nodes {
			b.WriteString(fmt.Sprintf("- `%s` (%s, %s)\n", node.Label, node.Role, node.Confidence))
		}
		for _, gap := range record.Gaps {
			b.WriteString(fmt.Sprintf("- Gap: %s (`%s`)\n", gap.Reason, gap.RequiredCapability))
		}
	}
	return b.String()
}
