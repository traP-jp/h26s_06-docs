import axios from "axios";

import { http } from "../lib/http";
import { buildAuthCallbackPath, buildAuthLoginPath, buildMePath } from "../lib/paths";
import type { AuthUser } from "../types/api";

type JsonObject = Record<string, unknown>;

export async function fetchCurrentUser(): Promise<AuthUser | undefined> {
    try {
        const { data } = await http.get<JsonObject>(buildMePath());

        if (data.authenticated === false) return undefined;

        const user = isJsonObject(data.user) ? data.user : data;
        return normalizeUser(user);
    } catch (error) {
        if (axios.isAxiosError(error) && error.response?.status === 401) return undefined;
        throw error;
    }
}

export function beginLogin() {
    window.location.assign(buildAuthLoginPath());
}

export function redirectOAuthCallback() {
    if (window.location.pathname !== "/oauth/callback") return false;

    window.location.replace(buildAuthCallbackPath(window.location.search));
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
