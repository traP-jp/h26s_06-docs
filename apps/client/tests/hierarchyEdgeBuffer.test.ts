import { describe, expect, test } from "bun:test";

import { ChannelGraph } from "../src/core/channelGraph";
import { HierarchyEdgeBuffer } from "../src/rendering/hierarchyEdgeBuffer";
import type { ChannelDictionary } from "../src/types/api";

function createChannels(): ChannelDictionary {
    return {
        grand_root: {
            id: "grand_root",
            parentId: "",
            children: ["root"],
            depth: 0,
            islandId: -1,
        },
        root: {
            id: "root",
            parentId: "grand_root",
            children: ["branch", "inactive"],
            depth: 1,
            islandId: 0,
        },
        branch: {
            id: "branch",
            parentId: "root",
            children: ["hot", "warm"],
            depth: 2,
            islandId: 0,
        },
        hot: {
            id: "hot",
            parentId: "branch",
            children: [],
            depth: 3,
            islandId: 0,
        },
        warm: {
            id: "warm",
            parentId: "branch",
            children: [],
            depth: 3,
            islandId: 0,
        },
        inactive: {
            id: "inactive",
            parentId: "root",
            children: [],
            depth: 2,
            islandId: 0,
        },
    };
}

function edgeIds(graph: ChannelGraph, buffer: HierarchyEdgeBuffer) {
    return Array.from(buffer.nodeIndices.subarray(0, buffer.count), index => {
        return graph.nodes[index]!.id;
    });
}

describe("HierarchyEdgeBuffer", () => {
    test("includes only active channels' paths to grand root", () => {
        const graph = new ChannelGraph(createChannels());
        graph.get("hot")!.relativeScore = 0.8;
        graph.get("warm")!.relativeScore = 0.4;
        const buffer = new HierarchyEdgeBuffer(graph.nodes);

        buffer.update(graph.nodes, true);

        expect(edgeIds(graph, buffer)).toEqual(["root", "branch", "hot", "warm"]);
        const activity = Array.from(buffer.activity.subarray(0, buffer.count));
        expect(activity[0]).toBeCloseTo(0.8);
        expect(activity[1]).toBeCloseTo(0.8);
        expect(activity[2]).toBeCloseTo(0.8);
        expect(activity[3]).toBeCloseTo(0.4);
    });

    test("includes every hierarchy edge in ALL mode", () => {
        const graph = new ChannelGraph(createChannels());
        const buffer = new HierarchyEdgeBuffer(graph.nodes);

        buffer.update(graph.nodes, false);

        expect(edgeIds(graph, buffer)).toEqual(["root", "branch", "hot", "warm", "inactive"]);
        expect(Array.from(buffer.activity.subarray(0, buffer.count))).toEqual([1, 1, 1, 1, 1]);
    });

    test("reuses its typed arrays while activity changes", () => {
        const graph = new ChannelGraph(createChannels());
        const buffer = new HierarchyEdgeBuffer(graph.nodes);
        const nodeIndices = buffer.nodeIndices;
        const activity = buffer.activity;

        graph.get("hot")!.relativeScore = 1;
        buffer.update(graph.nodes, true);
        graph.get("hot")!.relativeScore = 0;
        graph.get("inactive")!.relativeScore = 0.5;
        buffer.update(graph.nodes, true);

        expect(buffer.nodeIndices).toBe(nodeIndices);
        expect(buffer.activity).toBe(activity);
        expect(edgeIds(graph, buffer)).toEqual(["root", "inactive"]);
    });
});
