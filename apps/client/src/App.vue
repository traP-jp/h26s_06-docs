<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

import GalaxyCanvas from "./components/GalaxyCanvas.vue";
import { useAppState } from "./composables/useAppState";
import { ChannelGraph } from "./core/channelGraph";
import { beginLogin, fetchCurrentUser, logout } from "./services/auth";
import { calculateChannelLayout } from "./services/channelLayout";
import { EventStream } from "./services/eventStream";
import type { AuthUser } from "./types/api";

type AuthState = "checking" | "authenticated" | "unauthenticated" | "error";

const isDemoMode = new URLSearchParams(window.location.search).get("demo") === "1";

let stream: EventStream | undefined;
let pendingGraph: ChannelGraph | undefined;
let layoutGeneration = 0;
let mounted = false;
let backgroundTimer: ReturnType<typeof setInterval> | undefined;
let backgroundUpdatedAt = 0;
let authGeneration = 0;

const {
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
} = useAppState();

const authState = ref<AuthState>(isDemoMode ? "authenticated" : "checking");
const authMessage = ref(isDemoMode ? "デモストリームに接続します" : "traQ のログイン状態を確認中");
const currentUser = ref<AuthUser>();
const focusId = ref<string | undefined>();

const showAuthCover = computed(() => !isDemoMode && authState.value !== "authenticated");
const showLoading = computed(() => !showAuthCover.value && !graph.value);
const accessLabel = computed(() => (isDemoMode ? "DEMO ACCESS" : "traQ OAUTH"));
const authPrimaryLabel = computed(() =>
    authState.value === "checking" ? "確認中..." : "traQ でログイン"
);

function handleVisibilityChange() {
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

function reloadPage() {
    window.location.reload();
}

function handleLogin() {
    beginLogin();
}

async function handleLogout() {
    stopStream(true);
    authState.value = "checking";
    authMessage.value = "ログアウト中";
    currentUser.value = undefined;
    status.value = "ログアウト中";

    try {
        await logout();
        authState.value = "unauthenticated";
        authMessage.value = "ログアウトしました。再度ログインしてください。";
        connection.value = "closed";
        status.value = "ログアウトしました";
    } catch {
        await retryAuthentication();
    }
}

async function retryAuthentication() {
    if (isDemoMode) return;
    const authenticated = await refreshAuthentication("ログインが必要です。");
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
        resetActivity();
    }
}

async function refreshAuthentication(unauthenticatedMessage: string) {
    const currentGeneration = ++authGeneration;
    authState.value = "checking";
    authMessage.value = "traQ のログイン状態を確認中";
    status.value = "認証状態を確認中";

    try {
        const user = await fetchCurrentUser();
        if (!mounted || currentGeneration !== authGeneration) return false;

        if (!user) {
            currentUser.value = undefined;
            authState.value = "unauthenticated";
            authMessage.value = unauthenticatedMessage;
            stopStream(true);
            connection.value = "closed";
            status.value = "ログインが必要です";
            return false;
        }

        currentUser.value = user;
        authState.value = "authenticated";
        authMessage.value = `${user.displayName} でログイン中`;
        return true;
    } catch (error) {
        if (!mounted || currentGeneration !== authGeneration) return false;

        currentUser.value = undefined;
        authState.value = "error";
        authMessage.value = "認証状態を取得できませんでした。時間をおいて再試行してください。";
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
            currentUser.value = undefined;
            authState.value = "unauthenticated";
            authMessage.value = "セッションの有効期限が切れました。再ログインしてください。";
            stopStream(true);
            connection.value = "closed";
            status.value = "セッション切れ";
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
            nextGraph.updateVisibility(selectedId.value);
            const positions = await calculateChannelLayout(nextGraph.nodes);
            if (!mounted || generation !== layoutGeneration) return;
            nextGraph.applyLayout(positions, true);
            graph.value = nextGraph;
            pendingGraph = undefined;
            connection.value = "open";
            status.value = isDemoMode ? "デモストリーム受信中" : "ライブストリーム受信中";
        },
        onTrigger(trigger) {
            (pendingGraph ?? graph.value)?.applyTrigger(trigger);
            recordTrigger(trigger);
        },
        onSync(payload) {
            (pendingGraph ?? graph.value)?.sync(payload.deltas);
            updatedAt.value = new Date(payload.ts * 1000).toLocaleTimeString("ja-JP");
            if (graph.value) {
                const changed = graph.value.updateVisibility(selectedId.value);
                if (changed) {
                    const generation = ++layoutGeneration;
                    calculateChannelLayout(graph.value.nodes).then(positions => {
                        if (generation === layoutGeneration) {
                            graph.value?.applyLayout(positions);
                        }
                    });
                }
            }
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
    document.addEventListener("visibilitychange", handleVisibilityChange);

    if (isDemoMode) {
        authMessage.value = "デモモードで接続中";
        connectStream();
        return;
    }

    void retryAuthentication();
});

