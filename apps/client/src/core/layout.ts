import {
    forceCollide,
    forceLink,
    forceManyBody,
    forceSimulation,
    forceX,
    forceY,
    forceZ,
} from "d3-force-3d";
import type { Force, SimulationLinkDatum, SimulationNodeDatum } from "d3-force-3d";

const POSITION_COMPONENTS = 3;
const GOLDEN_ANGLE = Math.PI * (3 - Math.sqrt(5));
const ROOT_SEPARATION = 55;
const MIN_OUTWARD_DOT = 0.001;
const HEAT_REPULSION_STRENGTH = 34;

export interface LayoutNode {
    index: number;
    parentIndex: number;
    children: number[];
    depth: number;
    islandId: number;
    isLayoutActive: boolean;
    isExpansionOrigin: boolean;
    emphasis: number;
    relativeScore: number;
    x: number;
    y: number;
    z: number;
}

interface ForceNode extends SimulationNodeDatum {
    id: number;
    depth: number;
    islandId: number;
    radius: number;
    emphasis: number;
    heatScore: number;
}

interface ForceLinkDatum extends SimulationLinkDatum<ForceNode> {
    desiredDistance: number;
    strength: number;
}

interface Point {
    x: number;
    y: number;
    z: number;
}

interface ActiveHierarchy {
    childrenByParent: ReadonlyMap<number, readonly number[]>;
    ordinalByChild: ReadonlyMap<number, number>;
}

export function calculateLayout(nodes: LayoutNode[]) {
    const positions = new Float32Array(nodes.length * POSITION_COMPONENTS);
    if (nodes.length === 0) return positions;

    for (const node of nodes) {
        setPosition(positions, node.index, node.x, node.y, node.z);
    }

    const activeNodes = nodes.filter(node => node.isLayoutActive);
    if (activeNodes.length === 0) return positions;

    const activeByIndex = new Map(activeNodes.map(node => [node.index, node]));
    const hierarchy = createActiveHierarchy(activeNodes, activeByIndex);
    const islandIds = [
        ...new Set(activeNodes.map(node => node.islandId).filter(islandId => islandId >= 0)),
    ].sort((left, right) => left - right);
    const islandCenters = createIslandCenters(islandIds);
    const islandRoots = findIslandRoots(activeNodes, activeByIndex);

    seedMissingPositions(
        activeNodes,
        activeByIndex,
        islandCenters,
        islandRoots,
        hierarchy,
        positions
    );
    const heatScores = aggregateHeatScores(activeNodes, hierarchy);

    const forceNodes: ForceNode[] = activeNodes.map(node => {
        const position = readPosition(positions, node.index);
        const emphasisScale = emphasisScaleFor(node.emphasis);
        const radius = (node.depth <= 1 ? 8 : Math.max(4, 7 - node.depth * 0.45)) * emphasisScale;
        const forceNode: ForceNode = {
            id: node.index,
            depth: node.depth,
            islandId: node.islandId,
            radius,
            emphasis: node.emphasis,
            heatScore: heatScores.get(node.index) ?? node.relativeScore,
            x: position.x,
            y: position.y,
            z: position.z,
        };

        const islandRoot = islandRoots.get(node.islandId);
        if (node.index === islandRoot?.index) {
            const center = islandCenters.get(node.islandId);
            if (center) {
                forceNode.fx = center.x;
                forceNode.fy = center.y;
                forceNode.fz = center.z;
            }
        } else if (node.parentIndex < 0) {
            forceNode.fx = 0;
            forceNode.fy = 0;
            forceNode.fz = 0;
        }

        return forceNode;
    });

    const links = createLinks(activeNodes, activeByIndex, islandRoots, hierarchy);
    const simulationTicks = Math.min(120, Math.max(36, Math.floor(80_000 / activeNodes.length)));

    const simulation = forceSimulation(forceNodes, 3)
        .force(
            "link",
            forceLink<ForceNode, ForceLinkDatum>(links)
                .id(node => node.id)
                .distance(link => link.desiredDistance)
                .strength(link => link.strength)
                .iterations(4)
        )
        .force(
            "charge",
            forceManyBody<ForceNode>()
                .strength(node => -18 * node.radius)
                .distanceMax(240)
                .theta(1)
        )
        .force("heat-repulsion", forceHeatRepulsion())
        .force(
            "collide",
            forceCollide<ForceNode>()
                .radius(node => node.radius + 2)
                .strength(0.9)
                .iterations(1)
        )
        .force(
            "island-x",
            forceX<ForceNode>(node => islandCenters.get(node.islandId)?.x ?? 0).strength(node =>
                node.islandId >= 0 ? 0.012 : 0.08
            )
        )
        .force(
            "island-y",
            forceY<ForceNode>(node => islandCenters.get(node.islandId)?.y ?? 0).strength(node =>
                node.islandId >= 0 ? 0.012 : 0.08
            )
        )
        .force(
            "island-z",
            forceZ<ForceNode>(node => islandCenters.get(node.islandId)?.z ?? 0).strength(node =>
                node.islandId >= 0 ? 0.012 : 0.08
            )
        )
        .stop();

    simulation.tick(simulationTicks);
    constrainDeepNodesOutward(forceNodes, activeNodes, activeByIndex);

    for (const node of forceNodes) {
        setPosition(positions, node.id, node.x ?? 0, node.y ?? 0, node.z ?? 0);
    }

    return positions;
}

