package scan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAPIContractsExtractExplicitConsumerAuthentication(t *testing.T) {
	source := `import axios from 'axios';
export const load = () => axios.get('/orders', { headers: { Authorization: 'Bearer ' + token, 'X-API-Key': apiKey } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "EXTRACTED", "http_call_config", "token")
	assertContractAuth(t, contracts[0].Auth, "api_key", "EXTRACTED", "http_call_config", "apiKey")
}

func TestAPIContractsExtractExplicitBasicConsumerAuthentication(t *testing.T) {
	source := `import axios from 'axios';
export const create = () => axios.post('/orders', {}, { auth: { username: accountName, password: accountPassword } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "basic", "EXTRACTED", "http_call_config", "accountName,accountPassword")
}

func TestAPIContractsExtractConsumerSessionAuthentication(t *testing.T) {
	source := `export const load = () => fetch('/orders', { credentials: 'include' });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "session", "EXTRACTED", "http_call_config", "")
}

func TestAPIContractsExtractConsumerOAuthHelperAuthentication(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';
export const load = () => axios.get('/orders', { headers: { Authorization: 'Bearer ' + getAccessTokenSilently() } });`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "EXTRACTED", "http_call_config", "getAccessTokenSilently")
	assertContractAuth(t, contracts[0].Auth, "oauth2", "EXTRACTED", "oauth_helper", "getAccessTokenSilently")
	if contracts[0].Auth[0].Line != 3 || contracts[0].Auth[0].File != "src/api/orders.ts" {
		t.Fatalf("auth source location = %s:%d, want src/api/orders.ts:3", contracts[0].Auth[0].File, contracts[0].Auth[0].Line)
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
	source := `import axios from 'axios';
export const load = () => axios.get('/orders', { params: { api_key: serviceKey } });`
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
	source := `import axios from 'axios';
export const load = () => axios.get('/orders', { headers: { Authorization: 'Bearer sample-token-123', 'X-API-Key': 'sample-api-key-456' } });`
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

func TestAPIContractsInspectOnlyTopLevelAxiosConfigArgument(t *testing.T) {
	source := `import axios from 'axios';
export const create = () => axios.post(
  '/orders',
  { headers: { Authorization: 'Bearer ' + bodyToken } },
  { headers: { 'X-API-Key': requestKey } },
);`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "api_key", "EXTRACTED", "http_call_config", "requestKey")
	assertNoContractAuthKind(t, contracts[0].Auth, "bearer")
}

func TestAPIContractsInspectOnlyTopLevelFetchConfigArgument(t *testing.T) {
	source := `export const create = () => fetch(
  '/orders',
  {
    method: 'POST',
    body: JSON.stringify({ headers: { Authorization: 'Bearer ' + bodyToken } }),
  },
);`
	contracts := extractTestAPIContracts(t, source)

	if len(contracts[0].Auth) != 0 {
		t.Fatalf("fetch body was treated as request config auth: %#v", contracts[0].Auth)
	}
}

func TestAPIContractsRequireUnshadowedAxiosImport(t *testing.T) {
	source := `import axios from 'axios';
function loadShadowed(axios: Client) {
  return axios.get('/shadowed', { headers: { Authorization: 'Bearer ' + shadowToken } });
}
const loadArrowShadowed = (axios: Client) => axios.get('/arrow-shadowed');
export const load = () => axios.get('/orders');`
	contracts := extractTestAPIContractsAll(source)

	if len(contracts) != 1 || contracts[0].Path != "/orders" {
		t.Fatalf("shadowed axios produced contracts: %#v", contracts)
	}
}

func TestAPIContractsKeepSameNameClientInterceptorsInLexicalScope(t *testing.T) {
	source := `import axios from 'axios';
const client = axios.create();
client.interceptors.request.use((config) => {
  config.headers.Authorization = 'Bearer ' + rootToken;
  return config;
});
export const loadRoot = () => client.get('/root');

export function loadNested() {
  const client = axios.create();
  client.interceptors.request.use((config) => {
    config.headers['X-API-Key'] = nestedKey;
    return config;
  });
  return client.get('/nested');
}`
	contracts := extractTestAPIContractsAll(source)
	if len(contracts) != 2 {
		t.Fatalf("contracts=%#v, want two scoped client calls", contracts)
	}

	root := contractByPath(t, contracts, "/root")
	assertContractAuth(t, root.Auth, "bearer", "PARTIAL", "http_client_interceptor", "rootToken")
	assertNoContractAuthKind(t, root.Auth, "api_key")

	nested := contractByPath(t, contracts, "/nested")
	assertContractAuth(t, nested.Auth, "api_key", "PARTIAL", "http_client_interceptor", "nestedKey")
	assertNoContractAuthKind(t, nested.Auth, "bearer")
}

func TestAPIContractsMaskCommentsBeforeCallAndAuthScanning(t *testing.T) {
	source := `// axios.get('/line-comment', { headers: { Authorization: 'Bearer line-secret' } });
/*
axios.get('/block-comment', { headers: { 'X-API-Key': 'block-secret' } });
*/
export const load = () => fetch(
  '/orders',
  {
    // headers: { Authorization: 'Bearer commented-secret' },
    transformResponse: [(value) => ({ value /* ) } fake delimiters */ })],
  },
);`
	contracts := extractTestAPIContractsAll(source)

	if len(contracts) != 1 || contracts[0].Path != "/orders" {
		t.Fatalf("commented calls or delimiters affected extraction: %#v", contracts)
	}
	if len(contracts[0].Auth) != 0 {
		t.Fatalf("commented auth was extracted: %#v", contracts[0].Auth)
	}
}

func TestAPIContractsDiscoverMultilineAxiosAndFetchCalls(t *testing.T) {
	source := `import axios from 'axios';
export const loadAxios = () => axios
  .get(
    '/axios-orders',
    {
      transformResponse: [(value) => ({ value })],
      headers: { Authorization: 'Bearer ' + axiosToken },
    },
  );

export const loadFetch = () => fetch(
  '/fetch-orders',
  {
    method: 'POST',
    transform: () => callback(() => ({ nested: true })),
    credentials: 'include',
  },
);`
	contracts := extractTestAPIContractsAll(source)
	if len(contracts) != 2 {
		t.Fatalf("multiline contracts=%#v, want two", contracts)
	}

	axiosContract := contractByPath(t, contracts, "/axios-orders")
	if axiosContract.HTTPMethod != "GET" {
		t.Fatalf("axios method=%q, want GET", axiosContract.HTTPMethod)
	}
	assertContractAuth(t, axiosContract.Auth, "bearer", "EXTRACTED", "http_call_config", "axiosToken")

	fetchContract := contractByPath(t, contracts, "/fetch-orders")
	if fetchContract.HTTPMethod != "POST" {
		t.Fatalf("fetch method=%q, want POST", fetchContract.HTTPMethod)
	}
	assertContractAuth(t, fetchContract.Auth, "session", "EXTRACTED", "http_call_config", "")
}

func TestAPIContractsSortOAuthHelpersAcrossImportPermutations(t *testing.T) {
	imports := []string{
		"getAccessToken as delta",
		"getAccessTokenSilently as alpha",
		"getTokenSilently as charlie",
		"acquireTokenSilent as bravo",
	}
	want := []string{"alpha", "bravo", "charlie", "delta"}
	for permutation := 0; permutation < len(imports); permutation++ {
		rotated := append(append([]string(nil), imports[permutation:]...), imports[:permutation]...)
		source := `import axios from 'axios';
import { ` + strings.Join(rotated, ", ") + ` } from '@example/oauth-client';
export const load = () => axios.get('/orders', { headers: {
  Authorization: 'Bearer ' + delta(),
  authorization: 'Bearer ' + alpha(),
  'Authorization': 'Bearer ' + charlie(),
  'authorization': 'Bearer ' + bravo(),
} });`
		for attempt := 0; attempt < 16; attempt++ {
			contracts := extractTestAPIContracts(t, source)
			got := authExpressions(contracts[0].Auth, "oauth2")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("permutation %d attempt %d oauth order=%#v, want %#v", permutation, attempt, got, want)
			}
		}
	}
}

