package example;

import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
final class JobHousekeepingController {
  @DeleteMapping("/internal/jobs/housekeeping")
  void purgeArchivedJobs() {
    runHousekeepingBatch();
  }

  private void runHousekeepingBatch() {}
}
