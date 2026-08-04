package example.inventory;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
interface AllocationReservationRepository extends JpaRepository<AllocationReservation, Long> {
    void deleteByOrderId(String orderId);
}
