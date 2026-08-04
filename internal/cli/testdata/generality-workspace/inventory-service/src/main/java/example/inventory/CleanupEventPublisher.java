package example.inventory;

import org.springframework.stereotype.Component;

@Component
final class CleanupEventPublisher {
    void publishCleanupCompleted(String orderId) {}
}
