package example;

import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Import;

@Configuration
@Import({JobClientAuth.class, JobClientRetry.class})
final class JobClientConfig {
  String allJobsPath() {
    return "/internal/jobs";
  }
}
