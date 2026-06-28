import type { ViewersPayload } from "../types/api";

export function parseViewersPayload(raw: string): ViewersPayload | undefined {
    const payload = tryParseJSON(raw);
    if (!isRecord(payload) || !Array.isArray(payload.viewers)) return undefined;
    if (!payload.viewers.every(viewer => typeof viewer === "string" && viewer.length > 0)) {
        return undefined;
    }
    return { viewers: [...new Set(payload.viewers)] };
}

function tryParseJSON(raw: string): unknown {
    try {
        return JSON.parse(raw) as unknown;
    } catch {
        return undefined;
    }
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}
