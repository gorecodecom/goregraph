package example.archive;
public final class ArchiveClientGenerated {
    public interface Transport { byte[] post(String path); }
    private final Transport transport;
    public ArchiveClientGenerated(Transport transport) { this.transport = transport; }
    public byte[] exportReport() { return transport.post("/reports/export"); }
}
