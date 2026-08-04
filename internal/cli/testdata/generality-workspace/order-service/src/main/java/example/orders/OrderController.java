package example.orders;

import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/orders")
final class OrderController {
    private final OrderCancellationService cancellationService;

    OrderController(OrderCancellationService cancellationService) {
        this.cancellationService = cancellationService;
    }

    @DeleteMapping("/{orderId}")
    @PreAuthorize("hasAuthority('ORDER_CANCEL')")
    void cancel(@PathVariable String orderId) {
        cancellationService.cancel(orderId);
    }
}
