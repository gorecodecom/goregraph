package example;

import org.springframework.retry.annotation.Backoff;
import org.springframework.retry.annotation.Retryable;

final class JobClientRetry {
  @Retryable(retryFor = JobClientException.class, maxAttempts = 3, backoff = @Backoff(delay = 100))
  <T> T execute(JobClientCall<T> call) {
    return call.invoke();
  }
}

interface JobClientCall<T> {
  T invoke();
}

final class JobClientException extends RuntimeException {}
