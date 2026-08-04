package example.inventory;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
interface StockReservationRepository extends JpaRepository<StockReservation, Long> {
    void deleteByOrderId(String orderId);

    void saveCleanupState(String orderId);
}
