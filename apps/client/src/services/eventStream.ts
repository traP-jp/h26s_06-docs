import type { ChannelDictionary, ConnectionState, SyncPayload, TriggerPayload } from "../types/api";

interface EventStreamHandlers {
    demo: boolean;
    onState: (state: ConnectionState, message: string) => void;
    onInit: (channels: ChannelDictionary) => void;
    onTrigger: (payload: TriggerPayload) => void;
    onSync: (payload: SyncPayload) => void;
    onMalformedEvent?: (eventName: string) => void;
    onConnectionError?: () => boolean | Promise<boolean>;
}

export class EventStream {
    private source?: EventSource;
    private retryTimer?: ReturnType<typeof setTimeout>;
    private retryCount = 0;
    private stopped = true;

    constructor(private readonly handlers: EventStreamHandlers) {}

    connect() {
        this.stopped = false;
        this.clearConnection();
        this.open();
    }

    disconnect() {
        this.stopped = true;
        this.clearConnection();
        this.handlers.onState("closed", "切断しました");
    }

    private open() {
        this.handlers.onState("connecting", "ストリームへ接続中");
        const query = this.handlers.demo ? "?demo=1" : "";
        const source = new EventSource(`/api/events${query}`, {
            withCredentials: true,
        });
        this.source = source;

        source.addEventListener("init", event => {
            const payload = parseEvent<{ channels: ChannelDictionary }>(event);
            if (payload) {
                this.retryCount = 0;
                this.handlers.onInit(payload.channels);
            } else {
                this.handlers.onMalformedEvent?.("init");
            }
        });
        source.addEventListener("status", event => {
            const payload = parseEvent<{ status: string }>(event);
            this.handlers.onState("open", payload?.status ?? "接続済み");
        });
        source.addEventListener("trigger", event => {
            const payload = parseEvent<TriggerPayload>(event);
            if (payload) this.handlers.onTrigger(payload);
            else this.handlers.onMalformedEvent?.("trigger");
        });
        source.addEventListener("sync", event => {
            const payload = parseEvent<SyncPayload>(event);
            if (payload) this.handlers.onSync(payload);
            else this.handlers.onMalformedEvent?.("sync");
        });
        source.addEventListener("stream-error", event => {
            const payload = parseEvent<{ error: string }>(event);
            this.handlers.onState("closed", payload?.error ?? "受信エラー");
        });
        source.onerror = async () => {
            if (this.stopped || source !== this.source) return;
            source.close();
            this.source = undefined;
            if (await this.handlers.onConnectionError?.()) return;
            if (this.stopped || this.source) return;
            this.scheduleReconnect();
        };
    }

    private scheduleReconnect() {
        const delay = Math.min(30_000, 1000 * 2 ** this.retryCount);
        this.retryCount += 1;
        this.handlers.onState("connecting", `${Math.ceil(delay / 1000)}秒後に再接続します`);
        this.retryTimer = setTimeout(() => {
            this.retryTimer = undefined;
            if (!this.stopped) this.open();
        }, delay);
    }

    private clearConnection() {
        this.source?.close();
        this.source = undefined;
        if (this.retryTimer) clearTimeout(this.retryTimer);
        this.retryTimer = undefined;
    }
}

function parseEvent<T>(event: Event): T | undefined {
    if (!(event instanceof MessageEvent) || typeof event.data !== "string") return undefined;
    try {
        return JSON.parse(event.data) as T;
    } catch {
        return undefined;
    }
}
