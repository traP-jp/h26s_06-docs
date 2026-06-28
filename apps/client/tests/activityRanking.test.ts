import { describe, expect, test } from "bun:test";

import type { ChannelNode } from "../src/core/channelGraph";
import { rankActivityChannels } from "../src/services/activityRanking";

function createNode(
    id: string,
    relativeScore: number,
    currentScore = relativeScore,
    name = id
): ChannelNode {
    return {
        index: 0,
        id,
        name,
        parentId: null,
        children: [],
        islandId: 0,
        depth: 1,
        currentScore,
        targetScore: currentScore,
        relativeScore,
        x: 0,
        y: 0,
        z: 0,
        targetX: 0,
        targetY: 0,
        targetZ: 0,
        visibilityAlpha: 1,
        isLayoutActive: true,
        isExpansionOrigin: false,
        emphasis: 1,
        activeDescendantScore: 0,
        color: "#00bfff",
    };
}

describe("rankActivityChannels", () => {
    test("returns at most five active channels in descending heat order", () => {
        const nodes = [
            createNode("grand_root", 1),
            createNode("sixth", 0.1),
            createNode("second", 0.8),
            createNode("fifth", 0.2),
            createNode("first", 1.2),
            createNode("inactive", 0),
            createNode("fourth", 0.4),
            createNode("third", 0.6),
        ];

        expect(rankActivityChannels(nodes)).toEqual([
            { id: "first", name: "first", heat: 100, color: "#00bfff" },
            { id: "second", name: "second", heat: 80, color: "#00bfff" },
            { id: "third", name: "third", heat: 60, color: "#00bfff" },
            { id: "fourth", name: "fourth", heat: 40, color: "#00bfff" },
            { id: "fifth", name: "fifth", heat: 20, color: "#00bfff" },
        ]);
    });

    test("uses score and channel name as stable tie breakers", () => {
        const nodes = [
            createNode("beta", 0.5, 2, "beta"),
            createNode("alpha-2", 0.5, 1, "alpha-2"),
            createNode("alpha-1", 0.5, 1, "alpha-1"),
        ];

        expect(rankActivityChannels(nodes, 3).map(channel => channel.id)).toEqual([
            "beta",
            "alpha-1",
            "alpha-2",
        ]);
    });

    test("returns no channels when the limit is not positive", () => {
        expect(rankActivityChannels([createNode("general", 1)], 0)).toEqual([]);
    });
});
