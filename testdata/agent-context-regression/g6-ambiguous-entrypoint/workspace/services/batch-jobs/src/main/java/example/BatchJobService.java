package example;

import java.util.List;

final class BatchJobService {
  private final JobRepository repository;

  BatchJobService(JobRepository repository) {
    this.repository = repository;
  }

  void remove(String jobId) {
    repository.deleteById(jobId);
  }

  Job create(Job job) {
    return repository.save(job);
  }

  List<Job> list() {
    return repository.findAll();
  }
}
