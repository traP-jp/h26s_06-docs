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
            relativeScore: 0,
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

function createHeatRepulsionTree(relativeScore: number): LayoutNode[] {
    const nodes = createIrregularTree().slice(0, 2);
    nodes[0]!.children = [1];
    nodes[1]!.children = [2];
    nodes.push({
        index: 2,
        parentIndex: 1,
        children: [],
        depth: 2,
        islandId: 0,
        isLayoutActive: true,
        isExpansionOrigin: false,
        emphasis: 1,
        relativeScore,
        x: 0,
        y: 0,
        z: 0,
    });
    return nodes;
}

function createChildHeatRepulsionTree(relativeScore: number): LayoutNode[] {
    const nodes = createHeatRepulsionTree(0);
    const childIndex = nodes.length;
    nodes[2]!.children = [childIndex];
    nodes.push({
        index: childIndex,
        parentIndex: 2,
        children: [],
        depth: 3,
        islandId: 0,
        isLayoutActive: true,
        isExpansionOrigin: false,
        emphasis: 1,
        relativeScore,
        x: 0,
        y: 0,
        z: 0,
    });
    return nodes;
}

function createFanoutTree(childCount: number): LayoutNode[] {
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
            relativeScore: 0,
            x: 0,
            y: 0,
            z: 0,
        });
        if (parentIndex >= 0) nodes[parentIndex]?.children.push(index);
        return index;
    };

    const grandRoot = add(-1, 0, -1);
    const root = add(grandRoot, 1, 0);
    for (let index = 0; index < childCount; index += 1) add(root, 2, 0);
    return nodes;
}

function createDeepFanoutTree(childCount: number): LayoutNode[] {
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
            relativeScore: 0,
            x: 0,
            y: 0,
            z: 0,
        });
        if (parentIndex >= 0) nodes[parentIndex]?.children.push(index);
        return index;
    };

    const grandRoot = add(-1, 0, -1);
    const root = add(grandRoot, 1, 0);
    const branch = add(root, 2, 0);
    for (let index = 0; index < childCount; index += 1) add(branch, 3, 0);
    return nodes;
}