func TestAPIContractsCredentialTemplateLiteralsNeverSerializeValues(t *testing.T) {
	source := "import axios from 'axios';\n" +
		"export const load = () => axios.get('/orders', {\n" +
		"  headers: { Authorization: `Bearer ${'s3cr3tToken'}`, 'X-API-Key': `${\"s3cr3tKey\"}` },\n" +
		"  auth: { username: `${'s3cr3tUser'}`, password: `${\"s3cr3tPassword\"}` },\n" +
		"});"
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "EXTRACTED", "http_call_config", "")
	assertContractAuth(t, contracts[0].Auth, "api_key", "EXTRACTED", "http_call_config", "")
	assertContractAuth(t, contracts[0].Auth, "basic", "EXTRACTED", "http_call_config", "")
	assertSerializedContractsOmit(t, contracts, "s3cr3tToken", "s3cr3tKey", "s3cr3tUser", "s3cr3tPassword")
}

func TestAPIContractsOAuthHelpersRequireUnshadowedImportedBinding(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';

export const load = () => axios.get('/real', {
  headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
});

export function loadParameter(getAccessTokenSilently: TokenFactory) {
  return axios.get('/parameter', { headers: { Authorization: 'Bearer ' + getAccessTokenSilently() } });
}

export function loadLocal() {
  const getAccessTokenSilently = () => 'local-value';
  return axios.get('/local', { headers: { Authorization: 'Bearer ' + getAccessTokenSilently() } });
}`
	contracts := extractTestAPIContractsAll(source)
	if len(contracts) != 3 {
		t.Fatalf("contracts=%#v, want three calls", contracts)
	}
	assertContractAuth(t, contractByPath(t, contracts, "/real").Auth, "oauth2", "EXTRACTED", "oauth_helper", "getAccessTokenSilently")
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/parameter").Auth, "oauth2")
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/local").Auth, "oauth2")
}

func TestAPIContractsRequireCompleteUnshadowedAxiosReceiver(t *testing.T) {
	source := `import axios from 'axios';
namespace.axios.get('/namespace-suffix');

function destructured({ axios }: Clients) {
  return axios.get('/destructured');
}

try {
  throw new Error('client');
} catch (axios) {
  axios.get('/catch');
}

class Loader {
  load(axios: Client) {
    return axios.get('/method');
  }
}

function local() {
  const axios = fakeClient;
  return axios.get('/local');
}

export const load = () => axios.get('/orders');`
	contracts := extractTestAPIContractsAll(source)
	if len(contracts) != 1 || contracts[0].Path != "/orders" {
		t.Fatalf("incomplete or shadowed axios receiver produced contracts: %#v", contracts)
	}
}

func TestAPIContractsIgnoreNestedRequestConfigHeaderAssignments(t *testing.T) {
	source := `import axios from 'axios';
export const save = () => axios.post('/orders', {}, {
  transformRequest: [(payload) => {
    payload.headers.Authorization = 'Bearer ' + payloadToken;
    payload.headers['X-API-Key'] = payloadKey;
    return payload;
  }],
});`
	contracts := extractTestAPIContracts(t, source)
	if len(contracts[0].Auth) != 0 {
		t.Fatalf("nested transform payload assignment produced auth: %#v", contracts[0].Auth)
	}
}

func TestAPIContractsInterceptorAssignmentsRequireResolvedConfigParameter(t *testing.T) {
	source := `import axios from 'axios';
const client = axios.create();
client.interceptors.request.use((requestConfig) => {
  const payload = { headers: {} };
  payload.headers.Authorization = 'Bearer ' + payloadToken;
  requestConfig.headers.Authorization = 'Bearer ' + interceptorToken;
  requestConfig.headers['X-API-Key'] = interceptorKey;
  return requestConfig;
});
export const load = () => client.get('/orders');`
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "PARTIAL", "http_client_interceptor", "interceptorToken")
	assertContractAuth(t, contracts[0].Auth, "api_key", "PARTIAL", "http_client_interceptor", "interceptorKey")
	assertNoContractAuthExpression(t, contracts[0].Auth, "payloadToken")
}

func TestAPIContractsLexerHandlesRegexLiteralsAndTemplateInterpolation(t *testing.T) {
	source := "const syntax = /[\\\"'`{}()]/g;\n" +
		"export const interpolated = `${enabled ? fetch('/interpolated', { credentials: 'include' }) : ''}`;\n" +
		"export const load = () => fetch('/orders', { credentials: 'include' });"
	contracts := extractTestAPIContractsAll(source)
	if len(contracts) != 2 {
		t.Fatalf("regex or template interpolation corrupted call discovery: %#v", contracts)
	}
	assertContractAuth(t, contractByPath(t, contracts, "/interpolated").Auth, "session", "EXTRACTED", "http_call_config", "")
	assertContractAuth(t, contractByPath(t, contracts, "/orders").Auth, "session", "EXTRACTED", "http_call_config", "")
}

