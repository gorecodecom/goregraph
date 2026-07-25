package example;

import java.util.List;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/internal/jobs")
final class JobController {
  private final JobService service;

  JobController(JobService service) {
    this.service = service;
  }

  @GetMapping
  List<Job> list(
      @RequestParam String catalogId,
      @RequestParam String itemId) {
    return service.list(catalogId, itemId);
  }
}
