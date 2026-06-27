import { describe, expect, test } from "bun:test";

import { ChannelGraph } from "../src/core/channelGraph";
import type { ChannelDictionary } from "../src/types/api";

function createDenseChannels(): ChannelDictionary {
    const channels: ChannelDictionary = {
        grand_root: {
            id: "grand_root",
            parentId: "",
            children: ["dense"],
            depth: 0,
            islandId: -1,
        },
        dense: {
            id: "dense",
            parentId: "grand_root",
            children: [],
            depth: 1,
            islandId: 0,
        },
    };

    for (let index = 0; index < 60; index += 1) {
        const id = `child-${index}`;
        channels.dense!.children.push(id);
        channels[id] = {
            id,
            parentId: "dense",
            children: [],
            depth: 2,
            islandId: 0,
        };
    }
    return channels;
}

function emphasizedChildIds(graph: ChannelGraph) {
    return graph
        .get("dense")!
        .children.map(index => graph.nodes[index]!)
        .filter(node => node.emphasis === 1)
        .map(node => node.id);
}

describe("ChannelGraph dense child emphasis", () => {
    test("emphasizes a sample and changes it after selection changes", () => {
        const graph = new ChannelGraph(createDenseChannels());

        graph.updateVisibility();
        const initial = emphasizedChildIds(graph);
        expect(initial).toHaveLength(12);

        graph.updateVisibility("dense");
        const expanded = emphasizedChildIds(graph);
        expect(expanded).toHaveLength(12);
        expect(expanded).not.toEqual(initial);

        graph.updateVisibility("child-59");
        const selected = emphasizedChildIds(graph);
        expect(selected).toContain("child-59");
        expect(selected).not.toEqual(expanded);
    });

    test("does not resample when visibility is recomputed without an interaction", () => {
        const graph = new ChannelGraph(createDenseChannels());

        graph.updateVisibility("dense");
        const initial = emphasizedChildIds(graph);
        graph.updateVisibility("dense");

        expect(emphasizedChildIds(graph)).toEqual(initial);
    });
});
