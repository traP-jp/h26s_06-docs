import {
    forceCollide,
    forceLink,
    forceManyBody,
    forceSimulation,
    forceX,
    forceY,
    forceZ,
} from "d3-force-3d";
import type { SimulationLinkDatum, SimulationNodeDatum } from "d3-force-3d";

const POSITION_COMPONENTS = 3;

export interface LayoutNode {
    index: number;
    parentIndex: number;
    children: number[];
    depth: number;
    islandId: number;
    isLayoutActive: boolean;
    x: number;
    y: number;
    z: number;
}

interface ForceNode extends SimulationNodeDatum {
    id: number;
    depth: number;
    islandId: number;
    radius: number;
}

function pseudoRandom(seed: number) {
    const x = Math.sin(seed * 9999.99) * 10000;
    return x - Math.floor(x);
}

interface ForceLinkDatum extends SimulationLinkDatum<ForceNode> {
    desiredDistance: number;
    strength: number;
}

export function calculateLayout(nodes: LayoutNode[]) {
    const positions = new Float32Array(nodes.length * POSITION_COMPONENTS);
    if (nodes.length === 0) return positions;

    for (const n of nodes) {
        let { x, y, z } = n;
        if (n.isLayoutActive && x === 0 && y === 0 && z === 0 && n.parentIndex >= 0) {
            const p = nodes[n.parentIndex];
            if (p) {
                x = p.x + (Math.random() - 0.5) * 10;
                y = p.y + (Math.random() - 0.5) * 10;
                z = p.z + (Math.random() - 0.5) * 10;
            }
        }
        setPosition(positions, n.index, x, y, z);
    }

    const activeNodes = nodes.filter(n => n.isLayoutActive);
    if (activeNodes.length === 0) return positions;

    const radii = new Float32Array(nodes.length);
    const ordered = [...activeNodes].sort((a, b) => b.depth - a.depth);
    for (const node of ordered) {
        const activeChildren = node.children
            .map(index => nodes[index])
            .filter((c): c is LayoutNode => c !== undefined && c.isLayoutActive);
        if (activeChildren.length === 0) {
            radii[node.index] = 4 + pseudoRandom(node.index) * 6;
        } else {
            let sumArea = 0;
            for (const child of activeChildren) {
                const r = radii[child.index] ?? 12;
                const padding = 1 + pseudoRandom(child.index) * 0.15;
                sumArea += Math.PI * (r * padding) * (r * padding);
            }
            radii[node.index] =
                Math.sqrt(sumArea / Math.PI) * (1.0 + pseudoRandom(node.index) * 0.15) + 4;
        }
    }

    const islandCount = Math.max(1, ...activeNodes.map(n => n.islandId)) + 1;
    const islandCenters = Array.from({ length: islandCount }, (_, index) => {
        const angle = (index / islandCount) * Math.PI * 2;
        return { x: Math.cos(angle) * 300, y: Math.sin(angle) * 300, z: 0 };
    });

    const forceNodes: ForceNode[] = activeNodes.map(node => {
        const p = readPosition(positions, node.index);
        return {
            id: node.index,
            depth: node.depth,
            islandId: node.islandId,
            radius: radii[node.index] ?? 12,
            x: p.x,
            y: p.y,
            z: p.z,
        };
    });

    const links: ForceLinkDatum[] = [];
    for (const node of activeNodes) {
        for (const childIndex of node.children) {
            const childNode = activeNodes.find(n => n.index === childIndex);
            if (childNode) {
                const distanceNoise = 0.65 + pseudoRandom(node.index ^ childIndex) * 0.45;
                const strengthNoise = 0.3 + pseudoRandom((node.index ^ childIndex) + 1) * 0.6;
                links.push({
                    source: node.index,
                    target: childIndex,
                    desiredDistance:
                        ((radii[node.index] ?? 12) + (radii[childIndex] ?? 12)) * distanceNoise,
                    strength: strengthNoise,
                });
            }
        }
    }

    const simulationTicks = Math.min(80, Math.max(12, Math.floor(24000 / activeNodes.length)));

    const simulation = forceSimulation(forceNodes, 3)
        .force(
            "link",
            forceLink<ForceNode, ForceLinkDatum>(links)
                .id(node => node.id)
                .distance(link => link.desiredDistance)
                .strength(link => link.strength)
                .iterations(1)
        )
        .force(
            "charge",
            forceManyBody<ForceNode>()
                .strength(node => -8 * node.radius * (0.7 + pseudoRandom(node.id) * 0.6))
                .distanceMax(600)
                .theta(1)
        )
        .force(
            "collide",
            forceCollide<ForceNode>()
                .radius(node => node.radius * (0.8 + pseudoRandom(node.id) * 0.3))
                .strength(0.8)
                .iterations(1)
        )
        .force(
            "island-x",
            forceX<ForceNode>(node => islandCenters[node.islandId]?.x ?? 0).strength(node =>
                node.depth === 0 ? 0.05 : 0
            )
        )
        .force(
            "island-y",
            forceY<ForceNode>(node => islandCenters[node.islandId]?.y ?? 0).strength(node =>
                node.depth === 0 ? 0.05 : 0
            )
        )
        .force(
            "island-z",
            forceZ<ForceNode>(0).strength(node => (node.depth === 0 ? 0.05 : 0))
        )
        .stop();

    simulation.tick(simulationTicks);

    for (const node of forceNodes) {
        setPosition(positions, node.id, node.x ?? 0, node.y ?? 0, node.z ?? 0);
    }

    return positions;
}

function setPosition(positions: Float32Array, index: number, x: number, y: number, z: number) {
    const offset = index * POSITION_COMPONENTS;
    positions[offset] = x;
    positions[offset + 1] = y;
    positions[offset + 2] = z;
}

function readPosition(positions: Float32Array, index: number) {
    const offset = index * POSITION_COMPONENTS;
    return {
        x: positions[offset] ?? 0,
        y: positions[offset + 1] ?? 0,
        z: positions[offset + 2] ?? 0,
    };
}
