package example;

import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
final class AccountController {
  private final AccountService accountService;

  AccountController(AccountService accountService) {
    this.accountService = accountService;
  }

  @DeleteMapping("/accounts/{accountId}")
  void remove(@PathVariable String accountId) {
    accountService.remove(accountId);
  }

  @PostMapping("/accounts")
  void create(@RequestBody Account account) {
    accountService.create(account);
  }
}
