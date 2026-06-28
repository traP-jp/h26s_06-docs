<script setup lang="ts">
import type { ChannelGraph } from "../core/channelGraph";

defineProps<{
    graph?: ChannelGraph;
    eventCount: number;
    updatedAt: string;
}>();
</script>

<template>
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
        <div class="stream-clock">
            <span>LAST UPDATE</span>
            <time>{{ updatedAt || "—" }}</time>
        </div>
    </aside>
</template>
