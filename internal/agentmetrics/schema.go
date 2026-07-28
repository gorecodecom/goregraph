// Package agentmetrics owns the metric column contracts shared by agent
// transcript analysis, benchmark artifacts, and public documentation.
package agentmetrics

// AnalyzerHeader is the tab-separated transcript analyzer metric schema.
const AnalyzerHeader = "tool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tunique_source_files"

// RegressionSummaryHeader is the tab-separated monotonic regression summary
// schema.
const RegressionSummaryHeader = "case\tquery\tbuild\trun\tattempt\ttokens\ttool_calls\tcontext_calls\trepeated_full_packs\tbroad_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tcontext_millis\tlog"

// ReleaseSummaryHeader is the tab-separated baseline-versus-assisted release
// summary schema.
const ReleaseSummaryHeader = "variant\trun\ttokens\ttool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tunique_source_files\tlog"
