import bgmUrl from "./bgm.mp3";
import postUrl from "./post.mp3";
import moveUrl from "./move.mp3";

type SfxName = "post" | "move";

type StorageKeys = {
    muted: string;
    masterVolume: string;
    bgmVolume: string;
    postVolume: string;
    moveVolume: string;
};

type AudioSettings = {
    muted: boolean;
    masterVolume: number;
    bgmVolume: number;
    postVolume: number;
    moveVolume: number;
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
    postVolume: "audio-post-volume",
    moveVolume: "audio-move-volume",
};

const DEFAULT_SETTINGS: AudioSettings = {
    muted: false,
    masterVolume: 0.5,
    bgmVolume: 0.3,
    postVolume: 0.1,
    moveVolume: 0.1,
};

class AudioManager {
    public unlocked: boolean;

    public muted: boolean;
    public masterVolume: number;
    public bgmVolume: number;
    public postVolume: number;
    public moveVolume: number;

    private bgm: HTMLAudioElement;
    private sfxPools: Record<SfxName, HTMLAudioElement[]>;
    private lastPlayedAt: Record<SfxName, number>;
    private cooldownMs: Record<SfxName, number>;
    private maxActiveSfx: Record<SfxName | "total", number>;

    constructor() {
        this.unlocked = false;

        this.muted = this.loadBoolean(
            STORAGE_KEYS.muted,
            DEFAULT_SETTINGS.muted,
        );

        this.masterVolume = this.loadVolume(
            STORAGE_KEYS.masterVolume,
            DEFAULT_SETTINGS.masterVolume,
        );

        this.bgmVolume = this.loadVolume(
            STORAGE_KEYS.bgmVolume,
            DEFAULT_SETTINGS.bgmVolume,
        );

        this.postVolume = this.loadVolume(
            STORAGE_KEYS.postVolume,
            DEFAULT_SETTINGS.postVolume,
        );

        this.moveVolume = this.loadVolume(
            STORAGE_KEYS.moveVolume,
            DEFAULT_SETTINGS.moveVolume,
        );

        this.bgm = this.createAudio(bgmUrl, { loop: true });

        this.sfxPools = {
            post: this.createAudioPool(postUrl, 4),
            move: this.createAudioPool(moveUrl, 4),
        };

        this.lastPlayedAt = {
            post: 0,
            move: 0,
        };

        this.cooldownMs = {
            post: 100,
            move: 150,
        };

        this.maxActiveSfx = {
            total: 5,
            post: 3,
            move: 2,
        };

        this.applyMuted();
        this.applyVolumes();

        console.log("audioManager loaded", {
            muted: this.muted,
            masterVolume: this.masterVolume,
            bgmVolume: this.bgmVolume,
            postVolume: this.postVolume,
            moveVolume: this.moveVolume,
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

    createAudio(src: string, options: AudioOptions = {}): HTMLAudioElement {
        const audio = new Audio(src);
        audio.preload = "auto";
        audio.loop = Boolean(options.loop);
        audio.muted = this.muted;
        return audio;
    }

    createAudioPool(src: string, size: number): HTMLAudioElement[] {
        return Array.from({ length: size }, () => this.createAudio(src));
    }

    getAllAudioElements(): HTMLAudioElement[] {
        return [
            this.bgm,
            ...this.sfxPools.post,
            ...this.sfxPools.move,
        ];
    }

    applyMuted(): void {
        for (const audio of this.getAllAudioElements()) {
            audio.muted = this.muted;
        }
    }

    applyVolumes(): void {
        this.bgm.volume = this.clampVolume(
            this.masterVolume * this.bgmVolume,
        );

        for (const audio of this.sfxPools.post) {
            audio.volume = this.clampVolume(
                this.masterVolume * this.postVolume,
            );
        }

        for (const audio of this.sfxPools.move) {
            audio.volume = this.clampVolume(
                this.masterVolume * this.moveVolume,
            );
        }
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
        this.applyMuted();
    }

    setMasterVolume(volume: number): void {
        this.masterVolume = this.clampVolume(volume);
        localStorage.setItem(
            STORAGE_KEYS.masterVolume,
            String(this.masterVolume),
        );
        this.applyVolumes();
    }

    setBgmVolume(volume: number): void {
        this.bgmVolume = this.clampVolume(volume);
        localStorage.setItem(
            STORAGE_KEYS.bgmVolume,
            String(this.bgmVolume),
        );
        this.applyVolumes();
    }

    setPostVolume(volume: number): void {
        this.postVolume = this.clampVolume(volume);
        localStorage.setItem(
            STORAGE_KEYS.postVolume,
            String(this.postVolume),
        );
        this.applyVolumes();
    }

    setMoveVolume(volume: number): void {
        this.moveVolume = this.clampVolume(volume);
        localStorage.setItem(
            STORAGE_KEYS.moveVolume,
            String(this.moveVolume),
        );
        this.applyVolumes();
    }

    setSfxVolume(volume: number): void {
        this.setPostVolume(volume);
        this.setMoveVolume(volume);
    }

    resetSettings(): void {
        this.muted = DEFAULT_SETTINGS.muted;
        this.masterVolume = DEFAULT_SETTINGS.masterVolume;
        this.bgmVolume = DEFAULT_SETTINGS.bgmVolume;
        this.postVolume = DEFAULT_SETTINGS.postVolume;
        this.moveVolume = DEFAULT_SETTINGS.moveVolume;

        localStorage.setItem(STORAGE_KEYS.muted, String(this.muted));
        localStorage.setItem(
            STORAGE_KEYS.masterVolume,
            String(this.masterVolume),
        );
        localStorage.setItem(STORAGE_KEYS.bgmVolume, String(this.bgmVolume));
        localStorage.setItem(STORAGE_KEYS.postVolume, String(this.postVolume));
        localStorage.setItem(STORAGE_KEYS.moveVolume, String(this.moveVolume));

        this.applyMuted();
        this.applyVolumes();
    }

    getSettings(): AudioSettings {
        return {
            muted: this.muted,
            masterVolume: this.masterVolume,
            bgmVolume: this.bgmVolume,
            postVolume: this.postVolume,
            moveVolume: this.moveVolume,
        };
    }

    getActiveSfxCount(name: SfxName): number {
        const pool = this.sfxPools[name];

        if (!pool) return 0;

        return pool.filter((audio) => !audio.paused && !audio.ended).length;
    }

    getTotalActiveSfxCount(): number {
        return (Object.keys(this.sfxPools) as SfxName[]).reduce(
            (sum, name) => {
                return sum + this.getActiveSfxCount(name);
            },
            0,
        );
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
        const audio = pool.find((item) => item.paused || item.ended);

        if (!audio) {
            return;
        }

        this.lastPlayedAt[name] = Date.now();
        audio.currentTime = 0;

        audio.play()
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
}

export const audioManager = new AudioManager();