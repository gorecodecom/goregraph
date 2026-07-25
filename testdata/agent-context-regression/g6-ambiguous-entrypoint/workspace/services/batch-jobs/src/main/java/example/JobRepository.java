package example;

import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

interface JobRepository extends JpaRepository<Job, String> {
  List<Job> findAll();

  Job save(Job job);

  void deleteById(String jobId);
}
