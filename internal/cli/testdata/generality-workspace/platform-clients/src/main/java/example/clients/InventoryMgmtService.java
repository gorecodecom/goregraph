package example.clients;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.client.support.BasicAuthenticationInterceptor;
import org.springframework.retry.annotation.Retryable;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestClient;

@Service
public final class InventoryMgmtService {
    private final RestClient client;

    public InventoryMgmtService(
            @Value("${inventory-client.base-url}") String baseUrl,
            @Value("${inventory-client.username}") String username,
            @Value("${inventory-client.password}") String password) {
        this.client = RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new BasicAuthenticationInterceptor(username, password))
                .build();
    }

    @Retryable(maxAttempts = 3)
    public void cleanupReservation(String orderId) {
        client.delete().uri("/inventory/reservations/{orderId}", orderId).retrieve().toBodilessEntity();
    }
}
