<script setup lang="ts">
import { ref } from "vue";

import { audioManager } from "../audio/audioManager";
import { useAudioSettings } from "../composables/useAudioSettings";
import type { AuthUser, ConnectionState } from "../types/api";

const open = defineModel<boolean>({ required: true });
const shortcutsOpen = ref(false);
defineProps<{
    connection: ConnectionState;
    connectionLabel: string;
    status: string;
    isDemoMode: boolean;
    currentUser?: AuthUser;
}>();

const {
    muted,
    masterVolume,
    bgmVolume,
    sfxVolume,
    handleMutedChange,
    handleMasterVolumeChange,
    handleBgmVolumeChange,
    handleSfxVolumeChange,
    resetSettings,
} = useAudioSettings();

function openDrawer(): void {
    audioManager.unlock({ startBgm: false });
    open.value = true;
}

function closeDrawer(): void {
    open.value = false;
    shortcutsOpen.value = false;
}

function openShortcuts(): void {
    shortcutsOpen.value = true;
}

function closeShortcuts(): void {
    shortcutsOpen.value = false;
}
</script>

<template>
    <button
        type="button"
        class="settingsButton"
        :aria-expanded="open"
        aria-label="音声設定を開く"
        @click.stop="openDrawer"
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
            v-if="open"
            class="settingsBackdrop"
            @click="closeDrawer"
        />
    </Transition>

    <Transition name="settings-slide">
        <aside
            v-if="open"
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
                    @click="closeDrawer"
                >
                    ×
                </button>
            </header>

            <section class="settingsGroup">
                <label class="settingsToggle">
                    <input
                        type="checkbox"
                        :checked="muted"
                        @change="handleMutedChange"
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
                        @input="handleMasterVolumeChange"
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
                        @input="handleBgmVolumeChange"
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
                        @input="handleSfxVolumeChange"
                    />
                </div>

                <button
                    type="button"
                    class="resetSettingsButton"
                    @click="resetSettings"
                >
                    初期値に戻す
                </button>
            </section>

            <section class="settingsGroup">
                <button
                    type="button"
                    class="shortcutsButton"
                    @click="openShortcuts"
                >
                    Shortcuts
                </button>
            </section>

            <footer class="settingsStatus">
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
            </footer>
        </aside>
    </Transition>

    <Transition name="shortcut-fade">
        <div
            v-if="shortcutsOpen"
            class="shortcutOverlay"
            role="presentation"
            @click="closeShortcuts"
        >
            <section
                class="shortcutDialog"
                role="dialog"
                aria-modal="true"
                aria-label="キーボードショートカット"
                @click.stop
                @keydown.esc.stop.prevent="closeShortcuts"
            >
                <header class="shortcutHeader">
                    <div>
                        <p class="eyebrow">keyboard</p>
                        <h2>Shortcuts</h2>
                    </div>
                    <button
                        type="button"
                        class="settingsClose"
                        aria-label="ショートカット一覧を閉じる"
                        @click="closeShortcuts"
                    >
                        ×
                    </button>
                </header>

                <div class="shortcutGrid">
                    <article class="shortcutSection">
                        <h3>Camera</h3>
                        <dl>
                            <div>
                                <dt><kbd>W</kbd><kbd>A</kbd><kbd>S</kbd><kbd>D</kbd></dt>
                                <dd>カメラを移動</dd>
                            </div>
                            <div>
                                <dt><kbd>Q</kbd><kbd>E</kbd></dt>
                                <dd>ズームアウト / ズームイン</dd>
                            </div>
                            <div>
                                <dt><kbd>I</kbd><kbd>J</kbd><kbd>K</kbd><kbd>L</kbd></dt>
                                <dd>選択中ノードを中心に回転</dd>
                            </div>
                        </dl>
                    </article>

                    <article class="shortcutSection">
                        <h3>Navigation</h3>
                        <dl>
                            <div>
                                <dt><kbd>↑</kbd><kbd>↓</kbd></dt>
                                <dd>子 / 親チャンネルへ移動</dd>
                            </div>
                            <div>
                                <dt><kbd>←</kbd><kbd>→</kbd></dt>
                                <dd>兄弟チャンネルへ移動</dd>
                            </div>
                            <div>
                                <dt><kbd>R</kbd></dt>
                                <dd>Grand Root を選択</dd>
                            </div>
                        </dl>
                    </article>

                    <article class="shortcutSection">
                        <h3>App</h3>
                        <dl>
                            <div>
                                <dt><kbd>M</kbd></dt>
                                <dd>ミュート切り替え</dd>
                            </div>
                            <div>
                                <dt><kbd>Esc</kbd></dt>
                                <dd>選択解除 / 設定を開閉</dd>
                            </div>
                        </dl>
                    </article>
                </div>
            </section>
        </div>
    </Transition>
</template>
