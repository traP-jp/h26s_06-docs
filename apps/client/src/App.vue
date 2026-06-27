<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

import { audioManager } from "./audio/audioManager.ts";
import GalaxyCanvas from "./components/GalaxyCanvas.vue";
import { useAppState } from "./composables/useAppState";
import { ChannelGraph } from "./core/channelGraph";
import { beginLogin, fetchCurrentUser } from "./services/auth";
import { calculateChannelLayout } from "./services/channelLayout";
import { EventStream } from "./services/eventStream";
import type { AuthUser } from "./types/api";

type AuthState = "checking" | "authenticated" | "error" | "forbidden";

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
const currentUser = ref<AuthUser>();
const focusId = ref<string | undefined>();

const showLoading = computed(
    () => authState.value !== "error" && authState.value !== "forbidden" && !graph.value
);

// audio settings
const muted = ref(audioManager.muted);
const masterVolume = ref(audioManager.masterVolume);
const bgmVolume = ref(audioManager.bgmVolume);
const sfxVolume = ref(audioManager.sfxVolume);

// settings drawer
const settingsOpen = ref(false);

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

function reloadPage(): void {
    window.location.reload();
}

function unlockAudio(): void {
    if (authState.value !== "authenticated") return;
    audioManager.unlock();
}

function openSettings(): void {
    audioManager.unlock({ startBgm: false });
    settingsOpen.value = true;
}

function closeSettings(): void {
    settingsOpen.value = false;
}

function changeMuted(event: Event): void {
    const target = event.target as HTMLInputElement;
    muted.value = target.checked;
    audioManager.setMuted(muted.value);
}

function changeMasterVolume(event: Event): void {
    const target = event.target as HTMLInputElement;
    const value = Number(target.value);

    masterVolume.value = value;
    audioManager.setMasterVolume(value);
}

function changeBgmVolume(event: Event): void {
    const target = event.target as HTMLInputElement;
    const value = Number(target.value);

    bgmVolume.value = value;
    audioManager.setBgmVolume(value);
}

function changeSfxVolume(event: Event): void {
    const target = event.target as HTMLInputElement;
    const value = Number(target.value);

    sfxVolume.value = value;
    audioManager.setSfxVolume(value);
}