function createLinearTree(maxDepth: number): LayoutNode[] {
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
            relativeScore: 0,
            x: 0,
            y: 0,
            z: 0,
        });
        if (parentIndex >= 0) nodes[parentIndex]?.children.push(index);
        return index;
    };

    let parent = add(-1, 0, -1);
    for (let depth = 1; depth <= maxDepth; depth += 1) {
        parent = add(parent, depth, 0);
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

function subtract(left: ReturnType<typeof position>, right: ReturnType<typeof position>) {
    return {
        x: left.x - right.x,
        y: left.y - right.y,
        z: left.z - right.z,
    };
}

function dot(left: ReturnType<typeof position>, right: ReturnType<typeof position>) {
    return left.x * right.x + left.y * right.y + left.z * right.z;
}

function normalize(point: ReturnType<typeof position>) {
    const length = Math.hypot(point.x, point.y, point.z);
    if (length < 0.0001) return { x: 1, y: 0, z: 0 };
    return { x: point.x / length, y: point.y / length, z: point.z / length };
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
        expect(distance(smallRoot, crowdedBranch)).toBeLessThan(180);
    });

    test("is deterministic for the same topology", () => {
        const nodes = createIrregularTree();
        expect(calculateLayout(nodes)).toEqual(calculateLayout(nodes));
    });

    test("keeps channels at depth three and deeper outside their parent branch", () => {
        const nodes = createIrregularTree();
        const positions = calculateLayout(nodes);

        for (const node of nodes) {
            if (node.depth < 3) continue;
            const parent = nodes[node.parentIndex];
            const grandParent = parent ? nodes[parent.parentIndex] : undefined;
            if (!parent || !grandParent) continue;

            const grandParentPosition = position(positions, grandParent.index);
            const parentPosition = position(positions, parent.index);
            const nodePosition = position(positions, node.index);
            const outwardAxis = normalize(subtract(parentPosition, grandParentPosition));
            const parentToNode = subtract(nodePosition, parentPosition);

            expect(dot(parentToNode, outwardAxis)).toBeGreaterThan(0);
        }
    });

    test("pushes hotter channels farther from grand root", () => {
        const coolPositions = calculateLayout(createHeatRepulsionTree(0));
        const hotPositions = calculateLayout(createHeatRepulsionTree(1));

        expect(distance(position(hotPositions, 2), position(hotPositions, 0))).toBeGreaterThan(
            distance(position(coolPositions, 2), position(coolPositions, 0))
        );
    });

    test("pushes parents of hot children farther from grand root", () => {
        const coolPositions = calculateLayout(createChildHeatRepulsionTree(0));
        const hotChildPositions = calculateLayout(createChildHeatRepulsionTree(1));

        expect(
            distance(position(hotChildPositions, 2), position(hotChildPositions, 0))
        ).toBeGreaterThan(distance(position(coolPositions, 2), position(coolPositions, 0)));
    });

    test("places first-level channels farther from grand root in all channel mode", () => {
        const nodes = createIrregularTree();
        const collapsedPositions = calculateLayout(nodes);
        const allChannelPositions = calculateLayout(nodes, { displayMode: "all" });

        expect(
            distance(position(allChannelPositions, 1), position(allChannelPositions, 0))
        ).toBeGreaterThan(
            distance(position(collapsedPositions, 1), position(collapsedPositions, 0))
        );
        expect(
            distance(position(allChannelPositions, 1), position(allChannelPositions, 0))
        ).toBeGreaterThan(400);
    });

    test("spreads dense deep all-channel children around their parent", () => {
        const nodes = createDeepFanoutTree(96);
        const positions = calculateLayout(nodes, { displayMode: "all" });
        const grandParent = position(positions, 1);
        const parent = position(positions, 2);
        const outwardAxis = normalize(subtract(parent, grandParent));
        const childOffsets = nodes
            .slice(3)
            .map(node => subtract(position(positions, node.index), parent));
        const childDirections = childOffsets.map(normalize);
        const childDots = childDirections.map(direction => dot(direction, outwardAxis));
        const childDistances = childOffsets.map(offset => Math.sqrt(dot(offset, offset)));

        expect(Math.min(...childDots)).toBeLessThan(-0.25);
        expect(Math.max(...childDots)).toBeGreaterThan(0.25);
        expect(Math.max(...childDistances) - Math.min(...childDistances)).toBeGreaterThan(8);
    });

    test("keeps sparse deep all-channel children in their parent's outward hemisphere", () => {
        const nodes = createDeepFanoutTree(8);
        const positions = calculateLayout(nodes, { displayMode: "all" });
        const grandParent = position(positions, 1);
        const parent = position(positions, 2);
        const outwardAxis = normalize(subtract(parent, grandParent));
        const childOffsets = nodes
            .slice(3)
            .map(node => subtract(position(positions, node.index), parent));
        const axialDistances = childOffsets.map(offset => dot(offset, outwardAxis));
        const lateralDistances = childOffsets.map(offset => {
            const axialDistance = dot(offset, outwardAxis);
            return Math.sqrt(Math.max(0, dot(offset, offset) - axialDistance * axialDistance));
        });

        expect(Math.min(...axialDistances)).toBeGreaterThanOrEqual(-0.001);
        expect(Math.max(...lateralDistances)).toBeGreaterThan(35);
    });

    test("caps dense child repulsion in all channel mode", () => {
        const nodes = createFanoutTree(240);
        const positions = calculateLayout(nodes, { displayMode: "all" });
        const parent = position(positions, 1);
        const childDistances = nodes
            .slice(2)
            .map(node => distance(position(positions, node.index), parent));

        expect(Math.max(...childDistances)).toBeLessThanOrEqual(96.001);
    });

    test("spreads very dense children in every direction in all channel mode", () => {
        const nodes = createFanoutTree(160);
        const positions = calculateLayout(nodes, { displayMode: "all" });
        const grandRoot = position(positions, 0);
        const parent = position(positions, 1);
        const outwardAxis = normalize(subtract(parent, grandRoot));
        const childDots = nodes
            .slice(2)
            .map(node =>
                dot(normalize(subtract(position(positions, node.index), parent)), outwardAxis)
            );

        expect(Math.min(...childDots)).toBeLessThan(-0.25);
        expect(Math.max(...childDots)).toBeGreaterThan(0.25);
    });

    test("keeps deep all-channel chains from collapsing into their parents", () => {
        const nodes = createLinearTree(10);
        const positions = calculateLayout(nodes, { displayMode: "all" });

        for (const node of nodes.filter(node => node.depth >= 3)) {
            const parent = nodes[node.parentIndex];
            expect(parent).toBeDefined();

            expect(
                distance(position(positions, node.index), position(positions, parent!.index))
            ).toBeGreaterThan(35);
        }
    });
});
