package example;

import org.springframework.data.repository.CrudRepository;

interface CatalogRepository extends CrudRepository<CatalogItem, String> {
  void deleteById(String itemId);
}

record CatalogItem(String id) {}