func TestAPIContractsCredentialInterpolationPrivacyHandlesNestedSyntax(t *testing.T) {
	source := "import axios from 'axios';\n" +
		"export const load = () => axios.get('/orders', {\n" +
		"  headers: {\n" +
		"    Authorization: `Bearer ${\"alphaBearer}escaped\\\"Secret\"}`,\n" +
		"    'X-API-Key': `${'betaApi}escaped\\'Secret'}`,\n" +
		"  },\n" +
		"  auth: {\n" +
		"    username: `${resolveUser({ nested: { value: \"gammaUser}escaped\\\"Secret\" } })}`,\n" +
		"    password: `${\"deltaPassword}escaped\\\"Secret\"}`,\n" +
		"  },\n" +
		"});"
	contracts := extractTestAPIContracts(t, source)

	assertContractAuth(t, contracts[0].Auth, "bearer", "EXTRACTED", "http_call_config", "")
	assertContractAuth(t, contracts[0].Auth, "api_key", "EXTRACTED", "http_call_config", "")
	assertContractAuth(t, contracts[0].Auth, "basic", "EXTRACTED", "http_call_config", "resolveUser")
	assertSerializedContractsOmit(t, contracts,
		"alphaBearer", "betaApi", "gammaUser", "deltaPassword", "escaped", "Secret",
	)
}