function resetAudioSettings(): void {
    audioManager.resetSettings();

    muted.value = audioManager.muted;
    masterVolume.value = audioManager.masterVolume;
    bgmVolume.value = audioManager.bgmVolume;
    sfxVolume.value = audioManager.sfxVolume;
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
    <main
        class="app-shell"
        @pointerdown.capture.once="unlockAudio"
    >
        <button
            v-if="authState === 'authenticated'"
            type="button"
            class="settingsButton"
            :aria-expanded="settingsOpen"
            aria-label="音声設定を開く"
            @click.stop="openSettings"
        >
            <svg
                viewBox="0 0 24 24"
                aria-hidden="true"
            >
                <path
                    d="M12 15.25A3.25 3.25 0 1 0 12 8.75a3.25 3.25 0 0 0 0 6.5Zm7.2-3.25c0-.45-.05-.88-.13-1.3l2-1.55-2-3.46-2.47 1a8.12 8.12 0 0 0-2.25-1.3L14 2.75h-4l-.35 2.64A8.12 8.12 0 0 0 7.4 6.7l-2.47-1-2 3.46 2 1.55a7.16 7.16 0 0 0 0 2.6l-2 1.55 2 3.46 2.47-1a8.12 8.12 0 0 0 2.25 1.3l.35 2.64h4l.35-2.64a8.12 8.12 0 0 0 2.25-1.3l2.47 1 2-3.46-2-1.55c.08-.42.13-.85.13-1.3Z"
                />
            </svg>
        </button>

        <Transition name="settings-fade">
            <div
                v-if="settingsOpen"
                class="settingsBackdrop"
                @click="closeSettings"
            />
        </Transition>

        <Transition name="settings-slide">
            <aside
                v-if="settingsOpen"
                class="settingsDrawer"
                role="dialog"
                aria-modal="true"
                aria-label="音声設定"
                @pointerdown.stop
                @wheel.stop
                @click.stop
            >
                <header class="settingsHeader">
                    <div>
                        <p class="eyebrow">settings</p>
                        <h2>Sound</h2>
                    </div>

                    <button
                        type="button"
                        class="settingsClose"
                        aria-label="設定を閉じる"
                        @click="closeSettings"
                    >
                        ×
                    </button>
                </header>

                <section class="settingsGroup">
                    <label class="settingsToggle">
                        <input
                            type="checkbox"
                            :checked="muted"
                            @change="changeMuted"
                        />
                        <span
                            class="settingsToggleIcon"
                            aria-hidden="true"
                        >
                            <svg viewBox="0 0 24 24">
                                <path d="M4 9v6h4l5 4V5L8 9H4Z" />
                                <path d="m17 9 4 4m0-4-4 4" />
                            </svg>
                        </span>
                        <span class="settingsToggleLabel">ミュート</span>
                        <span
                            class="settingsToggleSwitch"
                            aria-hidden="true"
                        />
                    </label>
                </section>

                <section class="settingsGroup">
                    <h3>音量</h3>

                    <div class="volumeControl">
                        <div class="volumeLabel">
                            <label for="master-volume">
                                <svg
                                    viewBox="0 0 24 24"
                                    aria-hidden="true"
                                >
                                    <path d="M4 9v6h4l5 4V5L8 9H4Z" />
                                    <path d="M16 9.5a4 4 0 0 1 0 5m2.5-7.5a7 7 0 0 1 0 10" />
                                </svg>
                                全体音量
                            </label>
                            <output> {{ Math.round(masterVolume * 100) }}% </output>
                        </div>
                        <input
                            id="master-volume"
                            type="range"
                            min="0"
                            max="1"
                            step="0.01"
                            :value="masterVolume"
                            :style="{ '--range-progress': `${masterVolume * 100}%` }"
                            @input="changeMasterVolume"
                        />
                    </div>

                    <div class="volumeControl">
                        <div class="volumeLabel">
                            <label for="bgm-volume">
                                <svg
                                    viewBox="0 0 24 24"
                                    aria-hidden="true"
                                >
                                    <path d="M9 18V6l10-2v12" />
                                    <circle
                                        cx="6"
                                        cy="18"
                                        r="3"
                                    />
                                    <circle
                                        cx="16"
                                        cy="16"
                                        r="3"
                                    />
                                </svg>
                                BGM
                            </label>
                            <output> {{ Math.round(bgmVolume * 100) }}% </output>
                        </div>
                        <input
                            id="bgm-volume"
                            type="range"
                            min="0"
                            max="1"
                            step="0.01"
                            :value="bgmVolume"
                            :style="{ '--range-progress': `${bgmVolume * 100}%` }"
                            @input="changeBgmVolume"
                        />
                    </div>

                    <div class="volumeControl">
                        <div class="volumeLabel">
                            <label for="sfx-volume">
                                <svg
                                    viewBox="0 0 24 24"
                                    aria-hidden="true"
                                >
                                    <path d="M4 9v6h4l5 4V5L8 9H4Z" />
                                    <path d="m17 9 4 4m0-4-4 4" />
                                </svg>
                                SE
                            </label>
                            <output> {{ Math.round(sfxVolume * 100) }}% </output>
                        </div>
                        <input
                            id="sfx-volume"
                            type="range"
                            min="0"
                            max="1"
                            step="0.01"
                            :value="sfxVolume"
                            :style="{ '--range-progress': `${sfxVolume * 100}%` }"
                            @input="changeSfxVolume"
                        />
                    </div>

                    <button
                        type="button"
                        class="resetSettingsButton"
                        @click="resetAudioSettings"
                    >
                        初期値に戻す
                    </button>
                </section>
            </aside>
        </Transition>

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

        <header class="topbar ui-panel">
            <div>
                <img
                    src="./assets/Qosmos_logoless.png"
                    alt="Qosmos logo"
                    class="topbar-logo"
                />
                <p class="eyebrow">traQ ACTIVITY OBSERVATORY</p>
                <h1>Qosmos</h1>
            </div>
            <div
                v-if="authState === 'authenticated'"
                class="topbar__meta"
            >
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
                </div>
            </div>
        </header>

        <aside
            v-if="authState === 'authenticated'"
            class="metrics ui-panel"
        >
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

        <div
            v-if="authState === 'authenticated'"
            class="display-controls ui-panel"
        >
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
            <a
                v-if="selected.path !== '# '"
                class="details__path"
                :href="selected.pathHref"
                target="_blank"
                rel="noopener noreferrer"
            >
                {{ selected.path }}
            </a>
            <dl>
                <div>
                    <dt>ACTIVITY</dt>
                    <dd>{{ (selected.relativeScore * 100).toFixed(0) }}</dd>
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

        <footer
            v-if="authState === 'authenticated'"
            class="hint"
        >
            <span>DRAG</span> 移動 <span>SCROLL</span> 拡大・縮小 <span>CLICK</span> 詳細
        </footer>
    </main>
</template>
