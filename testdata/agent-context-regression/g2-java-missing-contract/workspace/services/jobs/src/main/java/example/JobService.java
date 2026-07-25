package example;

import java.util.List;

final class JobService {
  private final JobRepository repository;

  JobService(JobRepository repository) {
    this.repository = repository;
  }

  List<Job> list(String catalogId, String itemId) {
    return repository.findByCatalogIdAndItemId(catalogId, itemId);
  }
}
