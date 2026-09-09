package example.orders;
public final class OrderController {
    private final OrderService service;
    public OrderController(OrderService service) { this.service = service; }
    @DeleteMapping("/api/orders/{id}")
    public void cancel(String id) { service.cancel(id); }
}
