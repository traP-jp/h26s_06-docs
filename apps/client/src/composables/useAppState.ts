import { computed, ref, shallowRef } from "vue";

import type { ChannelGraph } from "../core/channelGraph";
import type { ConnectionState, TriggerPayload } from "../types/api";

const EVENT_TOAST_DURATION_MS = 5200;
const MAX_EVENT_TOASTS = 4;

interface EventToast {
    id: number;
    tone: "message" | "move";
    title: string;
    detail: string;
    time: string;
}

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
    const eventToasts = ref<EventToast[]>([]);
    const renderError = ref<string>();
    const toastTimers = new Map<number, ReturnType<typeof setTimeout>>();
    let nextToastId = 1;

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
        const time = new Date().toLocaleTimeString("ja-JP");

        eventCount.value += 1;
        lastEvent.value =
            trigger.type === "msg"
                ? `${channelName} にメッセージ`
                : `${channelName} へユーザーが移動`;
        updatedAt.value = time;
        pushEventToast({
            tone: trigger.type === "msg" ? "message" : "move",
            title: trigger.type === "msg" ? "メッセージを検知" : "ユーザー移動を検知",
            detail: lastEvent.value,
            time,
        });
    }

    function resetActivity() {
        graph.value = undefined;
        selectedId.value = undefined;
        eventCount.value = 0;
        lastEvent.value = "初期データを待っています";
        updatedAt.value = "";
        clearEventToasts();
    }

    function pushEventToast(toast: Omit<EventToast, "id">) {
        const id = nextToastId++;

        eventToasts.value = [...eventToasts.value, { ...toast, id }].slice(-MAX_EVENT_TOASTS);
        for (const staleId of toastTimers.keys()) {
            if (eventToasts.value.some(current => current.id === staleId)) continue;
            clearEventToastTimer(staleId);
        }

        toastTimers.set(
            id,
            setTimeout(() => {
                dismissEventToast(id);
            }, EVENT_TOAST_DURATION_MS)
        );
    }

    function dismissEventToast(id: number) {
        clearEventToastTimer(id);
        eventToasts.value = eventToasts.value.filter(toast => toast.id !== id);
    }

    function clearEventToastTimer(id: number) {
        const timer = toastTimers.get(id);
        if (timer) clearTimeout(timer);
        toastTimers.delete(id);
    }

    function clearEventToasts() {
        for (const timer of toastTimers.values()) {
            clearTimeout(timer);
        }

        toastTimers.clear();
        eventToasts.value = [];
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
        eventToasts,
        renderError,
        selected,
        connectionLabel,
        recordTrigger,
        resetActivity,
        dismissEventToast,
        clearEventToasts,
    };
}