function createActiveHierarchy(
    activeNodes: LayoutNode[],
    activeByIndex: ReadonlyMap<number, LayoutNode>
): ActiveHierarchy {
    const childrenByParent = new Map<number, readonly number[]>();
    const ordinalByChild = new Map<number, number>();

    for (const node of activeNodes) {
        const children = node.children.filter(childIndex => activeByIndex.has(childIndex));
        childrenByParent.set(node.index, children);
        for (let ordinal = 0; ordinal < children.length; ordinal += 1) {
            const childIndex = children[ordinal];
            if (childIndex !== undefined) ordinalByChild.set(childIndex, ordinal);
        }
    }

    return { childrenByParent, ordinalByChild };
}

function aggregateHeatScores(activeNodes: LayoutNode[], hierarchy: ActiveHierarchy) {
    const heatScores = new Map<number, number>();
    const nodeByIndex = new Map(activeNodes.map(node => [node.index, node]));

    const visit = (node: LayoutNode): number => {
        const cached = heatScores.get(node.index);
        if (cached !== undefined) return cached;

        let heat = node.relativeScore;
        for (const childIndex of hierarchy.childrenByParent.get(node.index) ?? []) {
            const child = nodeByIndex.get(childIndex);
            if (child) heat = Math.max(heat, visit(child));
        }
        heatScores.set(node.index, heat);
        return heat;
    };

    for (const node of activeNodes) visit(node);
    return heatScores;
}

function createIslandCenters(islandIds: number[]) {
    const centers = new Map<number, Point>();
    if (islandIds.length === 0) return centers;

    const radius = Math.max(260, ROOT_SEPARATION * Math.sqrt(islandIds.length));
    for (let ordinal = 0; ordinal < islandIds.length; ordinal += 1) {
        const islandId = islandIds[ordinal];
        if (islandId === undefined) continue;
        const y = 1 - (2 * (ordinal + 0.5)) / islandIds.length;
        const horizontalRadius = Math.sqrt(Math.max(0, 1 - y * y));
        const angle = ordinal * GOLDEN_ANGLE;
        centers.set(islandId, {
            x: Math.cos(angle) * horizontalRadius * radius,
            y: y * radius,
            z: Math.sin(angle) * horizontalRadius * radius,
        });
    }
    return centers;
}

function findIslandRoots(
    activeNodes: LayoutNode[],
    activeByIndex: ReadonlyMap<number, LayoutNode>
) {
    const roots = new Map<number, LayoutNode>();
    for (const node of activeNodes) {
        if (node.islandId < 0) continue;
        const parent = activeByIndex.get(node.parentIndex);
        const current = roots.get(node.islandId);
        const crossesIslandBoundary = !parent || parent.islandId !== node.islandId;
        if (
            crossesIslandBoundary ||
            !current ||
            node.depth < current.depth ||
            (node.depth === current.depth && node.index < current.index)
        ) {
            roots.set(node.islandId, node);
        }
    }
    return roots;
}