func TestAPIContractsDeclarationNamesShadowImportedBindings(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';

export function axiosFunctionShadow() {
  function axios() {}
  return axios.get('/axios-function-shadow');
}

export function axiosClassShadow() {
  class axios {}
  return axios.get('/axios-class-shadow');
}

export function oauthFunctionShadow() {
  function getAccessTokenSilently() { return 'local'; }
  return axios.get('/oauth-function-shadow', {
    headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
  });
}

export function oauthClassShadow() {
  class getAccessTokenSilently {}
  return axios.get('/oauth-class-shadow', {
    headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
  });
}

export const real = () => axios.get('/real', {
  headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
});`
	contracts := extractTestAPIContractsAll(source)

	if len(contracts) != 3 {
		t.Fatalf("declaration shadows produced contracts: %#v", contracts)
	}
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/oauth-function-shadow").Auth, "oauth2")
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/oauth-class-shadow").Auth, "oauth2")
	assertContractAuth(t, contractByPath(t, contracts, "/real").Auth, "oauth2", "EXTRACTED", "oauth_helper", "getAccessTokenSilently")
}

func TestAPIContractsNestedArrowParametersShadowImportedBindings(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';

export const axiosShadow = (
  axios: Client<() => { get(): Result }> = makeClient(() => ({ headers: {} })),
) => axios.get('/arrow-axios-shadow');

export const oauthShadow = (
  getAccessTokenSilently: TokenFactory<() => Promise<Token>> = factory(() => ({ token: 'local' })),
) => axios.get('/arrow-oauth-shadow', {
  headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
});

export const real = () => axios.get('/real');`
	contracts := extractTestAPIContractsAll(source)

	if len(contracts) != 2 {
		t.Fatalf("nested arrow parameter shadows produced contracts: %#v", contracts)
	}
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/arrow-oauth-shadow").Auth, "oauth2")
	if contractByPath(t, contracts, "/real").Path != "/real" {
		t.Fatal("unshadowed axios contract missing")
	}
}

func TestAPIContractsSemanticAuthMatchingIgnoresLiteralAndCommentText(t *testing.T) {
	source := "import axios from 'axios';\n" +
		"import { getAccessTokenSilently } from '@auth0/auth0-react';\n" +
		"const client = axios.create();\n" +
		"client.interceptors.request.use((config) => {\n" +
		"  const docs = \"config.headers.Authorization = 'Bearer ' + getAccessTokenSilently()\";\n" +
		"  const example = `config.headers['X-API-Key'] = documentedKey`;\n" +
		"  // config.headers.Authorization = 'Bearer ' + commentedToken;\n" +
		"  return config;\n" +
		"});\n" +
		"export const interceptor = () => client.get('/interceptor');\n" +
		"export const literalHelper = () => axios.get('/literal-helper', {\n" +
		"  headers: { Authorization: 'Bearer getAccessTokenSilently()' },\n" +
		"});"
	contracts := extractTestAPIContractsAll(source)

	if auth := contractByPath(t, contracts, "/interceptor").Auth; len(auth) != 0 {
		t.Fatalf("literal or comment interceptor text produced auth: %#v", auth)
	}
	literalHelper := contractByPath(t, contracts, "/literal-helper")
	assertContractAuth(t, literalHelper.Auth, "bearer", "EXTRACTED", "http_call_config", "")
	assertNoContractAuthKind(t, literalHelper.Auth, "oauth2")
}

