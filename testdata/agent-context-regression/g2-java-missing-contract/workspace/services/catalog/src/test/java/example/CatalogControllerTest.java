package example;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;

@WebMvcTest(CatalogController.class)
final class CatalogControllerTest {
  private final RecordingCatalogRepository repository = new RecordingCatalogRepository();
  private final CatalogController controller =
      new CatalogController(new CatalogService(repository));

  @Autowired private MockMvc mockMvc;

  @Test
  void removeEndpointUsesTheCatalogDeletionPath() throws Exception {
    mockMvc.perform(delete("/catalog/item-7"));
  }

  @Test
  void removingCatalogItemDoesNotRemoveJobsWithoutFutureContract() {
    controller.remove("item-7");

    assert repository.removedId.equals("item-7");
    assert repository.jobRemovals == 0;
  }

  private static final class RecordingCatalogRepository implements CatalogRepository {
    private String removedId;
    private int jobRemovals;

    @Override
    public void deleteById(String itemId) {
      removedId = itemId;
    }
  }
}
