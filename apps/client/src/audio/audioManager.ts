import bgmUrl from "./bgm.mp3";
import bloomUrl from "./bloom.mp3";
import closeUrl from "./close.mp3";
import moveUrl from "./move.mp3";
import postUrl from "./post.mp3";

type SfxName = "post" | "move" | "bloom" | "close";
type AudioName = "bgm" | SfxName;

type StorageKeys = {
    muted: string;
    masterVolume: string;
    bgmVolume: string;
    sfxVolume: string;
};

type AudioSettings = {
    muted: boolean;
    masterVolume: number;
    bgmVolume: number;
    sfxVolume: number;
};

type UnlockOptions = {
    startBgm?: boolean;
};

type AudioOptions = {
    loop?: boolean;
};

const STORAGE_KEYS: StorageKeys = {
    muted: "audio-muted",
    masterVolume: "audio-master-volume",
    bgmVolume: "audio-bgm-volume",
    sfxVolume: "audio-sfx-volume",
};

const DEFAULT_SETTINGS: AudioSettings = {
    muted: false,
    masterVolume: 0.5,
    bgmVolume: 0.3,
    sfxVolume: 0.4,
};

const AUDIO_VOLUME_MULTIPLIERS = {
    bgm: 1.0,
    post: 0.5,
    move: 0.4,
    bloom: 1.8,
    close: 2.3,
} as const satisfies Record<AudioName, number>;

const VOLUME_FADE_DURATION_MS = 600;

class AudioManager {
    public unlocked: boolean;

    public muted: boolean;
    public masterVolume: number;
    public bgmVolume: number;
    public sfxVolume: number;

    private bgm: HTMLAudioElement;
    private sfxPools: Record<SfxName, HTMLAudioElement[]>;
    private lastPlayedAt: Record<SfxName, number>;
    private cooldownMs: Record<SfxName, number>;
    private maxActiveSfx: Record<SfxName | "total", number>;
    private volumeAnimationFrame: number | undefined;

    constructor() {
        this.unlocked = false;

        this.muted = this.loadBoolean(STORAGE_KEYS.muted, DEFAULT_SETTINGS.muted);

        this.masterVolume = this.loadVolume(
            STORAGE_KEYS.masterVolume,
            DEFAULT_SETTINGS.masterVolume
        );

        this.bgmVolume = this.loadVolume(STORAGE_KEYS.bgmVolume, DEFAULT_SETTINGS.bgmVolume);

        this.sfxVolume = this.loadVolume(STORAGE_KEYS.sfxVolume, DEFAULT_SETTINGS.sfxVolume);

        this.bgm = this.createAudio(bgmUrl, { loop: true });

        this.sfxPools = {
            post: this.createAudioPool(postUrl, 4),
            move: this.createAudioPool(moveUrl, 4),
            bloom: this.createAudioPool(bloomUrl, 4),
            close: this.createAudioPool(closeUrl, 4),
        };

        this.lastPlayedAt = {
            post: 0,
            move: 0,
            bloom: 0,
            close: 0,
        };

        this.cooldownMs = {
            post: 100,
            move: 150,
            bloom: 200,
            close: 200,
        };

        this.maxActiveSfx = {
            total: 5,
            post: 3,
            move: 2,
            bloom: 2,
            close: 2,
        };

        this.applyMuted();
        this.applyVolumes();

        console.log("audioManager loaded", {
            muted: this.muted,
            masterVolume: this.masterVolume,
            bgmVolume: this.bgmVolume,
            sfxVolume: this.sfxVolume,
        });
    }

    loadVolume(key: string, defaultValue: number): number {
        const raw = localStorage.getItem(key);

        if (raw === null) {
            return defaultValue;
        }

        const value = Number(raw);

        if (!Number.isFinite(value)) {
            return defaultValue;
        }

        return this.clampVolume(value);
    }

    loadBoolean(key: string, defaultValue: boolean): boolean {
        const raw = localStorage.getItem(key);

        if (raw === null) {
            return defaultValue;
        }

        return raw === "true";
    }

    clampVolume(value: number): number {
        return Math.max(0, Math.min(1, Number(value)));
    }

    createAudio(source: string, options: AudioOptions = {}): HTMLAudioElement {
        const audio = new Audio(source);
        audio.preload = "auto";
        audio.loop = Boolean(options.loop);
        audio.muted = this.muted;
        return audio;
    }

    createAudioPool(source: string, size: number): HTMLAudioElement[] {
        return Array.from({ length: size }, () => this.createAudio(source));
    }

    getAllAudioElements(): HTMLAudioElement[] {
        return [
            this.bgm,
            ...this.sfxPools.post,
            ...this.sfxPools.move,
            ...this.sfxPools.bloom,
            ...this.sfxPools.close,
        ];
    }

    applyMuted(): void {
        for (const audio of this.getAllAudioElements()) {
            audio.muted = this.muted;
        }
    }

    getTargetVolume(categoryVolume: number, audioName: AudioName): number {
        if (this.muted) return 0;

        return this.clampVolume(
            this.masterVolume * categoryVolume * AUDIO_VOLUME_MULTIPLIERS[audioName]
        );
    }

