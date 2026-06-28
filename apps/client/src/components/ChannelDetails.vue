<script setup lang="ts">
import type { SelectedChannel } from "../composables/useAppState";

defineProps<{
    selected: SelectedChannel;
    activity: number;
    viewerCount?: number;
    viewersPending: boolean;
}>();

const emit = defineEmits<{
    close: [];
}>();
</script>

<template>
    <aside class="details ui-panel">
        <button
            class="details__close"
            @click="emit('close')"
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
            <div class="details__activity">
                <dt>
                    ACTIVITY
                    <output>{{ activity }}</output>
                </dt>
                <dd>
                    <progress
                        :value="activity"
                        max="100"
                        :aria-label="`ACTIVITY ${activity}`"
                    />
                </dd>
            </div>
            <div>
                <dt>VIEWERS</dt>
                <dd>{{ viewersPending ? "…" : (viewerCount ?? "—") }}</dd>
            </div>
            <div>
                <dt>CHILDREN</dt>
                <dd>{{ selected.children.length }}</dd>
            </div>
        </dl>
    </aside>
</template>
