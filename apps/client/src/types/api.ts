export interface InitChannel {
    id: string;
    name?: string;
    parentId: string;
    children: string[];
    islandId?: number;
    depth?: number;
}

export type ChannelDictionary = Record<string, InitChannel>;

export type TriggerPayload =
    | { type: "msg"; ch: string }
    | { type: "mov"; usr?: string; from?: string; to: string };

export interface SyncPayload {
    ts: number;
    deltas: Record<string, number>;
}

export type ConnectionState = "connecting" | "open" | "closed";

export interface AuthUser {
    id?: string;
    name: string;
    displayName: string;
    iconUrl?: string;
}
