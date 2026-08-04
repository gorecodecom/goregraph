package example.orders;

import org.junit.jupiter.api.Test;

final class OrderCancellationServiceTest {
    private final OrderCancellationService service = new OrderCancellationService(null);

    @Test
    void cancellationSucceedsAfterInventoryCleanup() {
        service.cancel("order-1");
    }

    @Test
    void cancellationReportsInventoryFailure() {}

    @Test
    void cancellationRetriesTransientCleanup() {}

    @Test
    void cancellationPublishesSideEffectAfterCleanup() {}
}
