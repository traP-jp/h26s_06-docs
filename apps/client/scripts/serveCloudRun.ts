/// <reference types="bun" />

import { extname, normalize, resolve, sep } from "node:path";

const distRoot = "/dist";
const port = getPort(process.env.PORT);
const backendProxyEnabled = isTruthy(process.env.GCLOUD_BACKEND_PROXY);
const serverUpstream = backendProxyEnabled
    ? new URL("http://localhost:8080")
    : null;

const contentTypes = new Map([
    [".css", "text/css; charset=utf-8"],
    [".html", "text/html; charset=utf-8"],
    [".ico", "image/x-icon"],
    [".js", "text/javascript; charset=utf-8"],
    [".json", "application/json; charset=utf-8"],
    [".mp3", "audio/mpeg"],
    [".png", "image/png"],
    [".svg", "image/svg+xml"],
    [".wasm", "application/wasm"],
]);

type StaticHeaders = Record<string, string>;

Bun.serve({
    port,
    async fetch(request) {
        const url = new URL(request.url);

        if (url.pathname.startsWith("/api")) {
            if (serverUpstream === null) {
                return new Response("backend proxy disabled", { status: 404 });
            }
            return proxyApiRequest(request, url, serverUpstream);
        }

        if (url.pathname === "/healthz") {
            return new Response(null, { status: 204 });
        }

        return serveStaticFile(url.pathname);
    },
});

console.info(
    serverUpstream === null
        ? `client listening on :${port}; backend proxy disabled`
        : `client listening on :${port}; proxying /api to ${serverUpstream}`,
);

function getPort(value: string | undefined): number {
    const parsedPort = Number.parseInt(value ?? "", 10);

    return Number.isInteger(parsedPort) && parsedPort > 0 ? parsedPort : 5173;
}

function isTruthy(value: string | undefined): boolean {
    const normalizedValue = value?.toLowerCase();

    return (
        normalizedValue === "1" ||
        normalizedValue === "true" ||
        normalizedValue === "yes" ||
        normalizedValue === "on"
    );
}

async function proxyApiRequest(
    request: Request,
    url: URL,
    upstream: URL,
): Promise<Response> {
    const target = new URL(url.pathname + url.search, upstream);
    const headers = new Headers(request.headers);
    headers.delete("host");

    return fetch(target, {
        body: allowsBody(request.method) ? request.body : undefined,
        headers,
        method: request.method,
        redirect: "manual",
    });
}

async function serveStaticFile(pathname: string): Promise<Response> {
    if (pathname.endsWith("/")) {
        return serveIndex();
    }

    const filePath = resolveStaticPath(pathname);
    const file = Bun.file(filePath);

    if (await file.exists()) {
        return new Response(file, {
            headers: fileHeaders(filePath),
        });
    }

    return serveIndex();
}

function serveIndex(): Response {
    return new Response(Bun.file(resolve(distRoot, "index.html")), {
        headers: fileHeaders("index.html"),
    });
}

function resolveStaticPath(pathname: string): string {
    const decodedPathname = decodeURIComponent(pathname);
    const normalizedPathname = normalize(decodedPathname).replace(
        /^(\.\.[/\\])+/,
        "",
    );
    const filePath = resolve(distRoot, `.${normalizedPathname}`);

    if (!filePath.startsWith(`${distRoot}${sep}`) && filePath !== distRoot) {
        return resolve(distRoot, "index.html");
    }

    return filePath;
}

function fileHeaders(filePath: string): StaticHeaders {
    const contentType = contentTypes.get(extname(filePath));

    return contentType === undefined ? {} : { "content-type": contentType };
}

function allowsBody(method: string): boolean {
    return method !== "GET" && method !== "HEAD";
}
