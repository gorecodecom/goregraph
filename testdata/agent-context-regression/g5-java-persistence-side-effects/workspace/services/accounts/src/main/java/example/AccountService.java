package example;

import java.util.List;

final class AccountService {
  private final AccountRepository accountRepository;
  private final AuditLog auditLog;
  private final MailSender mailSender;
  private final UserDirectory userDirectory;

  AccountService(
      AccountRepository accountRepository,
      AuditLog auditLog,
      MailSender mailSender,
      UserDirectory userDirectory) {
    this.accountRepository = accountRepository;
    this.auditLog = auditLog;
    this.mailSender = mailSender;
    this.userDirectory = userDirectory;
  }

  void remove(String accountId) {
    Account account = accountRepository.findByAccountId(accountId);
    accountRepository.delete(account);
    auditLog.recordAccountRemoval(accountId);
    mailSender.sendAccountRemoved(account.ownerEmail());
    userDirectory.invalidate(account.ownerId());
  }

  Account create(Account account) {
    return accountRepository.save(account);
  }

  List<Account> findAll() {
    return accountRepository.findAll();
  }
}
