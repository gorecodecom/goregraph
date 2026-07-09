package scan

import "testing"

func TestBuildServiceDependenciesFromJavaCommonClientImports(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/example/CadasterController.java", Language: "java"}, `package com.example;

import com.weka.common.licenseservice.LicenseMgmtService;
import com.weka.common.userservice.UserMgmtService;

class CadasterController {
  private final LicenseMgmtService licenseMgmtService;
  private final UserMgmtService userMgmtService;
}
`)

	records := buildServiceDependencies(
		WorkspaceProjectRecord{Path: "microservices/ms-cadaster"},
		[]JavaSourceRecord{source},
	)

	requireServiceDependency(t, records, "microservices/ms-cadaster", "ms-licenseservice")
	requireServiceDependency(t, records, "microservices/ms-cadaster", "ms-userservice")
}

func TestBuildServiceDependenciesIgnoresLocalAndFrameworkServices(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/com/example/CadasterController.java", Language: "java"}, `package com.example;

import com.weka.vd.api.cadaster.service.CadasterService;
import org.springframework.security.core.userdetails.UserDetailsService;

class CadasterController {
  private final CadasterService cadasterService;
  private final UserDetailsService userDetailsService;
}
`)

	records := buildServiceDependencies(
		WorkspaceProjectRecord{Path: "microservices/ms-cadaster"},
		[]JavaSourceRecord{source},
	)

	if len(records) != 0 {
		t.Fatalf("local/framework services should not become workspace dependencies: %#v", records)
	}
}

func requireServiceDependency(t *testing.T, records []WorkspaceServiceDependencyRecord, fromProject, toService string) {
	t.Helper()
	for _, record := range records {
		if record.FromProject == fromProject && record.ToService == toService {
			if record.Confidence != "EXTRACTED" || record.Evidence == "" {
				t.Fatalf("dependency missing confidence/evidence: %#v", record)
			}
			return
		}
	}
	t.Fatalf("missing service dependency %s -> %s in %#v", fromProject, toService, records)
}
