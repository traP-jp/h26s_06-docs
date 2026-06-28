<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

import { audioManager } from "./audio/audioManager";
import ActivityChannels from "./components/ActivityChannels.vue";
import AppMetrics from "./components/AppMetrics.vue";
import AppTopBar from "./components/AppTopBar.vue";
import ChannelDetails from "./components/ChannelDetails.vue";
import DisplayControls from "./components/DisplayControls.vue";
import GalaxyCanvas from "./components/GalaxyCanvas.vue";
import SettingsDrawer from "./components/SettingsDrawer.vue";
import { useAppState } from "./composables/useAppState";
import { useAudioSettings } from "./composables/useAudioSettings";
import { useBackgroundSync } from "./composables/useBackgroundSync";
import { type ChannelDisplayMode, ChannelGraph } from "./core/channelGraph";
import {
    type CameraMoveDirection,
    type CameraRotationDirection,
    type CameraZoomDirection,
    KeyboardController,
} from "./core/keyboardController";
import { beginLogin, fetchCurrentUser } from "./services/auth";
import { calculateChannelLayout } from "./services/channelLayout";
import { ChannelStatus } from "./services/channelStatus";
import { EventStream } from "./services/eventStream";
import { KeyboardManager } from "./services/keyboardManager";
import type { AuthUser, ViewersPayload } from "./types/api";

type AuthState = "checking" | "authenticated" | "error" | "forbidden";
interface GalaxyCanvasControls {
    setCameraMoveActive: (direction: CameraMoveDirection, active: boolean) => void;
    setCameraZoomActive: (direction: CameraZoomDirection, active: boolean) => void;
    setCameraRotationActive: (direction: CameraRotationDirection, active: boolean) => void;
    releaseCameraControls: () => void;
}

const isDemoMode = new URLSearchParams(window.location.search).get("demo") === "1";
const SELECTED_LAYOUT_DEBOUNCE_MS = 120;

let stream: EventStream | undefined;
let pendingGraph: ChannelGraph | undefined;
let layoutGeneration = 0;
let mounted = false;
let authGeneration = 0;
let selectedLayoutTimer: ReturnType<typeof setTimeout> | undefined;

const {
    graph,
    connection,
    status,
    selectedId,
    activeOnly,
    displayMode,
    eventCount,
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
} = useAppState();

const { toggleMuted } = useAudioSettings();

useBackgroundSync(graph);

const authState = ref<AuthState>(isDemoMode ? "authenticated" : "checking");
const currentUser = ref<AuthUser>();
const focusId = ref<string | undefined>();
const focusRevision = ref(0);
const settingsOpen = ref(false);
const detailsOpen = ref(false);
const activity = ref(0);
let pendingStatusChannelId: string | undefined;
let bufferedViewers: ViewersPayload | undefined;

function applyViewers(payload: ViewersPayload): void {
    viewers.value = payload.viewers;
    viewersPending.value = false;
    viewersUnavailable.value = false;
}

const channelStatus = isDemoMode
    ? undefined
    : new ChannelStatus({
          onSending(channelId) {
              pendingStatusChannelId = channelId;
              bufferedViewers = undefined;
          },
          onApplied(channelId) {
              if (pendingStatusChannelId === channelId) pendingStatusChannelId = undefined;
              if (channelId === (selectedId.value ?? "") && bufferedViewers) {
                  applyViewers(bufferedViewers);
                  bufferedViewers = undefined;
              }
          },
          onError(channelId) {
              if (pendingStatusChannelId === channelId) pendingStatusChannelId = undefined;
              if (channelId !== (selectedId.value ?? "")) return;
              viewersPending.value = false;
              viewersUnavailable.value = true;
          },
      });
const galaxyCanvas = ref<GalaxyCanvasControls>();

const showLoading = computed(
    () => authState.value !== "error" && authState.value !== "forbidden" && !graph.value
);
const effectiveActiveOnly = computed(() => displayMode.value === "collapsed" && activeOnly.value);

function reloadPage(): void {
    window.location.reload();
}

function unlockAudio(): void {
    if (authState.value !== "authenticated") return;
    audioManager.unlock();
}

