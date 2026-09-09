package example.orders;

public final class OrderService {
  public interface Repository {
    void cancel(String id);
  }

  public interface Publisher {
    void publish(OrderCancelled event);
  }

  public interface TransactionHooks {
    void afterCommit(Runnable callback);
  }

  public record OrderCancelled(String orderId) {
  }

  private final Repository repository;
  private final Publisher publisher;
  private final TransactionHooks transactions;

  public OrderService(Repository repository, Publisher publisher, TransactionHooks transactions) {
    this.repository = repository;
    this.publisher = publisher;
    this.transactions = transactions;
  }

  public void cancel(String id) {
    repository.cancel(id);
    publisher.publish(new OrderCancelled(id));
  }
}
