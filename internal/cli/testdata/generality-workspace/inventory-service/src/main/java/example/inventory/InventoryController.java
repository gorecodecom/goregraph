package example.inventory;

import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/inventory")
final class InventoryController {
    private final ReservationCleanupService cleanupService;

    InventoryController(ReservationCleanupService cleanupService) {
        this.cleanupService = cleanupService;
    }

    @DeleteMapping("/reservations/{orderId}")
    void cleanup(@PathVariable String orderId) {
        cleanupService.cleanup(orderId);
    }
}
