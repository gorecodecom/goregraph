package example.payments;
public final class TransferPaymentController {
    @PostMapping("/transfers/{id}/capture")
    public String capture(String id) { return "transfer:" + id; }
}
