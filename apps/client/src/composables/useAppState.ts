import { computed, ref, shallowRef, watch } from "vue";

import type { ChannelGraph, ChannelNode } from "../core/channelGraph";
import type { ConnectionState, TriggerPayload } from "../types/api";

export interface NavigationTargets {
    parentId?: string;
    childId?: string;
    previousSiblingId?: string;
    nextSiblingId?: string;
}

export type SelectedChannel = ChannelNode & {
    path: string;
    pathHref: string;
    navigation: NavigationTargets;
};

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
    const viewers = ref<string[]>([]);
    const viewersPending = ref(false);
    const viewersUnavailable = ref(false);
    const rememberedChildByParent = ref<Record<string, string>>({});

    const selected = computed(() => {
        const channel = selectedId.value ? graph.value?.get(selectedId.value) : undefined;
        if (!channel) return undefined;
        const pathNodes = graph.value?.path(channel.id) ?? [];
        const channelPath = pathNodes
            .filter(node => node.id !== "grand_root")
            .map(node => node.name)
            .join(" / ");
        return {
            ...channel,
            path: `# ${channelPath}`,
            pathHref: `https://q.trap.jp/channels/${channelPath.replaceAll(" / ", "/")}`,
            navigation:
                graph.value?.navigationTargets(
                    channel.id,
                    rememberedChildByParent.value[channel.id]
                ) ?? {},
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
        viewers.value = [];
        viewersPending.value = false;
        viewersUnavailable.value = false;
        rememberedChildByParent.value = {};
        eventCount.value = 0;
        lastEvent.value = "初期データを待っています";
        updatedAt.value = "";
    }

    watch(selectedId, id => {
        const node = id ? graph.value?.get(id) : undefined;
        if (!node?.parentId) return;

        if (rememberedChildByParent.value[node.parentId] === node.id) return;
        rememberedChildByParent.value = {
            ...rememberedChildByParent.value,
            [node.parentId]: node.id,
        };
    });

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
        viewers,
        viewersPending,
        viewersUnavailable,
        selected,
        connectionLabel,
        recordTrigger,
        resetActivity,
    };
}
