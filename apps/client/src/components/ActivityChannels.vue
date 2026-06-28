<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

import type { ChannelGraph } from "../core/channelGraph";
import { type ActivityChannel, rankActivityChannels } from "../services/activityRanking";

const props = defineProps<{
    graph: ChannelGraph;
}>();
const emit = defineEmits<{
    select: [id: string];
}>();

const channels = ref<ActivityChannel[]>([]);
let updateTimer: ReturnType<typeof setInterval> | undefined;

function updateChannels(): void {
    const nextChannels = rankActivityChannels(props.graph.nodes);
    if (
        channels.value.length === nextChannels.length &&
        channels.value.every(
            (channel, index) =>
                channel.id === nextChannels[index]?.id && channel.heat === nextChannels[index]?.heat
        )
    ) {
        return;
    }
    channels.value = nextChannels;
}

watch(() => props.graph, updateChannels, { immediate: true });

onMounted(() => {
    updateTimer = setInterval(updateChannels, 250);
});

onBeforeUnmount(() => {
    if (updateTimer) clearInterval(updateTimer);
});
</script>

<template>
    <section
        class="activity-channels ui-panel"
        aria-labelledby="activity-channels-title"
    >
        <p
            id="activity-channels-title"
            class="eyebrow"
        >
            ACTIVE CHANNELS
        </p>
        <ol v-if="channels.length">
            <li
                v-for="(channel, index) in channels"
                :key="channel.id"
            >
                <button
                    type="button"
                    :aria-label="`#${channel.name}、ACTIVITY ${channel.heat}`"
                    @click="emit('select', channel.id)"
                >
                    <span class="activity-channel__rank">{{ index + 1 }}</span>
                    <span class="activity-channel__body">
                        <span class="activity-channel__summary">
                            <strong>#{{ channel.name }}</strong>
                            <output>{{ channel.heat }}</output>
                        </span>
                        <span
                            class="activity-channel__track"
                            aria-hidden="true"
                        >
                            <span
                                class="activity-channel__heat"
                                :style="{
                                    width: `${channel.heat}%`,
                                    '--channel-color': channel.color,
                                }"
                            />
                        </span>
                    </span>
                </button>
            </li>
        </ol>
        <p
            v-else
            class="activity-channels__empty"
        >
            NO ACTIVITY
        </p>
    </section>
</template>
