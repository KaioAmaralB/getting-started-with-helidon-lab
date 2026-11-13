package lab.trilha.http;

import io.helidon.json.binding.Json;

public final class Messages {
    private Messages() { }
    @Json.Entity
    public record Request(String message, String summary) { }
    @Json.Entity
    public record Response(String answer, String summary) { }
    @Json.Entity
    public record Health(String status) { }
    @Json.Entity
    public record RuntimeInfo(String javaVersion, boolean virtualThread, String providerMode) { }
}