function seedMissingPositions(
    activeNodes: LayoutNode[],
    activeByIndex: ReadonlyMap<number, LayoutNode>,
    islandCenters: ReadonlyMap<number, Point>,
    islandRoots: ReadonlyMap<number, LayoutNode>,
    hierarchy: ActiveHierarchy,
    positions: Float32Array
) {
    const seeded = new Set<number>();
    const visiting = new Set<number>();

    const seed = (node: LayoutNode) => {
        if (seeded.has(node.index)) return;
        if (visiting.has(node.index)) {
            setPosition(positions, node.index, 0, 0, 0);
            seeded.add(node.index);
            return;
        }
        visiting.add(node.index);

        const original = readPosition(positions, node.index);
        const parent = activeByIndex.get(node.parentIndex);
        const parentPosition = parent ? readPosition(positions, parent.index) : undefined;
        const overlapsParent =
            parentPosition !== undefined && squaredDistance(original, parentPosition) < 0.01;
        if (
            !parent?.isExpansionOrigin &&
            !overlapsParent &&
            (original.x !== 0 || original.y !== 0 || original.z !== 0)
        ) {
            seeded.add(node.index);
            visiting.delete(node.index);
            return;
        }

        const islandRoot = islandRoots.get(node.islandId);
        const islandCenter = islandCenters.get(node.islandId);
        if (node.index === islandRoot?.index && islandCenter) {
            setPosition(positions, node.index, islandCenter.x, islandCenter.y, islandCenter.z);
        } else {
            if (!parent) {
                setPosition(positions, node.index, 0, 0, 0);
            } else {
                seed(parent);
                const seededParentPosition = readPosition(positions, parent.index);
                const activeChildren = hierarchy.childrenByParent.get(parent.index) ?? [];
                const ordinal = hierarchy.ordinalByChild.get(node.index) ?? 0;
                const direction = outwardDirection(
                    ordinal,
                    activeChildren.length,
                    parent.index,
                    branchAxis(parent, activeByIndex, positions)
                );
                const childCount = hierarchy.childrenByParent.get(node.index)?.length ?? 0;
                const distance =
                    branchDistance(activeChildren.length, childCount) *
                    emphasisScaleFor(node.emphasis);
                setPosition(
                    positions,
                    node.index,
                    seededParentPosition.x + direction.x * distance,
                    seededParentPosition.y + direction.y * distance,
                    seededParentPosition.z + direction.z * distance
                );
            }
        }

        seeded.add(node.index);
        visiting.delete(node.index);
    };

    for (const node of activeNodes) seed(node);
}

function outwardDirection(ordinal: number, count: number, seed: number, axis: Point): Point {
    const safeCount = Math.max(1, count);
    const radialFraction = Math.sqrt((ordinal + 0.5) / safeCount);
    const coneAngle = radialFraction * Math.PI * 0.36;
    const axialScale = Math.cos(coneAngle);
    const radialScale = Math.sin(coneAngle);
    const angle = (ordinal + pseudoRandom(seed) * safeCount) * GOLDEN_ANGLE;

    const reference = Math.abs(axis.y) < 0.9 ? { x: 0, y: 1, z: 0 } : { x: 1, y: 0, z: 0 };
    const tangent = normalize(cross(reference, axis));
    const bitangent = cross(axis, tangent);

    return {
        x:
            axis.x * axialScale +
            (tangent.x * Math.cos(angle) + bitangent.x * Math.sin(angle)) * radialScale,
        y:
            axis.y * axialScale +
            (tangent.y * Math.cos(angle) + bitangent.y * Math.sin(angle)) * radialScale,
        z:
            axis.z * axialScale +
            (tangent.z * Math.cos(angle) + bitangent.z * Math.sin(angle)) * radialScale,
    };
}

function branchAxis(
    parent: LayoutNode,
    activeByIndex: ReadonlyMap<number, LayoutNode>,
    positions: Float32Array
) {
    const parentPosition = readPosition(positions, parent.index);
    const grandParent = activeByIndex.get(parent.parentIndex);
    if (grandParent) {
        const grandParentPosition = readPosition(positions, grandParent.index);
        const incoming = subtract(parentPosition, grandParentPosition);
        if (lengthSquared(incoming) > 0.01) return normalize(incoming);
    }
    if (lengthSquared(parentPosition) > 0.01) return normalize(parentPosition);
    return { x: 1, y: 0, z: 0 };
}

function forceHeatRepulsion(): Force<ForceNode> {
    let nodes: ForceNode[] = [];

    const force = (alpha: number) => {
        for (const node of nodes) {
            if (node.depth <= 0 || node.heatScore <= 0) continue;

            const position = pointFromForce(node);
            const axis =
                lengthSquared(position) > 0.01 ? normalize(position) : fallbackAxis(node.id);
            const strength = alpha * HEAT_REPULSION_STRENGTH * node.heatScore;
            node.vx = (node.vx ?? 0) + axis.x * strength;
            node.vy = (node.vy ?? 0) + axis.y * strength;
            node.vz = (node.vz ?? 0) + axis.z * strength;
        }
    };
    force.initialize = (initializedNodes: ForceNode[]) => {
        nodes = initializedNodes;
    };
    return force;
}

function constrainDeepNodesOutward(
    forceNodes: ForceNode[],
    activeNodes: LayoutNode[],
    activeByIndex: ReadonlyMap<number, LayoutNode>
) {
    const forceByID = new Map(forceNodes.map(node => [node.id, node]));
    const deepNodes = [...activeNodes]
        .filter(node => node.depth >= 3)
        .sort((left, right) => left.depth - right.depth);
    for (const node of deepNodes) {
        const parent = activeByIndex.get(node.parentIndex);
        const grandParent = parent ? activeByIndex.get(parent.parentIndex) : undefined;
        const forceNode = forceByID.get(node.index);
        const forceParent = parent ? forceByID.get(parent.index) : undefined;
        const forceGrandParent = grandParent ? forceByID.get(grandParent.index) : undefined;
        if (!parent || !grandParent || !forceNode || !forceParent || !forceGrandParent) continue;

        const grandParentPosition = pointFromForce(forceGrandParent);
        const parentPosition = pointFromForce(forceParent);
        const nodePosition = pointFromForce(forceNode);
        const displacement = subtract(nodePosition, parentPosition);
        const axis = outwardAxisFromGrandParent(parentPosition, grandParentPosition, displacement);
        const outwardDistance = dot(displacement, axis);
        if (outwardDistance > MIN_OUTWARD_DOT) continue;

        const correction = MIN_OUTWARD_DOT - outwardDistance;
        forceNode.x = nodePosition.x + axis.x * correction;
        forceNode.y = nodePosition.y + axis.y * correction;
        forceNode.z = nodePosition.z + axis.z * correction;
    }
}