    applyVolumes(durationMs = 0): void {
        if (this.volumeAnimationFrame !== undefined) {
            cancelAnimationFrame(this.volumeAnimationFrame);
            this.volumeAnimationFrame = undefined;
        }

        const sfxTargets = (Object.keys(this.sfxPools) as SfxName[]).flatMap(name =>
            this.sfxPools[name].map(
                audio => [audio, this.getTargetVolume(this.sfxVolume, name)] as const
            )
        );
        const targets = new Map<HTMLAudioElement, number>([
            [this.bgm, this.getTargetVolume(this.bgmVolume, "bgm")],
            ...sfxTargets,
        ]);

        if (durationMs <= 0) {
            for (const [audio, targetVolume] of targets) {
                audio.volume = targetVolume;
            }
            this.applyMuted();
            return;
        }

        const initialVolumes = new Map([...targets.keys()].map(audio => [audio, audio.volume]));
        const startedAt = performance.now();

        const updateVolumes = (now: number): void => {
            const progress = this.clampVolume((now - startedAt) / durationMs);

            for (const [audio, targetVolume] of targets) {
                const initialVolume = initialVolumes.get(audio) ?? targetVolume;
                const volume = initialVolume + (targetVolume - initialVolume) * progress;
                audio.volume = this.clampVolume(volume);
            }

            if (progress < 1) {
                this.volumeAnimationFrame = requestAnimationFrame(updateVolumes);
                return;
            }

            this.volumeAnimationFrame = undefined;
            this.applyMuted();
        };

        this.volumeAnimationFrame = requestAnimationFrame(updateVolumes);
    }

    unlock({ startBgm = true }: UnlockOptions = {}): void {
        console.log("audio unlock", {
            startBgm,
            alreadyUnlocked: this.unlocked,
        });

        if (this.unlocked) return;

        this.unlocked = true;

        if (startBgm) {
            this.playBgm();
        }
    }

    isUnlocked(): boolean {
        return this.unlocked;
    }

    playBgm(): void {
        if (!this.unlocked) return;
        if (!this.bgm.paused) return;

        this.bgm.play().catch((error: unknown) => {
            console.warn("BGMの再生に失敗しました", error);
        });
    }

    pauseBgm(): void {
        this.bgm.pause();
    }

    stopBgm(): void {
        this.bgm.pause();
        this.bgm.currentTime = 0;
    }

    setMuted(muted: boolean): void {
        this.muted = Boolean(muted);
        localStorage.setItem(STORAGE_KEYS.muted, String(this.muted));

        if (!this.muted) {
            this.applyMuted();
        }

        this.applyVolumes(VOLUME_FADE_DURATION_MS);
    }

    setMasterVolume(volume: number): void {
        this.masterVolume = this.clampVolume(volume);
        localStorage.setItem(STORAGE_KEYS.masterVolume, String(this.masterVolume));
        this.applyVolumes(VOLUME_FADE_DURATION_MS);
    }

    setBgmVolume(volume: number): void {
        this.bgmVolume = this.clampVolume(volume);
        localStorage.setItem(STORAGE_KEYS.bgmVolume, String(this.bgmVolume));
        this.applyVolumes(VOLUME_FADE_DURATION_MS);
    }

    setSfxVolume(volume: number): void {
        this.sfxVolume = this.clampVolume(volume);
        localStorage.setItem(STORAGE_KEYS.sfxVolume, String(this.sfxVolume));
        this.applyVolumes(VOLUME_FADE_DURATION_MS);
    }

    resetSettings(): void {
        this.muted = DEFAULT_SETTINGS.muted;
        this.masterVolume = DEFAULT_SETTINGS.masterVolume;
        this.bgmVolume = DEFAULT_SETTINGS.bgmVolume;
        this.sfxVolume = DEFAULT_SETTINGS.sfxVolume;

        localStorage.setItem(STORAGE_KEYS.muted, String(this.muted));
        localStorage.setItem(STORAGE_KEYS.masterVolume, String(this.masterVolume));
        localStorage.setItem(STORAGE_KEYS.bgmVolume, String(this.bgmVolume));
        localStorage.setItem(STORAGE_KEYS.sfxVolume, String(this.sfxVolume));

        this.applyMuted();
        this.applyVolumes(VOLUME_FADE_DURATION_MS);
    }

    getSettings(): AudioSettings {
        return {
            muted: this.muted,
            masterVolume: this.masterVolume,
            bgmVolume: this.bgmVolume,
            sfxVolume: this.sfxVolume,
        };
    }

    getActiveSfxCount(name: SfxName): number {
        const pool = this.sfxPools[name];

        if (!pool) return 0;

        return pool.filter(audio => !audio.paused && !audio.ended).length;
    }

    getTotalActiveSfxCount(): number {
        return (Object.keys(this.sfxPools) as SfxName[]).reduce((sum, name) => {
            return sum + this.getActiveSfxCount(name);
        }, 0);
    }

    canPlaySfx(name: SfxName): boolean {
        if (!this.unlocked) return false;
        if (this.muted) return false;
        if (!this.sfxPools[name]) return false;

        const now = Date.now();
        const last = this.lastPlayedAt[name] ?? 0;
        const cooldown = this.cooldownMs[name] ?? 0;

        if (now - last < cooldown) return false;

        if (this.getTotalActiveSfxCount() >= this.maxActiveSfx.total) {
            return false;
        }

        if (this.getActiveSfxCount(name) >= this.maxActiveSfx[name]) {
            return false;
        }

        return true;
    }

    playSfx(name: SfxName): void {
        if (!this.canPlaySfx(name)) {
            return;
        }

        const pool = this.sfxPools[name];
        const audio = pool.find(item => item.paused || item.ended);

        if (!audio) {
            return;
        }

        this.lastPlayedAt[name] = Date.now();
        audio.currentTime = 0;

        audio
            .play()
            .then(() => {
                console.log("sfx played", name);
            })
            .catch((error: unknown) => {
                console.warn(`${name} の効果音再生に失敗しました`, error);
            });
    }

    playPost(): void {
        this.playSfx("post");
    }

    playMove(): void {
        this.playSfx("move");
    }

    playBloom(): void {
        this.playSfx("bloom");
    }

    playClose(): void {
        this.playSfx("close");
    }
}

export const audioManager = new AudioManager();
