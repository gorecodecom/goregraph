package example.orders;
public final class OrderService {
    public interface OrderRepository { void markCancelled(String id); }
    private final OrderRepository repository;
    public OrderService(OrderRepository repository) { this.repository = repository; }
    public void cancel(String id) { repository.markCancelled(id); }
}