function openSettings(): void {
    if (authState.value !== "authenticated") return;
    audioManager.unlock({ startBgm: false });
    settingsOpen.value = true;
}

function closeSettings(): void {
    settingsOpen.value = false;
}

const keyboardController = new KeyboardController({
    getSelected: () => selected.value,
    getSelectedId: () => selectedId.value,
    setSelectedId: id => {
        selectedId.value = id;
    },
    isSettingsOpen: () => settingsOpen.value,
    onMuteToggle: toggleMuted,
    onSettingsOpen: openSettings,
    onSettingsClose: closeSettings,
    onCameraMoveChange: (direction, active) => {
        galaxyCanvas.value?.setCameraMoveActive(direction, active);
    },
    onCameraZoomChange: (direction, active) => {
        galaxyCanvas.value?.setCameraZoomActive(direction, active);
    },
    onCameraRotateChange: (direction, active) => {
        galaxyCanvas.value?.setCameraRotationActive(direction, active);
    },
    onCameraControlsRelease: () => {
        galaxyCanvas.value?.releaseCameraControls();
    },
});
const keyboardManager = new KeyboardManager(keyboardController);

function calculateCurrentLayout(targetGraph: ChannelGraph) {
    return calculateChannelLayout(targetGraph.nodes, { displayMode: displayMode.value });
}

function scheduleLayout(targetGraph: ChannelGraph, refocus = false): void {
    clearSelectedLayoutTimer();
    const generation = ++layoutGeneration;
    calculateCurrentLayout(targetGraph).then(positions => {
        if (generation === layoutGeneration && graph.value === targetGraph) {
            targetGraph.applyLayout(positions);
            if (refocus && focusId.value) focusRevision.value += 1;
        }
    });
}

function clearSelectedLayoutTimer(): void {
    if (!selectedLayoutTimer) return;
    clearTimeout(selectedLayoutTimer);
    selectedLayoutTimer = undefined;
}

function scheduleSelectedLayout(targetGraph: ChannelGraph, isClosingSelection: boolean): void {
    const generation = ++layoutGeneration;
    clearSelectedLayoutTimer();
    selectedLayoutTimer = setTimeout(() => {
        selectedLayoutTimer = undefined;
        calculateCurrentLayout(targetGraph).then(positions => {
            if (generation === layoutGeneration && graph.value === targetGraph) {
                targetGraph.applyLayout(positions);
                if (!isClosingSelection) audioManager.playBloom();
                if (focusId.value) focusRevision.value += 1;
            }
        });
    }, SELECTED_LAYOUT_DEBOUNCE_MS);
}

function revealMessageNode(id: string): void {
    graph.value?.revealMessageNode(id);
}

function updateGraphVisibility(targetGraph: ChannelGraph, selected?: string) {
    return targetGraph.updateVisibility(selected, undefined, displayMode.value);
}

function focusEventToast(channelId: string, toastId: number): void {
    if (!channelId) return;
    focusChannel(channelId);
    dismissEventToast(toastId);
}

function focusChannel(channelId: string): void {
    selectedId.value = channelId;
    focusId.value = channelId;
    focusRevision.value += 1;
}

async function retryAuthentication() {
    if (isDemoMode) return;
    const authenticated = await refreshAuthentication();
    if (authenticated) {
        connectStream();
    }
}

function stopStream(clearGraph: boolean) {
    layoutGeneration += 1;
    pendingGraph = undefined;
    stream?.disconnect();
    stream = undefined;
    if (clearGraph) {
        clearSelectedLayoutTimer();
        resetActivity();
    }
}

async function refreshAuthentication() {
    const currentGeneration = ++authGeneration;
    authState.value = "checking";
    status.value = "認証状態を確認中";

    try {
        const user = await fetchCurrentUser();
        if (!mounted || currentGeneration !== authGeneration) return false;

        if (!user) {
            stopStream(true);
            connection.value = "closed";
            status.value = "ログインが必要です";
            beginLogin();
            return false;
        }

        currentUser.value = user;
        authState.value = "authenticated";
        return true;
    } catch (error) {
        if (!mounted || currentGeneration !== authGeneration) return false;

        currentUser.value = undefined;
        authState.value = "error";
        stopStream(true);
        connection.value = "closed";
        status.value = error instanceof Error ? error.message : "認証確認エラー";
        return false;
    }
}

