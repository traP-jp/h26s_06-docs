import { defineConfig, loadEnv } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), "");
    const apiProxyTarget =
        env.API_PROXY_TARGET ??
        env.VITE_API_BASE_URL ??
        "http://localhost:8080";

    return {
        plugins: [vue()],
        resolve: {
            tsconfigPaths: true,
        },
        server: {
            proxy: {
                "/api": {
                    target: apiProxyTarget,
                    changeOrigin: true,
                },
            },
        },
    };
});
