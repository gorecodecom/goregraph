package example;

import org.springframework.data.jpa.repository.JpaRepository;

interface AccountRepository extends JpaRepository<Account, String> {
  Account findByAccountId(String accountId);

  void delete(Account account);
}
