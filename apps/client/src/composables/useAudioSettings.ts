import { ref } from "vue";

import { audioManager } from "../audio/audioManager";

const muted = ref(audioManager.muted);
const masterVolume = ref(audioManager.masterVolume);
const bgmVolume = ref(audioManager.bgmVolume);
const sfxVolume = ref(audioManager.sfxVolume);

export function useAudioSettings() {
    function toggleMuted(): void {
        muted.value = !muted.value;
        audioManager.setMuted(muted.value);
    }

    function handleMutedChange(event: Event): void {
        const target = event.target as HTMLInputElement;
        muted.value = target.checked;
        audioManager.setMuted(muted.value);
    }

    function handleMasterVolumeChange(event: Event): void {
        const value = Number((event.target as HTMLInputElement).value);
        masterVolume.value = value;
        audioManager.setMasterVolume(value);
    }

    function handleBgmVolumeChange(event: Event): void {
        const value = Number((event.target as HTMLInputElement).value);
        bgmVolume.value = value;
        audioManager.setBgmVolume(value);
    }

    function handleSfxVolumeChange(event: Event): void {
        const value = Number((event.target as HTMLInputElement).value);
        sfxVolume.value = value;
        audioManager.setSfxVolume(value);
    }

    function resetSettings(): void {
        audioManager.resetSettings();
        muted.value = audioManager.muted;
        masterVolume.value = audioManager.masterVolume;
        bgmVolume.value = audioManager.bgmVolume;
        sfxVolume.value = audioManager.sfxVolume;
    }

    return {
        muted,
        masterVolume,
        bgmVolume,
        sfxVolume,
        toggleMuted,
        handleMutedChange,
        handleMasterVolumeChange,
        handleBgmVolumeChange,
        handleSfxVolumeChange,
        resetSettings,
    };
}
