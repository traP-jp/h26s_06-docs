import { afterEach, describe, expect, test } from "bun:test";

import { useAppState } from "../src/composables/useAppState";

describe("useAppState event notifications", () => {
    const states: ReturnType<typeof useAppState>[] = [];

    afterEach(() => {
        for (const state of states) state.clearEventToasts();
        states.length = 0;
    });

    test("prefixes channel names and retains the channel target", () => {
        const state = useAppState();
        states.push(state);

        state.recordTrigger({ type: "msg", ch: "general" });

        expect(state.eventToasts.value).toHaveLength(1);
        expect(state.eventToasts.value[0]).toMatchObject({
            channelId: "general",
            detail: "#general にメッセージ",
        });
    });

    test("prefixes channel names in movement notifications", () => {
        const state = useAppState();
        states.push(state);

        state.recordTrigger({ type: "mov", to: "random" });

        expect(state.eventToasts.value[0]).toMatchObject({
            channelId: "random",
            detail: "#random へユーザーが移動",
        });
    });
});
