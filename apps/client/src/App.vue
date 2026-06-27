<script setup lang="ts">
import { onBeforeUnmount, onMounted } from "vue";
import { ref, watch } from "vue";

import GalaxyCanvas from "./components/GalaxyCanvas.vue";
import { useAppState } from "./composables/useAppState";
import { ChannelGraph } from "./core/channelGraph";
import { calculateChannelLayout } from "./services/channelLayout";
import { EventStream } from "./services/eventStream";

let stream: EventStream | undefined;
let pendingGraph: ChannelGraph | undefined;
let layoutGeneration = 0;
let mounted = false;
let backgroundTimer: ReturnType<typeof setInterval> | undefined;
let backgroundUpdatedAt = 0;
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
} = useAppState();

function handleVisibilityChange() {
    if (document.hidden) {
        graph.value?.clearVisualEvents();
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
    graph.value?.clearVisualEvents();
    graph.value?.requestSyncSnap();
}

function reloadPage() {
    window.location.reload();
}

onMounted(() => {
    mounted = true;
    document.addEventListener("visibilitychange", handleVisibilityChange);
    stream = new EventStream({
        demo: new URLSearchParams(location.search).get("demo") !== "0",
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
            status.value = "デモストリーム受信中";
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
    });
    stream.connect();
});

const focusId = ref<string | undefined>();

watch(selectedId, newId => {
    if (!graph.value) {
        focusId.value = newId;
        return;
    }
    const changed = graph.value.updateVisibility(newId);
    focusId.value = newId; // レイアウト計算を待たずに即座にカメラを移動開始

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
    layoutGeneration += 1;
    stream?.disconnect();
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
            v-else
            class="loading"
        >
            <span class="loading__orbit" />
            <p>CHANNEL UNIVERSE を構築中</p>
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
                <p class="eyebrow">traQ ACTIVITY OBSERVATORY</p>
                <h1>Channel Universe</h1>
            </div>
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
