package example;

import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/catalog")
final class CatalogController {
  private final CatalogService service;

  CatalogController(CatalogService service) {
    this.service = service;
  }

  @DeleteMapping("/{itemId}")
  void remove(@PathVariable String itemId) {
    service.remove(itemId);
  }
}