func TestAPIContractsInterceptorRequiresCompleteReceiver(t *testing.T) {
	source := `import axios from 'axios';
const client = axios.create();
namespace.client.interceptors.request.use((config) => {
  config.headers.Authorization = 'Bearer ' + namespaceToken;
  return config;
});
export const load = () => client.get('/orders');`
	contracts := extractTestAPIContracts(t, source)

	if len(contracts[0].Auth) != 0 {
		t.Fatalf("suffix interceptor attached to root client: %#v", contracts[0].Auth)
	}
}

func TestAPIContractsOAuthHelperRequiresCompleteBareCall(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';

export const memberHelpers = () => axios.get('/member-helpers', { headers: {
  Authorization: 'Bearer ' + provider.getAccessTokenSilently(),
  authorization: 'Bearer ' + provider?.getAccessTokenSilently(),
  'Authorization': 'Bearer ' + provider.#getAccessTokenSilently(),
  'authorization': 'Bearer ' + prefixgetAccessTokenSilently(),
  AUTHORIZATION: 'Bearer ' + $getAccessTokenSilently(),
} });

export const bareHelper = () => axios.get('/bare-helper', {
  headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
});`
	contracts := extractTestAPIContractsAll(source)

	assertNoContractAuthKind(t, contractByPath(t, contracts, "/member-helpers").Auth, "oauth2")
	assertContractAuth(t, contractByPath(t, contracts, "/bare-helper").Auth, "oauth2", "EXTRACTED", "oauth_helper", "getAccessTokenSilently")
}

func TestAPIContractsAnnotatedConciseArrowParametersShadowImports(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';

export const axiosShadow = (
  axios: Client,
): Promise<Result<Map<string, Token>>> => axios.get('/annotated-axios-shadow');

export const oauthShadow = (
  getAccessTokenSilently: TokenFactory,
): Promise<Result<() => Token>> => axios.get('/annotated-oauth-shadow', {
  headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
});

export const real = () => axios.get('/real');`
	contracts := extractTestAPIContractsAll(source)

	if len(contracts) != 2 {
		t.Fatalf("annotated arrow parameter shadows produced contracts: %#v", contracts)
	}
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/annotated-oauth-shadow").Auth, "oauth2")
	if contractByPath(t, contracts, "/real").Path != "/real" {
		t.Fatal("unshadowed axios contract missing")
	}
}

func TestAPIContractsNormalConciseArrowDoesNotReuseEarlierReturnAnnotation(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';

export const annotated = (value: Input): Result<Map<string, Token>> => transform(value);
export const axiosShadow = (axios: Client) => axios.get('/normal-axios-shadow');
export const oauthShadow = (getAccessTokenSilently: TokenFactory) => axios.get('/normal-oauth-shadow', {
  headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
});
export const real = () => axios.get('/real');`
	contracts := extractTestAPIContractsAll(source)

	if len(contracts) != 2 {
		t.Fatalf("normal arrow reused earlier return annotation: %#v", contracts)
	}
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/normal-oauth-shadow").Auth, "oauth2")
	if contractByPath(t, contracts, "/real").Path != "/real" {
		t.Fatal("unshadowed axios contract missing")
	}
}

func TestAPIContractsSameStatementArrowsDoNotReuseEarlierReturnAnnotation(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently } from '@auth0/auth0-react';

const annotated = (value: Input): Result<Map<string, Token>> => transform(value), axiosShadow = (axios: Client) => axios.get('/declarator-axios-shadow');
const arrows = [(value: Input): Result<Map<string, Token>> => transform(value), (axios: Client) => axios.get('/array-axios-shadow')];
const arrowObject = { annotated: (value: Input): Result<Map<string, Token>> => transform(value), oauth: (getAccessTokenSilently: TokenFactory) => axios.get('/object-oauth-shadow', {
  headers: { Authorization: 'Bearer ' + getAccessTokenSilently() },
}) };
export const real = () => axios.get('/real');`
	contracts := extractTestAPIContractsAll(source)

	if len(contracts) != 2 {
		t.Fatalf("same-statement arrow reused earlier return annotation: %#v", contracts)
	}
	assertNoContractAuthKind(t, contractByPath(t, contracts, "/object-oauth-shadow").Auth, "oauth2")
	if contractByPath(t, contracts, "/real").Path != "/real" {
		t.Fatal("unshadowed axios contract missing")
	}
}

