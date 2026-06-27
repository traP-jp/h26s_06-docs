import { describe, expect, test } from "bun:test";

import { type LayoutNode, calculateLayout } from "../src/core/layout";

function createIrregularTree(): LayoutNode[] {
    const nodes: LayoutNode[] = [];
    const add = (parentIndex: number, depth: number, islandId: number) => {
        const index = nodes.length;
        nodes.push({
            index,
            parentIndex,
            children: [],
            depth,
            islandId,
            isLayoutActive: true,
            isExpansionOrigin: false,
            emphasis: 1,
            x: 0,
            y: 0,
            z: 0,
        });
        if (parentIndex >= 0) nodes[parentIndex]?.children.push(index);
        return index;
    };

    const grandRoot = add(-1, 0, -1);
    const wideRoot = add(grandRoot, 1, 0);
    const deepRoot = add(grandRoot, 1, 7);
    const smallRoot = add(grandRoot, 1, 42);
    const crowdedBranch = add(smallRoot, 2, 42);

    for (let index = 0; index < 80; index += 1) {
        const child = add(wideRoot, 2, 0);
        if (index % 9 === 0) add(child, 3, 0);
    }

    let parent = deepRoot;
    for (let depth = 2; depth <= 14; depth += 1) {
        parent = add(parent, depth, 7);
    }

    for (let index = 0; index < 160; index += 1) {
        add(crowdedBranch, 3, 42);
    }
    return nodes;
}

function position(positions: Float32Array, index: number) {
    const offset = index * 3;
    return {
        x: positions[offset] ?? 0,
        y: positions[offset + 1] ?? 0,
        z: positions[offset + 2] ?? 0,
    };
}

function distance(left: ReturnType<typeof position>, right: ReturnType<typeof position>) {
    return Math.hypot(left.x - right.x, left.y - right.y, left.z - right.z);
}

describe("calculateLayout", () => {
    test("separates irregular islands and keeps every coordinate finite", () => {
        const nodes = createIrregularTree();
        const positions = calculateLayout(nodes);

        expect(positions).toHaveLength(nodes.length * 3);
        expect([...positions].every(Number.isFinite)).toBeTrue();
        expect(position(positions, 0)).toEqual({ x: 0, y: 0, z: 0 });

        const roots = [1, 2, 3].map(index => position(positions, index));
        expect(distance(roots[0]!, roots[1]!)).toBeGreaterThan(200);
        expect(distance(roots[0]!, roots[2]!)).toBeGreaterThan(200);
        expect(distance(roots[1]!, roots[2]!)).toBeGreaterThan(200);

        const smallRoot = position(positions, 3);
        const crowdedBranch = position(positions, 4);
        expect(distance(smallRoot, crowdedBranch)).toBeLessThan(90);
    });

    test("is deterministic for the same topology", () => {
        const nodes = createIrregularTree();
        expect(calculateLayout(nodes)).toEqual(calculateLayout(nodes));
    });
});
