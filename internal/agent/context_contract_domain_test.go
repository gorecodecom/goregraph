package agent

import (
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextContractDomainScoreIgnoresMutationAndTransportTerms(t *testing.T) {
	fact := scan.AgentContextFactRecord{
		Kind: "api_contract", Name: "DELETE /", Qualified: "UserClient.deleteTemplateUser",
		HTTPMethod: "DELETE", Path: "/", Search: "DELETE REST client user template",
	}
	query := "When a catalog entry is deleted and related jobs remain, determine the public REST " +
		"endpoint and required internal API contract."

	if score := contextContractDomainScore(fact, query); score != 0 {
		t.Fatalf("unrelated contract domain score = %d, want 0", score)
	}
}
