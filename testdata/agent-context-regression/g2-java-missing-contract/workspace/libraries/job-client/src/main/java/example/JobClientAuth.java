package example;

import org.springframework.context.annotation.Bean;
import org.springframework.http.HttpHeaders;

final class JobClientAuth {
  @Bean
  HttpHeaders basicAuthentication(JobClientCredentials credentials) {
    HttpHeaders headers = new HttpHeaders();
    headers.setBasicAuth(credentials.username(), credentials.password());
    return headers;
  }
}

record JobClientCredentials(String username, String password) {}
