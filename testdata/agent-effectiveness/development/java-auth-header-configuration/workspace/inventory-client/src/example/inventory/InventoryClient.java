package example.inventory;
public final class InventoryClient {
    public interface Request { Request uri(String template, String value); Request header(String name, String value); String retrieve(); }
    public interface RestClient { Request get(); }
    private final RestClient restClient;
    private final InventoryProperties properties;
    public InventoryClient(RestClient restClient, InventoryProperties properties) { this.restClient = restClient; this.properties = properties; }
    public String inventory(String sku) {
        return restClient.get().uri(properties.baseUrl() + "/inventory/{sku}", sku).retrieve();
    }
}
