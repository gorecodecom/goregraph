package example.inventory;

import org.springframework.stereotype.Service;

@Service
final class ReservationCleanupService {
    private final StockReservationRepository stockReservations;
    private final AllocationReservationRepository allocationReservations;
    private final CleanupEventPublisher cleanupEvents;

    ReservationCleanupService(
            StockReservationRepository stockReservations,
            AllocationReservationRepository allocationReservations,
            CleanupEventPublisher cleanupEvents) {
        this.stockReservations = stockReservations;
        this.allocationReservations = allocationReservations;
        this.cleanupEvents = cleanupEvents;
    }

    void cleanup(String orderId) {
        stockReservations.deleteByOrderId(orderId);
        allocationReservations.deleteByOrderId(orderId);
        stockReservations.saveCleanupState(orderId);
        cleanupEvents.publishCleanupCompleted(orderId);
    }
}
