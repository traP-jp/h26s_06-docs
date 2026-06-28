<script setup lang="ts">
import type { SelectedChannel } from "../composables/useAppState";

defineProps<{
    selected: SelectedChannel;
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
</template>
