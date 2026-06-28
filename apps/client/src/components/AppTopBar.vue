<script setup lang="ts">
import type { AuthUser, ConnectionState } from "../types/api";

defineProps<{
    authState: "checking" | "authenticated" | "error" | "forbidden";
    connection: ConnectionState;
    connectionLabel: string;
    status: string;
    isDemoMode: boolean;
    currentUser?: AuthUser;
}>();
</script>

<template>
    <header class="topbar ui-panel">
        <div class="topbar__brand">
            <img
                src="../assets/Qosmos_logoless.png"
                alt="Qosmos logo"
                class="topbar-logo"
            />
            <div class="topbar__title">
                <p class="eyebrow">traQ ACTIVITY OBSERVATORY</p>
                <h1>Qosmos</h1>
            </div>
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
</template>
