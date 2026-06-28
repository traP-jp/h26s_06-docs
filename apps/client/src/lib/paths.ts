export function buildMePath() {
    return "/api/me";
}

export function buildAuthLoginPath() {
    return "/api/auth/login";
}

export function buildAuthCallbackPath(search: string) {
    return `/api/auth/callback${search}`;
}

export function buildEventsPath(demo: boolean) {
    return demo ? "/api/events?demo=1" : "/api/events";
}

export function buildStatusPath() {
    return "/api/status";
}
