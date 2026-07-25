package example;

import java.util.List;
import org.springframework.data.repository.CrudRepository;

interface JobRepository extends CrudRepository<Job, String> {
  List<Job> findByCatalogIdAndItemId(String catalogId, String itemId);
}

record Job(String id, String catalogId, String itemId) {}
