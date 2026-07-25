package example;

final class JobHousekeeping {
  void publishRemoval(String catalogId, String itemId) {
    System.out.printf("job-removal catalog=%s item=%s%n", catalogId, itemId);
  }
}
