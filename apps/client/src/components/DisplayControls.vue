<script setup lang="ts">
const activeOnly = defineModel<boolean>({ required: true });
</script>

<template>
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
</template>
