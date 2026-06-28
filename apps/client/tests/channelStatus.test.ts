import { describe, expect, test } from "bun:test";

import { ChannelStatus } from "../src/services/channelStatus";

function deferred() {
    let resolve: () => void = () => {};
    const promise = new Promise<void>(resolvePromise => {
        resolve = resolvePromise;
    });
    return { promise, resolve };
}

describe("ChannelStatus", () => {
    test("serializes updates and applies only the latest queued channel", async () => {
        const first = deferred();
        const sent: string[] = [];
        const applied: string[] = [];
        const status = new ChannelStatus({
            send: channelId => {
                sent.push(channelId);
                return sent.length === 1 ? first.promise : Promise.resolve();
            },
            onApplied: channelId => applied.push(channelId),
        });

        status.setChannel("channel-a");
        status.setChannel("channel-b");
        status.setChannel("channel-c");
        expect(sent).toEqual(["channel-a"]);

        first.resolve();
        await Promise.resolve();
        await Promise.resolve();

        expect(sent).toEqual(["channel-a", "channel-c"]);
        expect(applied).toEqual(["channel-a", "channel-c"]);
    });

    test("reports a failed update and retries after the selection changes", async () => {
        const sent: string[] = [];
        const errors: string[] = [];
        const status = new ChannelStatus({
            send: channelId => {
                sent.push(channelId);
                return channelId === "channel-a"
                    ? Promise.reject(new Error("failed"))
                    : Promise.resolve();
            },
            onError: channelId => errors.push(channelId),
        });

        status.setChannel("channel-a");
        await Promise.resolve();
        await Promise.resolve();
        status.setChannel("channel-b");
        await Promise.resolve();
        await Promise.resolve();

        expect(sent).toEqual(["channel-a", "channel-b"]);
        expect(errors).toEqual(["channel-a"]);
    });
});
