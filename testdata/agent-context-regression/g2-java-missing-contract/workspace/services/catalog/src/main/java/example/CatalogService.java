package example;

final class CatalogService {
  private final CatalogRepository repository;

  CatalogService(CatalogRepository repository) {
    this.repository = repository;
  }

  void remove(String itemId) {
    repository.deleteById(itemId);
  }
}
