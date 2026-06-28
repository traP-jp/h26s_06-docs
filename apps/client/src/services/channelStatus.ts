import { http } from "../lib/http";
import { buildStatusPath } from "../lib/paths";

type StatusSender = (channelId: string) => Promise<void>;

interface ChannelStatusOptions {
    send?: StatusSender;
    onSending?: (channelId: string) => void;
    onApplied?: (channelId: string) => void;
    onError?: (channelId: string) => void;
}

async function sendChannelStatus(channelId: string): Promise<void> {
    await http.put(buildStatusPath(), { channelId });
}

/**
 * 選択変更を直列化し、遅いリクエストが新しい選択を上書きしないようにする。
 */
export class ChannelStatus {
    private requestedChannelId = "";
    private appliedChannelId: string | undefined;
    private sending = false;
    private readonly send: StatusSender;
    private readonly onSending?: (channelId: string) => void;
    private readonly onApplied?: (channelId: string) => void;
    private readonly onError?: (channelId: string) => void;

    constructor({
        send = sendChannelStatus,
        onSending,
        onApplied,
        onError,
    }: ChannelStatusOptions = {}) {
        this.send = send;
        this.onSending = onSending;
        this.onApplied = onApplied;
        this.onError = onError;
    }

    setChannel(channelId?: string): void {
        this.requestedChannelId = channelId ?? "";
        if (!this.sending && this.appliedChannelId !== this.requestedChannelId) {
            void this.flush();
        }
    }

    private async flush(): Promise<void> {
        this.sending = true;

        while (this.appliedChannelId !== this.requestedChannelId) {
            const channelId = this.requestedChannelId;
            this.onSending?.(channelId);

            try {
                await this.send(channelId);
                this.appliedChannelId = channelId;
                this.onApplied?.(channelId);
            } catch {
                this.onError?.(channelId);
                if (channelId === this.requestedChannelId) break;
            }
        }

        this.sending = false;
    }
}