watch(selectedId, newId => {
    if (!graph.value) {
        focusId.value = newId;
        return;
    }
    const changed = graph.value.updateVisibility(newId);
    focusId.value = newId;

    if (changed) {
        const generation = ++layoutGeneration;
        calculateChannelLayout(graph.value.nodes).then(positions => {
            if (generation === layoutGeneration) {
                graph.value?.applyLayout(positions);
            }
        });
    }
});

onBeforeUnmount(() => {
    mounted = false;
    authGeneration += 1;
    stopStream(false);
    document.removeEventListener("visibilitychange", handleVisibilityChange);
    if (backgroundTimer) clearInterval(backgroundTimer);
});
</script>

<template>
    <main class="app-shell">
        <GalaxyCanvas
            v-if="graph"
            :graph="graph"
            :selected-id="selectedId"
            :focus-id="focusId"
            :active-only="activeOnly"
            @select="selectedId = $event"
            @render-error="renderError = $event"
        />

        <div
            v-else-if="showLoading"
            class="loading"
        >
            <span class="loading__orbit" />
            <p>CHANNEL UNIVERSE を構築中</p>
        </div>

        <div
            v-if="showAuthCover"
            class="auth-cover"
        >
            <div class="auth-cover__card ui-panel">
                <p class="eyebrow">{{ accessLabel }}</p>
                <h2>Live Stream Requires traQ Login</h2>
                <p class="auth-cover__message">{{ authMessage }}</p>
                <div class="auth-cover__actions">
                    <button
                        :disabled="authState === 'checking'"
                        @click="handleLogin"
                    >
                        {{ authPrimaryLabel }}
                    </button>
                    <button
                        class="auth-cover__secondary"
                        @click="retryAuthentication"
                    >
                        再確認
                    </button>
                </div>
                <p class="auth-cover__hint">
                    URL に ?demo=1 を付けるとデモストリームへ接続できます。
                </p>
            </div>
        </div>

        <div
            v-if="renderError"
            class="render-error ui-panel"
        >
            <p class="eyebrow">RENDERER ERROR</p>
            <strong>{{ renderError }}</strong>
            <button @click="reloadPage">再読み込み</button>
        </div>

        <header class="topbar ui-panel">
            <div>
                <img src="./assets/Qosmos_logoless.png" alt="Qosmos logo" class="topbar-logo" />
                <p class="eyebrow">traQ ACTIVITY OBSERVATORY</p>
                <h1>Qosmos</h1>
            </div>
            <div class="topbar__meta">
                <div
                    class="connection"
                    :data-state="connection"
                >
                    <span class="connection__dot" />
                    <div>
                        <strong>{{ connectionLabel }}</strong>
                        <small>{{ status }}</small>
                    </div>
                </div>
                <div
                    v-if="isDemoMode"
                    class="session-pill"
                >
                    <span class="session-pill__label">MODE</span>
                    <strong>DEMO</strong>
                </div>
                <div
                    v-else-if="currentUser"
                    class="session-pill"
                >
                    <img
                        v-if="currentUser.iconUrl"
                        :src="currentUser.iconUrl"
                        :alt="currentUser.displayName"
                    />
                    <div>
                        <span class="session-pill__label">LOGGED IN</span>
                        <strong>{{ currentUser.displayName }}</strong>
                    </div>
                    <button @click="handleLogout">LOG OUT</button>
                </div>
            </div>
        </header>

        <aside class="metrics ui-panel">
            <p class="eyebrow">STREAM OVERVIEW</p>
            <dl>
                <div>
                    <dt>CHANNELS</dt>
                    <dd>{{ graph?.nodes.length ?? "—" }}</dd>
                </div>
                <div>
                    <dt>IMPULSES</dt>
                    <dd>{{ eventCount }}</dd>
                </div>
            </dl>
            <div class="latest">
                <span>LAST SIGNAL</span>
                <strong>{{ lastEvent }}</strong>
                <time>{{ updatedAt || "—" }}</time>
            </div>
        </aside>

        <div class="display-controls ui-panel">
            <p class="eyebrow">DISPLAY</p>
            <button
                :class="{ active: !activeOnly }"
                @click="activeOnly = false"
            >
                ALL
            </button>
            <button
                :class="{ active: activeOnly }"
                @click="activeOnly = true"
            >
                ACTIVE
            </button>
        </div>

        <aside
            v-if="selected"
            class="details ui-panel"
        >
            <button
                class="details__close"
                @click="selectedId = undefined"
            >
                ×
            </button>
            <p class="eyebrow">SELECTED CHANNEL</p>
            <h2>{{ selected.name }}</h2>
            <p class="details__path">{{ selected.path }}</p>
            <dl>
                <div>
                    <dt>ACTIVITY</dt>
                    <dd>{{ selected.currentScore.toFixed(1) }}</dd>
                </div>
                <div>
                    <dt>DEPTH</dt>
                    <dd>{{ selected.depth }}</dd>
                </div>
                <div>
                    <dt>CHILDREN</dt>
                    <dd>{{ selected.children.length }}</dd>
                </div>
            </dl>
        </aside>

        <footer class="hint">
            <span>DRAG</span> 移動 <span>SCROLL</span> 拡大・縮小 <span>CLICK</span> 詳細
        </footer>
    </main>
</template>
