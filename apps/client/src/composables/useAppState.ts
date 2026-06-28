import { computed, ref, shallowRef, watch } from "vue";

import type { ChannelDisplayMode, ChannelGraph, ChannelNode } from "../core/channelGraph";
import type { ConnectionState, TriggerPayload } from "../types/api";

const EVENT_TOAST_DURATION_MS = 5200;
const DISPLAY_PRESET_STORAGE_KEY = "qosmos.displayPreset";

type DisplayPreset = "all" | "normal" | "active";

interface EventToast {
    id: number;
    channelId: string;
    tone: "message" | "move";
    detail: string;
}

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

function readDisplayPreset(): DisplayPreset {
    try {
        if (typeof localStorage === "undefined") return "normal";

        const preset = localStorage.getItem(DISPLAY_PRESET_STORAGE_KEY);
        if (preset === "all" || preset === "normal" || preset === "active") return preset;
    } catch {
        // localStorage may be unavailable in restricted browser contexts.
    }

    return "normal";
}

function writeDisplayPreset(preset: DisplayPreset) {
    try {
        if (typeof localStorage === "undefined") return;
        localStorage.setItem(DISPLAY_PRESET_STORAGE_KEY, preset);
    } catch {
        // Display switching should continue even when persistence is unavailable.
    }
}

function displayPresetFor(displayMode: ChannelDisplayMode, activeOnly: boolean): DisplayPreset {
    if (displayMode === "all") return "all";
    return activeOnly ? "active" : "normal";
}

export function useAppState() {
    // ChannelGraph は毎フレーム自身を更新するため、Vue の深い監視から除外する。
    const initialDisplayPreset = readDisplayPreset();
    const graph = shallowRef<ChannelGraph>();
    const connection = ref<ConnectionState>("connecting");
    const status = ref("デモサーバーへ接続中");
    const selectedId = ref<string>();
    const activeOnly = ref(initialDisplayPreset === "active");
    const displayMode = ref<ChannelDisplayMode>(
        initialDisplayPreset === "all" ? "all" : "collapsed"
    );
    const eventCount = ref(0);
    const lastEvent = ref("初期データを待っています");
    const updatedAt = ref("");
    const eventToasts = ref<EventToast[]>([]);
    const renderError = ref<string>();
    const toastTimers = new Map<number, ReturnType<typeof setTimeout>>();
    let nextToastId = 1;
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
        const time = new Date().toLocaleTimeString("ja-JP");

        eventCount.value += 1;
        lastEvent.value =
            trigger.type === "msg"
                ? `#${channelName} にメッセージ`
                : `#${channelName} へユーザーが移動`;
        updatedAt.value = time;
        pushEventToast({
            channelId: id ?? "",
            tone: trigger.type === "msg" ? "message" : "move",
            detail: lastEvent.value,
        });
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
        clearEventToasts();
    }

    function pushEventToast(toast: Omit<EventToast, "id">) {
        const id = nextToastId++;

        clearEventToasts();
        eventToasts.value = [{ ...toast, id }];

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

    watch(selectedId, id => {
        const node = id ? graph.value?.get(id) : undefined;
        if (!node?.parentId) return;

        if (rememberedChildByParent.value[node.parentId] === node.id) return;
        rememberedChildByParent.value = {
            ...rememberedChildByParent.value,
            [node.parentId]: node.id,
        };
    });

    watch([displayMode, activeOnly], ([mode, active]) => {
        writeDisplayPreset(displayPresetFor(mode, active));
    });

    return {
        graph,
        connection,
        status,
        selectedId,
        activeOnly,
        displayMode,
        eventCount,
        lastEvent,
        updatedAt,
        eventToasts,
        renderError,
        viewers,
        viewersPending,
        viewersUnavailable,
        selected,
        connectionLabel,
        recordTrigger,
        resetActivity,
        dismissEventToast,
        clearEventToasts,
    };
}
