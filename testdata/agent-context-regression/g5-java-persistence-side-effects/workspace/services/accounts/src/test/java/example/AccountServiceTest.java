package example;

import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;

final class AccountServiceTest {
  private final RecordingAccountRepository repository = new RecordingAccountRepository();
  private final List<String> effects = new ArrayList<>();
  private final AccountService service =
      new AccountService(
          repository,
          accountId -> effects.add("audit:" + accountId),
          ownerEmail -> effects.add("mail:" + ownerEmail),
          ownerId -> effects.add("directory:" + ownerId));

  @Test
  void removeDeletesAndPublishesEveryBusinessSideEffect() {
    service.remove("account-7");

    assert repository.deleted.accountId().equals("account-7");
    assert effects.equals(
        List.of(
            "audit:account-7",
            "mail:owner@example.test",
            "directory:owner-3"));
  }
}

final class RecordingAccountRepository implements AccountRepository {
  Account deleted;

  @Override
  public Account findByAccountId(String accountId) {
    return new Account(accountId, "owner@example.test", "owner-3");
  }

  @Override
  public void delete(Account account) {
    deleted = account;
  }
}
