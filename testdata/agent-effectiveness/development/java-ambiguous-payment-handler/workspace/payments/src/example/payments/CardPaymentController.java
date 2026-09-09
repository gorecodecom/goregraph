package example.payments;
public final class CardPaymentController {
    @PostMapping("/cards/{id}/capture")
    public String capture(String id) { return "card:" + id; }
}
