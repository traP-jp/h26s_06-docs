import axios from "axios";

import { parseViewersPayload } from "./viewersPayload";

import { audioManager } from "../audio/audioManager";
import { http } from "../lib/http";
import { buildEventsPath } from "../lib/paths";
import type {
    ChannelDictionary,
    ConnectionState,
    SyncPayload,
    TriggerPayload,
    ViewersPayload,
} from "../types/api";

interface SSEEvent {
    event: string;
    data: string;
}

interface EventStreamHandlers {
    demo: boolean;
    onState: (state: ConnectionState, message: string) => void;
    onInit: (channels: ChannelDictionary) => void;
    onTrigger: (payload: TriggerPayload) => void;
    onSync: (payload: SyncPayload) => void;
    onViewers: (payload: ViewersPayload) => void;
    onMalformedEvent?: (eventName: string) => void;
    onConnectionError?: () => boolean | Promise<boolean>;
}

export class EventStream {
    private abortController?: AbortController;
    private retryTimer?: ReturnType<typeof setTimeout>;
    private retryCount = 0;
    private stopped = true;

    constructor(private readonly handlers: EventStreamHandlers) {}

    connect() {
        this.stopped = false;
        this.clearConnection();
        void this.open();
    }

    disconnect() {
        this.stopped = true;
        this.clearConnection();
        this.handlers.onState("closed", "切断しました");
    }

    private async open() {
        this.handlers.onState("connecting", "ストリームへ接続中");

        const controller = new AbortController();
        this.abortController = controller;

        try {
            const response = await http.get(buildEventsPath(this.handlers.demo), {
                adapter: "fetch",
                responseType: "stream",
                signal: controller.signal,
                headers: { Accept: "text/event-stream" },
            });

            for await (const sseEvent of readSSEStream(
                response.data as ReadableStream<Uint8Array>
            )) {
                if (this.stopped) break;
                this.dispatch(sseEvent);
            }

            if (!this.stopped) this.scheduleReconnect();
        } catch (error) {
            if (this.stopped) return;
            if (axios.isCancel(error)) return;

            if (await this.handlers.onConnectionError?.()) return;
            if (this.stopped) return;

            this.scheduleReconnect();
        }
    }

    private dispatch({ event, data }: SSEEvent) {
        switch (event) {
            case "init": {
                const payload = tryParseJSON<{ channels: ChannelDictionary }>(data);
                if (payload) {
                    this.retryCount = 0;
                    this.handlers.onInit(payload.channels);
                } else {
                    this.handlers.onMalformedEvent?.("init");
                }
                break;
            }
            case "status": {
                const payload = tryParseJSON<{ status: string }>(data);
                this.handlers.onState("open", payload?.status ?? "接続済み");
                break;
            }
            case "trigger": {
                const payload = tryParseJSON<TriggerPayload>(data);
                if (payload) {
                    this.playTriggerSound(payload);
                    this.handlers.onTrigger(payload);
                } else {
                    this.handlers.onMalformedEvent?.("trigger");
                }
                break;
            }
            case "sync": {
                const payload = tryParseJSON<SyncPayload>(data);
                if (payload) {
                    this.handlers.onSync(payload);
                } else {
                    this.handlers.onMalformedEvent?.("sync");
                }
                break;
            }
            case "viewers": {
                const payload = parseViewersPayload(data);
                if (payload) {
                    this.handlers.onViewers(payload);
                } else {
                    this.handlers.onMalformedEvent?.("viewers");
                }
                break;
            }
            case "stream-error": {
                const payload = tryParseJSON<{ error: string }>(data);
                this.handlers.onState("closed", payload?.error ?? "受信エラー");
                break;
            }
        }
    }

    private playTriggerSound(payload: TriggerPayload) {
        try {
            if (payload.type === "mov") {
                audioManager.playMove();
            }
        } catch (error) {
            console.warn("効果音の再生に失敗しました", error);
        }
    }

    private scheduleReconnect() {
        const delay = Math.min(30_000, 1000 * 2 ** this.retryCount);
        this.retryCount += 1;

        this.handlers.onState("connecting", `${Math.ceil(delay / 1000)}秒後に再接続します`);

        this.retryTimer = setTimeout(() => {
            this.retryTimer = undefined;
            if (!this.stopped) void this.open();
        }, delay);
    }

    private clearConnection() {
        this.abortController?.abort();
        this.abortController = undefined;

        if (this.retryTimer) {
            clearTimeout(this.retryTimer);
            this.retryTimer = undefined;
        }
    }
}

async function* readSSEStream(stream: ReadableStream<Uint8Array>): AsyncGenerator<SSEEvent> {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    try {
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const blocks = buffer.split("\n\n");
            buffer = blocks.pop() ?? "";

            for (const block of blocks) {
                const sseEvent = parseSSEBlock(block);
                if (sseEvent) yield sseEvent;
            }
        }
    } finally {
        reader.releaseLock();
    }
}

function parseSSEBlock(block: string): SSEEvent | null {
    let event = "message";
    const dataLines: string[] = [];

    for (const line of block.split("\n")) {
        if (line.startsWith("event:")) {
            event = line.slice(6).trim();
        } else if (line.startsWith("data:")) {
            dataLines.push(line.slice(5).trim());
        }
    }

    if (dataLines.length === 0) return null;
    return { event, data: dataLines.join("\n") };
}

function tryParseJSON<T>(raw: string): T | undefined {
    try {
        return JSON.parse(raw) as T;
    } catch {
        return undefined;
    }
}
