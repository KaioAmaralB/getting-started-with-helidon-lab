package lab.trilha.mcp;

import io.helidon.logging.common.LogConfig;
import io.helidon.service.registry.Service;
import io.helidon.service.registry.ServiceRegistryManager;

/** Inicia os serviços descobertos pelo binding gerado durante o build. */
@Service.GenerateBinding
public final class Main {
    static {
        LogConfig.initClass();
    }

    private Main() {
    }

    public static void main(String[] args) {
        LogConfig.configureRuntime();
        ServiceRegistryManager.start(ApplicationBinding.create());
    }
}
