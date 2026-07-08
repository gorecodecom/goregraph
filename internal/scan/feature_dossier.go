package scan

import (
	"fmt"
	"sort"
	"strings"
)

func buildFeatureDossiers(flows []WorkspaceFeatureFlowRecord, openContracts []WorkspaceContractMatchRecord) []FeatureDossierRecord {
	var records []FeatureDossierRecord
	for _, flow := range flows {
		backendHandler := strings.Trim(strings.TrimSpace(flow.BackendController)+"."+strings.TrimSpace(flow.BackendMethod), ".")
		record := FeatureDossierRecord{
			ID:                stableID("feature-dossier", flow.ID, flow.HTTPMethod, flow.Path),
			Route:             strings.TrimSpace(flow.HTTPMethod + " " + flow.Path),
			FrontendProject:   flow.FrontendProject,
			FrontendRoute:     flow.FrontendRoutePath,
			FrontendComponent: flow.FrontendComponent,
			FrontendCaller:    flow.FrontendCaller,
			BackendProject:    flow.BackendProject,
			BackendHandler:    backendHandler,
			RequestFields:     flow.BackendRequestFields,
			ResponseFields:    flow.BackendResponseFields,
			Auth:              flow.Auth,
			PersistencePath:   flow.PersistencePath,
			Tests:             flow.Tests,
			Risks:             append([]FieldRiskRecord(nil), flow.FieldRisks...),
			Confidence:        flow.Confidence,
			SourceFlowID:      flow.ID,
		}
		if len(record.Tests) == 0 {
			record.Risks = append(record.Risks, FieldRiskRecord{Kind: "missing_tests", Reason: "feature flow has no matched tests", Source: "feature_dossier", Confidence: "MATCHED"})
		}
		records = append(records, record)
	}
	for _, contract := range openContracts {
		if contract.Issue == contractIssueMatched || contract.Confidence == "RESOLVED" {
			continue
		}
		records = append(records, FeatureDossierRecord{
			ID:              stableID("feature-dossier-open-contract", contract.ID),
			Route:           strings.TrimSpace(contract.APIHTTPMethod + " " + contract.APIPath),
			FrontendProject: contract.APIProject,
			FrontendCaller:  contract.APICaller,
			Confidence:      contract.Confidence,
			Risks: []FieldRiskRecord{{
				Kind:       "open_contract",
				Reason:     firstNonEmpty(contract.ResolutionHint, contract.Reason),
				Source:     "workspace_contract_match",
				Confidence: contract.Confidence,
			}},
		})
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Route != records[j].Route {
			return records[i].Route < records[j].Route
		}
		return records[i].ID < records[j].ID
	})
	return records
}

func renderFeatureDossiersReport(records []FeatureDossierRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Feature Dossiers\n\n")
	if len(records) == 0 {
		b.WriteString("No feature dossiers detected.\n")
		return b.String()
	}
	for _, record := range records {
		b.WriteString(fmt.Sprintf("## %s\n\n", record.Route))
		if record.FrontendComponent != "" || record.FrontendRoute != "" || record.FrontendCaller != "" {
			b.WriteString(fmt.Sprintf("- Frontend: `%s` route `%s` caller `%s`\n", emptyAsNone(record.FrontendComponent), emptyAsNone(record.FrontendRoute), emptyAsNone(record.FrontendCaller)))
		}
		if record.BackendHandler != "" {
			b.WriteString(fmt.Sprintf("- Backend: `%s` in `%s`\n", record.BackendHandler, record.BackendProject))
		}
		for _, auth := range record.Auth {
			b.WriteString(fmt.Sprintf("- Auth: `%s` `%s`\n", auth.Kind, auth.Expression))
		}
		if len(record.RequestFields) > 0 {
			b.WriteString("- Request fields:")
			for _, field := range record.RequestFields {
				required := ""
				if field.Required {
					required = " required"
				}
				b.WriteString(fmt.Sprintf(" `%s:%s%s`", field.Name, field.Type, required))
			}
			b.WriteString("\n")
		}
		if len(record.ResponseFields) > 0 {
			b.WriteString("- Response fields:")
			for _, field := range record.ResponseFields {
				b.WriteString(fmt.Sprintf(" `%s:%s`", field.Name, field.Type))
			}
			b.WriteString("\n")
		}
		for _, step := range record.PersistencePath {
			b.WriteString(fmt.Sprintf("- Persistence: `%s.%s` entity `%s` table `%s`\n", step.Repository, step.Method, step.Entity, step.Table))
		}
		if len(record.Tests) > 0 {
			b.WriteString(fmt.Sprintf("- Tests: %d matched\n", len(record.Tests)))
		}
		for _, risk := range record.Risks {
			b.WriteString(fmt.Sprintf("- Risk: `%s` %s\n", risk.Kind, risk.Reason))
		}
		b.WriteString("\n")
	}
	return b.String()
}
