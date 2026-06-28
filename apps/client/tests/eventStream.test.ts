import { describe, expect, test } from "bun:test";

import { parseViewersPayload } from "../src/services/viewersPayload";

describe("viewers SSE payload", () => {
    test("parses viewer IDs and removes duplicates", () => {
        expect(parseViewersPayload('{"viewers":["user-b","user-a","user-b"]}')).toEqual({
            viewers: ["user-b", "user-a"],
        });
    });

    test("rejects malformed payloads", () => {
        expect(parseViewersPayload("{")).toBeUndefined();
        expect(parseViewersPayload('{"viewers":"user-a"}')).toBeUndefined();
        expect(parseViewersPayload('{"viewers":["user-a",""]}')).toBeUndefined();
        expect(parseViewersPayload('{"viewers":["user-a",1]}')).toBeUndefined();
    });
});
