import {
    AdditiveBlending,
    BufferGeometry,
    Color,
    DynamicDrawUsage,
    Float32BufferAttribute,
    InstancedMesh,
    Line,
    LineBasicMaterial,
    LineSegments,
    MeshBasicMaterial,
    SphereGeometry,
} from "three";

import { nodeWaverX, nodeWaverY, nodeWaverZ } from "./nodeWaver";

import type { ChannelGraph } from "../core/channelGraph";
import type { NodeBuffer } from "../core/nodeBuffer";

export type NodeMesh = InstancedMesh<SphereGeometry, MeshBasicMaterial>;
export type HierarchyEdges = LineSegments<BufferGeometry, LineBasicMaterial>;
export type SelectionPath = Line<BufferGeometry, LineBasicMaterial>;

export function createNodeMesh(
    graph: ChannelGraph,
    nodeBuffer: NodeBuffer,
    selectedId: string | undefined,
    activeOnly: boolean
) {
    const geometry = new SphereGeometry(1, 10, 8);
    const material = new MeshBasicMaterial({
        color: 0xffffff,
        toneMapped: false,
        transparent: true,
        opacity: 0.94,
        blending: AdditiveBlending,
        depthWrite: false,
    });
    const mesh = new InstancedMesh(geometry, material, graph.nodes.length);
    const instanceColor = new Color();

    mesh.instanceMatrix.setUsage(DynamicDrawUsage);
    nodeBuffer.update(graph.nodes, performance.now(), selectedId, activeOnly);
    writeNodeMeshMatrix(mesh, nodeBuffer);
    for (let index = 0; index < graph.nodes.length; index += 1) {
        const offset = index * 3;
        instanceColor.setRGB(
            nodeBuffer.colorData[offset] ?? 1,
            nodeBuffer.colorData[offset + 1] ?? 1,
            nodeBuffer.colorData[offset + 2] ?? 1
        );
        mesh.setColorAt(index, instanceColor);
    }
    mesh.instanceColor?.setUsage(DynamicDrawUsage);
    if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    mesh.frustumCulled = false;
    return mesh;
}

export function updateNodeMeshMatrix(mesh: NodeMesh, nodeBuffer: NodeBuffer) {
    writeNodeMeshMatrix(mesh, nodeBuffer);
    mesh.instanceMatrix.needsUpdate = true;
}

function writeNodeMeshMatrix(mesh: NodeMesh, nodeBuffer: NodeBuffer) {
    (mesh.instanceMatrix.array as Float32Array).set(nodeBuffer.matrixData);
}

export function createHierarchyEdges(graph: ChannelGraph) {
    const positions: number[] = [];
    const colors: number[] = [];
    const color = new Color();
    for (const node of graph.nodes) {
        if (!node.parentId) continue;
        const parent = graph.get(node.parentId);
        if (!parent) continue;
        positions.push(parent.x, parent.y, parent.z, node.x, node.y, node.z);
        color.set(parent.color);
        colors.push(color.r, color.g, color.b);
        color.set(node.color);
        colors.push(color.r, color.g, color.b);
    }
    const geometry = new BufferGeometry();
    geometry.setAttribute("position", new Float32BufferAttribute(positions, 3));
    geometry.setAttribute("color", new Float32BufferAttribute(colors, 3));
    geometry.userData.baseColors = new Float32Array(colors);
    const lines = new LineSegments(
        geometry,
        new LineBasicMaterial({
            vertexColors: true,
            transparent: true,
            opacity: 0.24,
            blending: AdditiveBlending,
            depthWrite: false,
        })
    );
    lines.frustumCulled = false;
    return lines;
}

export function updateHierarchyEdges(edges: HierarchyEdges, graph: ChannelGraph, now: number) {
    const positionAttribute = edges.geometry.getAttribute("position") as Float32BufferAttribute;
    const colorAttribute = edges.geometry.getAttribute("color") as Float32BufferAttribute;
    const positionArray = positionAttribute.array as Float32Array;
    const colorArray = colorAttribute.array as Float32Array;
    const baseColors = edges.geometry.userData.baseColors as Float32Array;

    let offset = 0;
    let colorOffset = 0;

    for (const node of graph.nodes) {
        if (!node.parentId) continue;
        const parent = graph.get(node.parentId);
        if (!parent) continue;

        const alpha =
            Math.min(parent.visibilityAlpha, node.visibilityAlpha) *
            Math.min(parent.emphasis, node.emphasis);

        positionArray[offset++] = parent.x + nodeWaverX(now, parent.index);
        positionArray[offset++] = parent.y + nodeWaverY(now, parent.index);
        positionArray[offset++] = parent.z + nodeWaverZ(now, parent.index);
        positionArray[offset++] = node.x + nodeWaverX(now, node.index);
        positionArray[offset++] = node.y + nodeWaverY(now, node.index);
        positionArray[offset++] = node.z + nodeWaverZ(now, node.index);

        colorArray[colorOffset] = baseColors[colorOffset]! * alpha;
        colorOffset++;
        colorArray[colorOffset] = baseColors[colorOffset]! * alpha;
        colorOffset++;
        colorArray[colorOffset] = baseColors[colorOffset]! * alpha;
        colorOffset++;

        colorArray[colorOffset] = baseColors[colorOffset]! * alpha;
        colorOffset++;
        colorArray[colorOffset] = baseColors[colorOffset]! * alpha;
        colorOffset++;
        colorArray[colorOffset] = baseColors[colorOffset]! * alpha;
        colorOffset++;
    }
    positionAttribute.needsUpdate = true;
    colorAttribute.needsUpdate = true;
}

export function createSelectionPath() {
    const geometry = new BufferGeometry();
    geometry.setAttribute("position", new Float32BufferAttribute([], 3));
    const line = new Line(
        geometry,
        new LineBasicMaterial({
            color: 0xffffff,
            transparent: true,
            opacity: 0.82,
            blending: AdditiveBlending,
            depthWrite: false,
            toneMapped: false,
        })
    );
    line.visible = false;
    line.frustumCulled = false;
    return line;
}

export function setSelectionPath(
    pathLine: SelectionPath,
    graph: ChannelGraph,
    id: string | undefined
) {
    if (!id) {
        pathLine.visible = false;
        return;
    }
    const path = graph.path(id);
    if (path.length < 2) {
        pathLine.visible = false;
        return;
    }
    const positions = path.flatMap(node => [node.x, node.y, node.z]);
    pathLine.geometry.setAttribute("position", new Float32BufferAttribute(positions, 3));
    pathLine.geometry.computeBoundingSphere();
    pathLine.material.color.set(path.at(-1)?.color ?? "#ffffff");
    pathLine.visible = true;
}

export function updateSelectionPathPositions(
    pathLine: SelectionPath,
    graph: ChannelGraph,
    selectedId: string | undefined,
    now: number
) {
    if (!pathLine.visible || !selectedId) return;
    const path = graph.path(selectedId);
    if (path.length < 2) return;

    const positionAttribute = pathLine.geometry.getAttribute("position") as Float32BufferAttribute;
    const positionArray = positionAttribute.array as Float32Array;
    let offset = 0;

    for (const node of path) {
        positionArray[offset++] = node.x + nodeWaverX(now, node.index);
        positionArray[offset++] = node.y + nodeWaverY(now, node.index);
        positionArray[offset++] = node.z + nodeWaverZ(now, node.index);
    }
    positionAttribute.needsUpdate = true;
}
