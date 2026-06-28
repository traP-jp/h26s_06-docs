import { afterEach, describe, expect, mock, spyOn, test } from "bun:test";

import { ChannelGraph } from "../src/core/channelGraph";
import type { ChannelDictionary } from "../src/types/api";

afterEach(() => {
    mock.restore();
});

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

function createDeepChannels(): ChannelDictionary {
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
            children: ["branch"],
            depth: 1,
            islandId: 0,
        },
        branch: {
            id: "branch",
            parentId: "root",
            children: ["leaf"],
            depth: 2,
            islandId: 0,
        },
        leaf: {
            id: "leaf",
            parentId: "branch",
            children: ["nested"],
            depth: 3,
            islandId: 0,
        },
        nested: {
            id: "nested",
            parentId: "leaf",
            children: [],
            depth: 4,
            islandId: 0,
        },
    };
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

describe("ChannelGraph active visibility", () => {
    test("uses init scores before the first frame update", () => {
        const channels = createDeepChannels();
        channels.leaf!.score = 3.4;
        const graph = new ChannelGraph(channels);

        graph.updateVisibility(undefined, 0);

        expect(graph.get("leaf")!.currentScore).toBe(3.4);
        expect(graph.get("leaf")!.relativeScore).toBe(1);
        expect(graph.get("leaf")!.isLayoutActive).toBe(true);
        expect(graph.get("branch")!.isLayoutActive).toBe(true);
    });

    test("activates hot channels and their ancestors at any depth", () => {
        const graph = new ChannelGraph(createDeepChannels());
        graph.get("leaf")!.relativeScore = 0.09;

        graph.updateVisibility(undefined, 0);

        expect(graph.get("leaf")!.isLayoutActive).toBe(true);
        expect(graph.get("branch")!.isLayoutActive).toBe(true);
        expect(graph.get("root")!.isLayoutActive).toBe(true);
        expect(graph.get("grand_root")!.isLayoutActive).toBe(true);
    });

    test("propagates active descendant scores to inactive parents", () => {
        const graph = new ChannelGraph(createDeepChannels());
        graph.get("leaf")!.relativeScore = 0.24;
        graph.get("branch")!.relativeScore = 0;

        graph.updateVisibility(undefined, 0);

        expect(graph.get("leaf")!.activeDescendantScore).toBe(0.24);
        expect(graph.get("branch")!.activeDescendantScore).toBe(0.24);
        expect(graph.get("root")!.activeDescendantScore).toBe(0.24);
    });

    test("reveals a hidden message path one reached node at a time", () => {
        const graph = new ChannelGraph(createDeepChannels());
        graph.updateVisibility();

        expect(graph.get("leaf")!.isLayoutActive).toBe(false);
        expect(graph.get("nested")!.isLayoutActive).toBe(false);

        expect(graph.applyTrigger({ type: "msg", ch: "nested" })).toBe(true);
        graph.updateVisibility();
        graph.update(1);

        expect(graph.takeVisualEvents()).toEqual([{ type: "message", channelId: "nested" }]);
        expect(graph.get("leaf")!.isLayoutActive).toBe(true);
        expect(graph.get("nested")!.isLayoutActive).toBe(true);
        expect(graph.get("leaf")!.visibilityAlpha).toBe(0);
        expect(graph.get("nested")!.visibilityAlpha).toBe(0);

        graph.revealMessageNode("leaf");
        graph.update(1);

        expect(graph.get("leaf")!.visibilityAlpha).toBeGreaterThan(0);
        expect(graph.get("nested")!.visibilityAlpha).toBe(0);

        graph.revealMessageNode("nested");
        graph.update(1);

        expect(graph.get("nested")!.visibilityAlpha).toBeGreaterThan(0);
    });
});

describe("ChannelGraph score sync", () => {
    test("keeps trigger prediction aligned with the server score calculation", () => {
        const channels = createDeepChannels();
        channels.leaf!.score = 1.25;
        const graph = new ChannelGraph(channels);

        graph.applyTrigger({ type: "msg", ch: "leaf", delta: 1.0 });

        expect(graph.get("leaf")!.currentScore).toBe(2.25);
        expect(graph.get("leaf")!.targetScore).toBe(2.25);
        expect(graph.get("branch")!.currentScore).toBe(0.45);
        expect(graph.get("branch")!.targetScore).toBe(0.45);

        graph.update(10);
        const decay = Math.exp(-10 / 300);
        expect(graph.get("leaf")!.currentScore).toBeCloseTo(2.25 * decay);
        expect(graph.get("leaf")!.targetScore).toBeCloseTo(2.25 * decay);
    });

    test("logs client values, true values, and their differences before syncing", () => {
        const channels = createDeepChannels();
        channels.leaf!.score = 1.25;
        const graph = new ChannelGraph(channels);
        const table = spyOn(console, "table").mockImplementation(() => {});

        graph.sync({ leaf: 2, unknown: 10 });

        expect(table).toHaveBeenCalledWith([
            {
                channelId: "leaf",
                clientValue: 1.25,
                trueValue: 2,
                difference: 0.75,
            },
        ]);
        expect(graph.get("leaf")!.targetScore).toBe(2);
    });
});

describe("ChannelGraph navigation targets", () => {
    test("uses parent, first child, and cyclic alphabetic sibling order", () => {
        const graph = new ChannelGraph(createDenseChannels());

        expect(graph.navigationTargets("child-0")).toEqual({
            parentId: "dense",
            childId: undefined,
            previousSiblingId: "child-59",
            nextSiblingId: "child-1",
        });
        expect(graph.navigationTargets("child-1")).toEqual({
            parentId: "dense",
            childId: undefined,
            previousSiblingId: "child-0",
            nextSiblingId: "child-2",
        });
        expect(graph.navigationTargets("child-59")).toEqual({
            parentId: "dense",
            childId: undefined,
            previousSiblingId: "child-58",
            nextSiblingId: "child-0",
        });
        expect(graph.navigationTargets("dense")).toEqual({
            parentId: "grand_root",
            childId: "child-0",
            previousSiblingId: undefined,
            nextSiblingId: undefined,
        });
        expect(graph.navigationTargets("dense", "child-42")).toEqual({
            parentId: "grand_root",
            childId: "child-42",
            previousSiblingId: undefined,
            nextSiblingId: undefined,
        });
    });
});
