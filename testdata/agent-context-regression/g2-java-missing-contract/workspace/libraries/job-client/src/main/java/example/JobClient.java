package example;

import java.util.List;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;

@FeignClient(name = "jobs", configuration = JobClientConfig.class)
interface JobClient {
  @GetMapping("/internal/jobs")
  List<JobPayload> listJobsForRemoval(@RequestParam String catalogId, @RequestParam String itemId);
}

record JobPayload(String id, String catalogId, String itemId) {}
