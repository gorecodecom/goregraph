package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAPIContractsExtractExplicitConsumerAuthentication(t *testing.T) {
	source := `export const load = () => axios.get('/orders', { headers: { Authorization: 'Bearer ' + token, 'X-API-Key': apiKey } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "EXTRACTED", "http_call_config", "token")
	assertContractAuth(t, contracts[0].Auth, "api_key", "EXTRACTED", "http_call_config", "apiKey")
}

func TestAPIContractsExtractExplicitBasicConsumerAuthentication(t *testing.T) {
	source := `export const create = () => axios.post('/orders', {}, { auth: { username: accountName, password: accountPassword } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "basic", "EXTRACTED", "http_call_config", "accountName,accountPassword")
}

func TestAPIContractsExtractConsumerSessionAuthentication(t *testing.T) {
	source := `export const load = () => fetch('/orders', { credentials: 'include' });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "session", "EXTRACTED", "http_call_config", "")
}

func TestAPIContractsExtractConsumerOAuthHelperAuthentication(t *testing.T) {
	source := `import { getAccessTokenSilently } from '@auth0/auth0-react';
export const load = () => axios.get('/orders', { headers: { Authorization: 'Bearer ' + getAccessTokenSilently() } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "EXTRACTED", "http_call_config", "getAccessTokenSilently")
	assertContractAuth(t, contracts[0].Auth, "oauth2", "EXTRACTED", "oauth_helper", "getAccessTokenSilently")
	if contracts[0].Auth[0].Line != 2 || contracts[0].Auth[0].File != "src/api/orders.ts" {
		t.Fatalf("auth source location = %s:%d, want src/api/orders.ts:2", contracts[0].Auth[0].File, contracts[0].Auth[0].Line)
	}
}

func TestAPIContractsAssociateConsumerAuthenticationInterceptorPartially(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';
const ordersClient = axios.create({ baseURL: '/api' });
ordersClient.interceptors.request.use((config) => {
  config.headers.Authorization = ` + "`Bearer ${getAccessTokenSilently()}`" + `;
  return config;
});
export const load = () => ordersClient.get('/orders');`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "PARTIAL", "http_client_interceptor", "getAccessTokenSilently")
	assertContractAuth(t, contracts[0].Auth, "oauth2", "PARTIAL", "http_client_interceptor", "getAccessTokenSilently")
}

func TestAPIContractsExtractConsumerAPIKeyQueryAuthentication(t *testing.T) {
	source := `export const load = () => axios.get('/orders', { params: { api_key: serviceKey } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "api_key", "EXTRACTED", "http_call_config", "serviceKey")
}

func TestAPIContractsDoNotInferConsumerAuthenticationOutsideAssociatedCall(t *testing.T) {
	source := `import axios from 'axios';
const adminClient = axios.create();
adminClient.interceptors.request.use((config) => {
  config.headers.Authorization = 'Bearer unrelated-secret';
  return config;
});
const publicClient = axios.create();
const bearerToken = 'also-unrelated';
export const load = () => publicClient.get('/orders');`
	contracts := extractTestAPIContracts(t, source)

	if len(contracts[0].Auth) != 0 {
		t.Fatalf("unassociated auth leaked into contract: %#v", contracts[0].Auth)
	}
	for _, auth := range contracts[0].Auth {
		if auth.Kind == "public" {
			t.Fatalf("request without auth was claimed public: %#v", contracts[0].Auth)
		}
	}
}

func TestAPIContractsConsumerAuthenticationOmitsCredentialValues(t *testing.T) {
	source := `export const load = () => axios.get('/orders', { headers: { Authorization: 'Bearer sample-token-123', 'X-API-Key': 'sample-api-key-456' } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "EXTRACTED", "http_call_config", "")
	assertContractAuth(t, contracts[0].Auth, "api_key", "EXTRACTED", "http_call_config", "")
	data, err := json.Marshal(contracts)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, secret := range []string{"sample-token-123", "sample-api-key-456"} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("serialized contracts contain credential value %q: %s", secret, serialized)
		}
	}
}

func extractTestAPIContracts(t *testing.T, source string) []APIContractRecord {
	t.Helper()
	contracts := extractAPIContracts(
		FileRecord{Path: "src/api/orders.ts", Language: "typescript"},
		strings.Split(source, "\n"),
		nil,
	)
	if len(contracts) != 1 {
		t.Fatalf("contracts=%#v, want exactly one", contracts)
	}
	return contracts
}

func assertContractAuth(t *testing.T, records []AuthRecord, kind, confidence, source, expression string) {
	t.Helper()
	for _, record := range records {
		if record.Kind != kind {
			continue
		}
		if record.Confidence != confidence || record.Source != source || record.Expression != expression {
			t.Fatalf("auth %q = %#v, want confidence=%q source=%q expression=%q", kind, record, confidence, source, expression)
		}
		return
	}
	t.Fatalf("auth kind %q missing from %#v", kind, records)
}
