package example;

import java.util.List;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
final class ScheduledJobController {
  private final ScheduledJobService service;

  ScheduledJobController(ScheduledJobService service) {
    this.service = service;
  }

  @DeleteMapping("/jobs/{jobId}")
  void remove(@PathVariable String jobId) {
    service.remove(jobId);
  }

  @PostMapping("/jobs")
  Job create(@RequestBody Job job) {
    return service.create(job);
  }

  @GetMapping("/jobs")
  List<Job> list() {
    return service.list();
  }
}
