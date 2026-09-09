package example.orders;
@interface PostMapping { String value(); }
final class Order {}
public final class OrderController {
    @PostMapping("/v1/orders")
    public Order create(Order order) { return order; }
}
