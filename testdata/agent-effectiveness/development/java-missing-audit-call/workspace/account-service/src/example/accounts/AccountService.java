package example.accounts;

public final class AccountService {
    public interface AccountRepository { void delete(Account account); }
    public interface AuditPublisher { void record(String event, String accountId); }
    public interface AccountMailSender { void accountRemoved(String accountId); }
    public record Account(String id) {}
    private final AccountRepository repository;
    private final AuditPublisher auditPublisher;
    private final AccountMailSender mailSender;
    public AccountService(AccountRepository repository, AuditPublisher auditPublisher, AccountMailSender mailSender) {
        this.repository = repository;
        this.auditPublisher = auditPublisher;
        this.mailSender = mailSender;
    }
    public void remove(Account account) {
        repository.delete(account);
        mailSender.accountRemoved(account.id());
    }
}
