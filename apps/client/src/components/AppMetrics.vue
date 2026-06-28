<script setup lang="ts">
import type { ChannelGraph } from "../core/channelGraph";

defineProps<{
    graph?: ChannelGraph;
    eventCount: number;
    lastEvent: string;
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
        <div class="latest">
            <span>LAST SIGNAL</span>
            <strong>{{ lastEvent }}</strong>
            <time>{{ updatedAt || "—" }}</time>
        </div>
    </aside>
</template>
