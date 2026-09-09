package example.shipping;
@interface PatchMapping { String value(); }
public final class ShipmentController {
    @PatchMapping("/shipments/{id}/dispatch")
    public String dispatch(String id) { return "dispatched:" + id; }
}
