import type { AuthUser } from "../types/api";

type JsonObject = Record<string, unknown>;

export async function fetchCurrentUser(): Promise<AuthUser | undefined> {
    const response = await fetch("/api/me", {
        credentials: "include",
        headers: {
            Accept: "application/json",
        },
    });

    if (response.status === 401) return undefined;
    if (!response.ok) {
        throw new Error(`/api/me returned ${response.status}`);
    }

    const payload = (await response.json()) as JsonObject;
    if (payload.authenticated === false) return undefined;

    const user = isJsonObject(payload.user) ? payload.user : payload;
    return normalizeUser(user);
}

export function beginLogin() {
    window.location.assign("/api/auth/login");
}

export function redirectOAuthCallback() {
    if (window.location.pathname !== "/oauth/callback") return false;

    window.location.replace(`/api/auth/callback${window.location.search}`);
    return true;
}

function normalizeUser(payload: JsonObject): AuthUser {
    const id = readString(payload.id) ?? readString(payload.userId) ?? readString(payload.name);
    const displayName =
        readString(payload.displayName) ??
        readString(payload.display_name) ??
        readString(payload.screenName) ??
        readString(payload.screen_name) ??
        readString(payload.name) ??
        "traQ ユーザー";
    const name = readString(payload.name) ?? displayName;
    const iconUrl = readString(payload.iconUrl) ?? readString(payload.icon_url);

    return {
        id,
        name,
        displayName,
        iconUrl,
    };
}

function readString(value: unknown): string | undefined {
    return typeof value === "string" && value.length > 0 ? value : undefined;
}

function isJsonObject(value: unknown): value is JsonObject {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
