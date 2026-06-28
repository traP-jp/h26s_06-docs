<script setup lang="ts">
import type { ChannelDisplayMode } from "../core/channelGraph";

const activeOnly = defineModel<boolean>({ required: true });
const displayMode = defineModel<ChannelDisplayMode>("displayMode", { required: true });

function showAllChannels(): void {
    displayMode.value = "all";
    activeOnly.value = false;
}

function showNormalChannels(): void {
    displayMode.value = "collapsed";
    activeOnly.value = false;
}

function showActiveChannels(): void {
    displayMode.value = "collapsed";
    activeOnly.value = true;
}
</script>

<template>
    <div class="display-controls ui-panel">
        <p class="eyebrow">DISPLAY</p>
        <button
            :class="{ active: displayMode === 'all' }"
            @click="showAllChannels"
        >
            ALL
        </button>
        <button
            :class="{ active: displayMode === 'collapsed' && !activeOnly }"
            @click="showNormalChannels"
        >
            NORMAL
        </button>
        <button
            :class="{ active: displayMode === 'collapsed' && activeOnly }"
            @click="showActiveChannels"
        >
            ACTIVE
        </button>
    </div>
</template>
