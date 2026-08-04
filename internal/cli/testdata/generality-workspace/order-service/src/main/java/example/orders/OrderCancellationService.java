package example.orders;

import example.clients.InventoryMgmtService;
import org.springframework.stereotype.Service;

@Service
final class OrderCancellationService {
    private final InventoryMgmtService inventoryClient;

    OrderCancellationService(InventoryMgmtService inventoryClient) {
        this.inventoryClient = inventoryClient;
    }

    void cancel(String orderId) {
        validateCancellation(orderId);
    }

    private void validateCancellation(String orderId) {
        if (orderId.isBlank()) {
            throw new IllegalArgumentException("orderId");
        }
    }
}