async function handleStreamConnectionError() {
    if (isDemoMode) return false;

    try {
        const user = await fetchCurrentUser();
        if (!user) {
            stopStream(true);
            connection.value = "closed";
            status.value = "セッション切れ";
            beginLogin();
            return true;
        }

        currentUser.value = user;
        authState.value = "authenticated";
        return false;
    } catch {
        return false;
    }
}

function connectStream() {
    stopStream(false);
    status.value = isDemoMode ? "デモサーバーへ接続中" : "ライブストリームへ接続中";
    stream = new EventStream({
        demo: isDemoMode,
        onState(nextState, message) {
            connection.value = nextState;
            status.value = message;
        },

        async onInit(channels) {
            const generation = ++layoutGeneration;
            const nextGraph = new ChannelGraph(channels);

            pendingGraph = nextGraph;
            status.value = `${nextGraph.nodes.length.toLocaleString()}チャンネルを配置中`;

            updateGraphVisibility(nextGraph, selectedId.value);

            const positions = await calculateCurrentLayout(nextGraph);

            if (!mounted || generation !== layoutGeneration) return;

            nextGraph.applyLayout(positions, true);
            graph.value = nextGraph;
            pendingGraph = undefined;
            connection.value = "open";
            status.value = isDemoMode ? "デモストリーム受信中" : "ライブストリーム受信中";
        },

        onTrigger(trigger) {
            const targetGraph = pendingGraph ?? graph.value;
            const visibilityChanged = targetGraph?.applyTrigger(trigger) ?? false;
            if (visibilityChanged && targetGraph && targetGraph === graph.value) {
                scheduleLayout(targetGraph);
            }
            recordTrigger(trigger);
        },

        onSync(payload) {
            (pendingGraph ?? graph.value)?.sync(payload.deltas);
            updatedAt.value = new Date(payload.ts * 1000).toLocaleTimeString("ja-JP");

            if (graph.value) {
                const changed = updateGraphVisibility(graph.value, selectedId.value);

                if (changed) {
                    const generation = ++layoutGeneration;

                    calculateCurrentLayout(graph.value).then(positions => {
                        if (generation === layoutGeneration) {
                            graph.value?.applyLayout(positions);
                            if (focusId.value) focusRevision.value += 1;
                        }
                    });
                }
            }
        },

        onViewers(payload) {
            if (!selectedId.value) return;
            if (pendingStatusChannelId !== undefined) {
                if (pendingStatusChannelId === selectedId.value) bufferedViewers = payload;
                return;
            }
            applyViewers(payload);
        },

        onMalformedEvent(eventName) {
            status.value = `${eventName} イベントを解釈できませんでした`;
        },
        onConnectionError: handleStreamConnectionError,
    });

    stream.connect();
}

onMounted(() => {
    mounted = true;
    keyboardManager.start();

    if (isDemoMode) {
        connectStream();
        return;
    }

    const params = new URLSearchParams(window.location.search);
    if (params.get("error") === "forbidden") {
        history.replaceState(null, "", window.location.pathname);
        authState.value = "forbidden";
        return;
    }

    void retryAuthentication();
});

watch(selectedId, (newId, oldId) => {
    detailsOpen.value = Boolean(newId);
    viewers.value = [];
    bufferedViewers = undefined;
    viewersPending.value = Boolean(newId) && !isDemoMode;
    viewersUnavailable.value = isDemoMode;
    channelStatus?.setChannel(newId);

    if (!graph.value) {
        focusId.value = newId;
        return;
    }

    const changed = updateGraphVisibility(graph.value, newId);
    focusId.value = newId;
    const isClosingSelection = !newId && Boolean(oldId);

    if (changed) {
        if (isClosingSelection) {
            audioManager.playClose();
        }
        scheduleSelectedLayout(graph.value, isClosingSelection);
    }
});

