package example;

import java.util.List;

final class ScheduledJobService {
  private final JobRepository repository;

  ScheduledJobService(JobRepository repository) {
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
