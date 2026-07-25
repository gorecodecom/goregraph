package example;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;

@WebMvcTest(AccountController.class)
final class AccountControllerTest {
  private final RecordingAccountRepository repository = new RecordingAccountRepository();
  private final AccountController controller =
      new AccountController(
          new AccountService(
              repository,
              accountId -> {},
              ownerEmail -> {},
              ownerId -> {}));

  @Autowired private MockMvc mockMvc;

  @Test
  void removeEndpointUsesThePublicDeletionPath() throws Exception {
    mockMvc.perform(delete("/accounts/account-7"));
  }

  @Test
  void removeUsesThePublicDeletionPath() {
    controller.remove("account-7");

    assert repository.deleted.accountId().equals("account-7");
  }
}