func TestAPIContractsPrecomputeBoundedFileAnalysisIndexes(t *testing.T) {
	source := `import axios from 'axios';
import { getAccessTokenSilently as zeta, acquireTokenSilent as alpha } from '@example/oauth-client';
const client = axios.create();
client.interceptors.request.use((config) => {
  config.headers.Authorization = 'Bearer ' + token;
  return config;
});
export const load = () => client.get('/orders');`
	analysis := buildJSAPIAnalysis(source)

	if len(analysis.lineStarts) != strings.Count(source, "\n")+1 {
		t.Fatalf("line-start index length=%d", len(analysis.lineStarts))
	}
	if len(analysis.model.scopeAtOffsets) != len(source)+1 {
		t.Fatalf("scope-offset index length=%d, want %d", len(analysis.model.scopeAtOffsets), len(source)+1)
	}
	if len(analysis.model.bindingsByScopeName) == 0 {
		t.Fatal("scope/name binding index was not precomputed")
	}
	if want := []string{"alpha", "zeta"}; !reflect.DeepEqual(analysis.model.oauthHelpers, want) {
		t.Fatalf("precomputed OAuth helpers=%#v, want %#v", analysis.model.oauthHelpers, want)
	}
	callStart := strings.Index(source, "client.get")
	binding, ok := analysis.model.resolveHTTPClient("client", callStart)
	if !ok {
		t.Fatal("precomputed client binding missing")
	}
	if evidence := analysis.interceptorAuthByBinding[binding]; len(evidence) != 1 {
		t.Fatalf("interceptor evidence=%#v, want one precomputed entry", evidence)
	}
}

func extractTestAPIContracts(t *testing.T, source string) []APIContractRecord {
	t.Helper()
	contracts := extractTestAPIContractsAll(source)
	if len(contracts) != 1 {
		t.Fatalf("contracts=%#v, want exactly one", contracts)
	}
	return contracts
}

func extractTestAPIContractsAll(source string) []APIContractRecord {
	return extractAPIContracts(
		FileRecord{Path: "src/api/orders.ts", Language: "typescript"},
		strings.Split(source, "\n"),
		nil,
	)
}

func contractByPath(t *testing.T, contracts []APIContractRecord, path string) APIContractRecord {
	t.Helper()
	for _, contract := range contracts {
		if contract.Path == path {
			return contract
		}
	}
	t.Fatalf("contract path %q missing from %#v", path, contracts)
	return APIContractRecord{}
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

func assertNoContractAuthKind(t *testing.T, records []AuthRecord, kind string) {
	t.Helper()
	for _, record := range records {
		if record.Kind == kind {
			t.Fatalf("unexpected auth kind %q in %#v", kind, records)
		}
	}
}

func authExpressions(records []AuthRecord, kind string) []string {
	var expressions []string
	for _, record := range records {
		if record.Kind == kind {
			expressions = append(expressions, record.Expression)
		}
	}
	return expressions
}

func assertSerializedContractsOmit(t *testing.T, contracts []APIContractRecord, values ...string) {
	t.Helper()
	data, err := json.Marshal(contracts)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, value := range values {
		if strings.Contains(serialized, value) {
			t.Fatalf("serialized contracts contain credential value %q: %s", value, serialized)
		}
	}
}

func assertNoContractAuthExpression(t *testing.T, records []AuthRecord, expression string) {
	t.Helper()
	for _, record := range records {
		if record.Expression == expression {
			t.Fatalf("unexpected auth expression %q in %#v", expression, records)
		}
	}
}
