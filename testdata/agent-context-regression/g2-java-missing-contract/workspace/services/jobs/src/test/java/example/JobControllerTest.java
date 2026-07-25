package example;

import java.util.List;
import org.junit.jupiter.api.Test;

final class JobControllerTest {
  private final RecordingJobRepository repository = new RecordingJobRepository();
  private final JobController controller =
      new JobController(new JobService(repository));

  @Test
  void listUsesTheCatalogAndItemFinder() {
    List<Job> jobs = controller.list("catalog-2", "item-7");

    assert jobs.size() == 1;
  }

  private static final class RecordingJobRepository implements JobRepository {
    @Override
    public List<Job> findByCatalogIdAndItemId(String catalogId, String itemId) {
      return List.of(new Job("job-1", catalogId, itemId));
    }
  }
}
