package scan

import "testing"

func TestBuildServiceDependenciesFromImportedClientBoundary(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/orders/OrderCancellationService.java", Language: "java"}, `package example.orders;

import com.acme.platform.inventory.InventoryMgmtService;

final class OrderCancellationService {
  private final InventoryMgmtService inventory;

  OrderCancellationService(InventoryMgmtService inventory) {
    this.inventory = inventory;
  }
}
`)

	records := buildServiceDependencies(
		WorkspaceProjectRecord{Path: "services/order-service"},
		[]JavaSourceRecord{source},
	)

	if len(records) != 1 {
		t.Fatalf("dependencies = %#v, want one imported client boundary", records)
	}
	record := records[0]
	if record.FromProject != "services/order-service" {
		t.Fatalf("from project = %q", record.FromProject)
	}
	if record.ToProject != "" || record.ToService != "" {
		t.Fatalf("project scan invented workspace ownership: %#v", record)
	}
	if record.ResolutionKey != "inventory" {
		t.Fatalf("resolution key = %q, want inventory", record.ResolutionKey)
	}
	if record.Kind != "java_client_import" || record.Confidence != "EXTRACTED" || record.Evidence == "" {
		t.Fatalf("dependency missing neutral evidence: %#v", record)
	}
}

func TestBuildServiceDependenciesRequiresImportedBoundaryUsage(t *testing.T) {
	source := extractJavaSource(FileRecord{Path: "src/main/java/example/orders/OrderService.java", Language: "java"}, `package example.orders;

import com.acme.platform.inventory.InventoryClient;

final class OrderService {}
`)

	records := buildServiceDependencies(WorkspaceProjectRecord{Path: "services/order-service"}, []JavaSourceRecord{source})
	if len(records) != 0 {
		t.Fatalf("unused import became a dependency: %#v", records)
	}
}

func TestBuildServiceDependenciesIgnoresLocalFrameworkAndNonBoundaryTypes(t *testing.T) {
	consumer := extractJavaSource(FileRecord{Path: "src/main/java/example/orders/OrderController.java", Language: "java"}, `package example.orders;

import example.orders.internal.OrderService;
import org.springframework.security.core.userdetails.UserDetailsService;
import com.acme.platform.inventory.InventoryPort;

final class OrderController {
  private final OrderService orderService;
  private final UserDetailsService userDetailsService;
  private final InventoryPort inventoryPort;
}
`)
	local := extractJavaSource(FileRecord{Path: "src/main/java/example/orders/internal/OrderService.java", Language: "java"}, `package example.orders.internal;

final class OrderService {}
`)

	records := buildServiceDependencies(
		WorkspaceProjectRecord{Path: "services/order-service"},
		[]JavaSourceRecord{consumer, local},
	)

	if len(records) != 0 {
		t.Fatalf("local/framework/non-boundary types became dependencies: %#v", records)
	}
}