watch(displayMode, (mode: ChannelDisplayMode) => {
    const targetGraph = graph.value;
    if (!targetGraph) return;

    const changed = updateGraphVisibility(targetGraph, selectedId.value);
    if (changed) scheduleLayout(targetGraph, mode === "all");
});

onBeforeUnmount(() => {
    keyboardManager.stop();
    mounted = false;
    authGeneration += 1;
    clearEventToasts();
    channelStatus?.setChannel();
    clearSelectedLayoutTimer();
    stopStream(false);
});
</script>

<template>
    <main
        class="app-shell"
        @pointerdown.capture.once="unlockAudio"
    >
        <SettingsDrawer
            v-if="authState === 'authenticated'"
            v-model="settingsOpen"
            :connection="connection"
            :connection-label="connectionLabel"
            :status="status"
            :is-demo-mode="isDemoMode"
            :current-user="currentUser"
        />

        <GalaxyCanvas
            v-if="graph"
            ref="galaxyCanvas"
            :graph="graph"
            :selected-id="selectedId"
            :focus-id="focusId"
            :focus-revision="focusRevision"
            :active-only="effectiveActiveOnly"
            :display-mode="displayMode"
            @select="selectedId = $event"
            @message-node-reached="revealMessageNode"
            @activity-change="activity = $event"
            @render-error="renderError = $event"
        />

        <div
            v-else-if="showLoading"
            class="loading"
        >
            <span class="loading__orbit" />
            <p>QOSMOS を構築中</p>
        </div>

        <div
            v-if="authState === 'error'"
            class="render-error ui-panel"
        >
            <p class="eyebrow">AUTH ERROR</p>
            <strong>認証状態を取得できませんでした</strong>
            <button @click="retryAuthentication">再試行</button>
        </div>

        <div
            v-if="authState === 'forbidden'"
            class="render-error ui-panel"
        >
            <p class="eyebrow">ACCESS DENIED</p>
            <strong>このアカウントはアクセスが許可されていません</strong>
        </div>

        <div
            v-if="renderError"
            class="render-error ui-panel"
        >
            <p class="eyebrow">RENDERER ERROR</p>
            <strong>{{ renderError }}</strong>
            <button @click="reloadPage">再読み込み</button>
        </div>

        <AppTopBar />

        <AppMetrics
            v-if="authState === 'authenticated'"
            :graph="graph"
            :event-count="eventCount"
            :updated-at="updatedAt"
        />

        <ActivityChannels
            v-if="authState === 'authenticated' && graph"
            :graph="graph"
            @select="focusChannel"
        />

        <aside
            v-if="authState === 'authenticated'"
            class="event-toasts"
            aria-live="polite"
            aria-label="イベント通知"
        >
            <TransitionGroup name="event-toast">
                <article
                    v-for="toast in eventToasts"
                    :key="toast.id"
                    class="event-toast"
                    :data-tone="toast.tone"
                >
                    <button
                        type="button"
                        class="event-toast__focus"
                        :aria-label="`${toast.detail}のチャンネルにフォーカス`"
                        @click="focusEventToast(toast.channelId, toast.id)"
                    >
                        <span
                            class="event-toast__signal"
                            aria-hidden="true"
                        />
                        <span class="event-toast__body">
                            {{ toast.detail }}
                        </span>
                    </button>
                    <button
                        type="button"
                        class="event-toast__close"
                        aria-label="イベント通知を閉じる"
                        @click="dismissEventToast(toast.id)"
                    >
                        ×
                    </button>
                </article>
            </TransitionGroup>
        </aside>

        <DisplayControls
            v-if="authState === 'authenticated'"
            v-model="activeOnly"
            v-model:display-mode="displayMode"
        />

        <ChannelDetails
            v-if="selected && detailsOpen"
            :selected="selected"
            :activity="activity"
            :viewer-count="viewersUnavailable ? undefined : viewers.length"
            :viewers-pending="viewersPending"
            @close="detailsOpen = false"
        />

        <footer
            v-if="authState === 'authenticated'"
            class="hint"
        >
            <span>DRAG</span> 移動 <span>SCROLL</span> 拡大・縮小 <span>CLICK</span> 詳細
        </footer>
    </main>
</template>
