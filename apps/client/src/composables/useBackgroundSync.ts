import { onBeforeUnmount, onMounted } from "vue";
import type { Ref } from "vue";

import type { ChannelGraph } from "../core/channelGraph";

export function useBackgroundSync(graph: Ref<ChannelGraph | undefined>): void {
    let backgroundTimer: ReturnType<typeof setInterval> | undefined;
    let backgroundUpdatedAt = 0;

    function handleVisibilityChange(): void {
        if (!graph.value) return;

        if (document.hidden) {
            graph.value.clearVisualEvents();
            backgroundUpdatedAt = performance.now();
            backgroundTimer = setInterval(() => {
                const now = performance.now();
                graph.value?.update((now - backgroundUpdatedAt) / 1000);
                backgroundUpdatedAt = now;
            }, 1000);
            return;
        }

        if (backgroundTimer) clearInterval(backgroundTimer);
        backgroundTimer = undefined;
        graph.value.clearVisualEvents();
        graph.value.requestSyncSnap();
    }

    onMounted(() => {
        document.addEventListener("visibilitychange", handleVisibilityChange);
    });

    onBeforeUnmount(() => {
        document.removeEventListener("visibilitychange", handleVisibilityChange);
        if (backgroundTimer) clearInterval(backgroundTimer);
    });
}
