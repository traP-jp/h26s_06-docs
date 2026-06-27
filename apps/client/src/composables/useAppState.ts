import { computed, ref, shallowRef } from "vue";

import type { ChannelGraph } from "../core/channelGraph";
import type { ConnectionState, TriggerPayload } from "../types/api";

export function useAppState() {
    // ChannelGraph は毎フレーム自身を更新するため、Vue の深い監視から除外する。
    const graph = shallowRef<ChannelGraph>();
    const connection = ref<ConnectionState>("connecting");
    const status = ref("デモサーバーへ接続中");
    const selectedId = ref<string>();
    const activeOnly = ref(false);
    const eventCount = ref(0);
    const lastEvent = ref("初期データを待っています");
    const updatedAt = ref("");
    const renderError = ref<string>();

    const selected = computed(() => {
        const channel = selectedId.value ? graph.value?.get(selectedId.value) : undefined;
        if (!channel) return undefined;
        return {
            ...channel,
            path: graph.value
                ?.path(channel.id)
                .map(node => node.name)
                .join(" / "),
        };
    });

    const connectionLabel = computed(() => {
        if (connection.value === "open") return "LIVE";
        if (connection.value === "connecting") return "CONNECTING";
        return "OFFLINE";
    });

    function recordTrigger(trigger: TriggerPayload) {
        const id = trigger.type === "msg" ? trigger.ch : trigger.to;
        const channelName = id ? (graph.value?.get(id)?.name ?? id) : "unknown";
        eventCount.value += 1;
        lastEvent.value =
            trigger.type === "msg"
                ? `${channelName} にメッセージ`
                : `${channelName} へユーザーが移動`;
        updatedAt.value = new Date().toLocaleTimeString("ja-JP");
    }

    function resetActivity() {
        graph.value = undefined;
        selectedId.value = undefined;
        eventCount.value = 0;
        lastEvent.value = "初期データを待っています";
        updatedAt.value = "";
    }

    return {
        graph,
        connection,
        status,
        selectedId,
        activeOnly,
        eventCount,
        lastEvent,
        updatedAt,
        renderError,
        selected,
        connectionLabel,
        recordTrigger,
        resetActivity,
    };
}