function outwardAxisFromGrandParent(
    parentPosition: Point,
    grandParentPosition: Point,
    fallback: Point
) {
    const parentBranch = subtract(parentPosition, grandParentPosition);
    if (lengthSquared(parentBranch) > 0.01) return normalize(parentBranch);
    if (lengthSquared(fallback) > 0.01) return normalize(fallback);
    return { x: 1, y: 0, z: 0 };
}

function pointFromForce(node: ForceNode): Point {
    return { x: node.x ?? 0, y: node.y ?? 0, z: node.z ?? 0 };
}

function createLinks(
    activeNodes: LayoutNode[],
    activeByIndex: ReadonlyMap<number, LayoutNode>,
    islandRoots: ReadonlyMap<number, LayoutNode>,
    hierarchy: ActiveHierarchy
) {
    const links: ForceLinkDatum[] = [];
    for (const node of activeNodes) {
        const activeChildren = hierarchy.childrenByParent.get(node.index) ?? [];
        for (const childIndex of activeChildren) {
            const child = activeByIndex.get(childIndex);
            if (!child || child.index === islandRoots.get(child.islandId)?.index) continue;

            const childCount = hierarchy.childrenByParent.get(child.index)?.length ?? 0;
            links.push({
                source: node.index,
                target: child.index,
                desiredDistance:
                    branchDistance(activeChildren.length, childCount) *
                    emphasisScaleFor(child.emphasis),
                strength: Math.max(
                    0.45,
                    Math.min(0.9, 0.68 - child.depth * 0.035 + Math.log2(childCount + 1) * 0.04)
                ),
            });
        }
    }
    return links;
}

function emphasisScaleFor(emphasis: number) {
    return 0.3 + Math.min(1, Math.max(0, emphasis)) * 0.7;
}

function branchDistance(siblingCount: number, childCount: number) {
    const siblingSpread = Math.min(18, Math.log2(siblingCount + 1) * 3.5);
    const subtreeSpread = Math.min(24, Math.log2(childCount + 1) * 4);
    return 20 + siblingSpread + subtreeSpread;
}

function pseudoRandom(seed: number) {
    const x = Math.sin((seed + 1) * 9999.99) * 10000;
    return x - Math.floor(x);
}

function fallbackAxis(seed: number): Point {
    const angle = pseudoRandom(seed) * Math.PI * 2;
    const z = pseudoRandom(seed + 97) * 2 - 1;
    const horizontalRadius = Math.sqrt(Math.max(0, 1 - z * z));
    return {
        x: Math.cos(angle) * horizontalRadius,
        y: Math.sin(angle) * horizontalRadius,
        z,
    };
}

function subtract(left: Point, right: Point): Point {
    return { x: left.x - right.x, y: left.y - right.y, z: left.z - right.z };
}

function cross(left: Point, right: Point): Point {
    return {
        x: left.y * right.z - left.z * right.y,
        y: left.z * right.x - left.x * right.z,
        z: left.x * right.y - left.y * right.x,
    };
}

function dot(left: Point, right: Point) {
    return left.x * right.x + left.y * right.y + left.z * right.z;
}

function lengthSquared(point: Point) {
    return point.x * point.x + point.y * point.y + point.z * point.z;
}

function normalize(point: Point): Point {
    const length = Math.sqrt(lengthSquared(point));
    if (length < 0.0001) return { x: 1, y: 0, z: 0 };
    return { x: point.x / length, y: point.y / length, z: point.z / length };
}

function squaredDistance(left: Point, right: Point) {
    return lengthSquared(subtract(left, right));
}

function setPosition(positions: Float32Array, index: number, x: number, y: number, z: number) {
    const offset = index * POSITION_COMPONENTS;
    positions[offset] = x;
    positions[offset + 1] = y;
    positions[offset + 2] = z;
}

function readPosition(positions: Float32Array, index: number): Point {
    const offset = index * POSITION_COMPONENTS;
    return {
        x: positions[offset] ?? 0,
        y: positions[offset + 1] ?? 0,
        z: positions[offset + 2] ?? 0,
    };
}
