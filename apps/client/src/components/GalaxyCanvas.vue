<script setup lang="ts">
import { onBeforeUnmount, onMounted, useTemplateRef, watch } from "vue";

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
    PerspectiveCamera,
    SRGBColorSpace,
    Scene,
    SphereGeometry,
    Vector2,
    Vector3,
    WebGLRenderer,
} from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";

import type { ChannelGraph } from "../core/channelGraph";
import { NodeBuffer } from "../core/nodeBuffer";
import { CameraController } from "../rendering/cameraController";
import { EffectPool } from "../rendering/effectPool";

const props = defineProps<{
    graph: ChannelGraph;
    selectedId?: string;
    focusId?: string;
    focusRevision: number;
    activeOnly: boolean;
}>();
const emit = defineEmits<{
    select: [id: string | undefined];
    messageNodeReached: [id: string];
    renderError: [message: string];
}>();
const host = useTemplateRef<HTMLDivElement>("host");
const selectionPathOpacity = 0.82;
const rootEdgeColor = "#7b8798";
const rootEdgeVisualOpacity = 0.26;

let renderer: WebGLRenderer | undefined;
let scene: Scene | undefined;
let camera: PerspectiveCamera | undefined;
let controls: OrbitControls | undefined;
let cameraController: CameraController | undefined;
let composer: EffectComposer | undefined;
let effects: EffectPool | undefined;
let nodes: InstancedMesh | undefined;
let hierarchyEdges: LineSegments<BufferGeometry, LineBasicMaterial> | undefined;
let selectionPath: Line<BufferGeometry, LineBasicMaterial> | undefined;
let frame = 0;
let lastFrame = performance.now();
let pointerDown = new Vector2();
let pointerLast = new Vector2();
let pointerMoved = false;
let pointerButton = -1;
let customRotationActive = false;
let hoverPending = false;
let resizeObserver: ResizeObserver | undefined;
let nodeBuffer = new NodeBuffer(props.graph.nodes.length);
const projectedNode = new Vector3();
const hoverPointer = new Vector2();
const instanceColor = new Color();
const NODE_PICK_RADIUS = 28;

function initialise() {
    const element = host.value;
    if (!element) return;
    try {
        scene = new Scene();
        camera = new PerspectiveCamera(52, 1, 0.1, 3000);
        const graphExtent = Math.max(
            360,
            ...props.graph.nodes.map(node => Math.hypot(node.x, node.y, node.z))
        );
        const initialDistance = Math.max(620, graphExtent * 1.65);
        camera.far = Math.max(3000, initialDistance * 4);
        camera.position.set(0, 0, initialDistance);
        camera.updateProjectionMatrix();

        renderer = new WebGLRenderer({
            antialias: true,
            alpha: true,
            powerPreference: "high-performance",
        });
        renderer.setPixelRatio(Math.min(devicePixelRatio, 1.5));
        renderer.setClearColor(0x000000, 0);
        renderer.outputColorSpace = SRGBColorSpace;
        element.append(renderer.domElement);

        controls = new OrbitControls(camera, renderer.domElement);
        controls.enableDamping = true;
        controls.dampingFactor = 0.06;
        controls.enableRotate = false;
        controls.minDistance = 80;
        controls.maxDistance = Math.max(1400, initialDistance * 2.5);
        controls.enablePan = true;
        cameraController = new CameraController(camera, controls);

        nodes = createNodeMesh();
        scene.add(nodes);
        hierarchyEdges = createEdges();
        scene.add(hierarchyEdges);
        selectionPath = createSelectionPath();
        scene.add(selectionPath);
        effects = new EffectPool(scene, props.graph, id => emit("messageNodeReached", id));
        composer = new EffectComposer(renderer);
        composer.addPass(new RenderPass(scene, camera));
        composer.addPass(
            new UnrealBloomPass(
                new Vector2(element.clientWidth, element.clientHeight),
                1.35,
                0.62,
                0.08
            )
        );
        renderer.compile(scene, camera);

        renderer.domElement.addEventListener("pointerdown", onPointerDown);
        renderer.domElement.addEventListener("pointermove", onPointerMove);
        renderer.domElement.addEventListener("pointerup", onPointerUp);
        renderer.domElement.addEventListener("pointerleave", onPointerLeave);
        renderer.domElement.addEventListener("webglcontextlost", onContextLost);
        resizeObserver = new ResizeObserver(resize);
        resizeObserver.observe(element);
        document.addEventListener("visibilitychange", resetFrameClock);
        resize();
        updateSelectionPath(props.selectedId);
        if (props.focusId) updateCameraTarget(props.focusId);
        else centerInitialCamera();
        frame = requestAnimationFrame(draw);
    } catch {
        emit("renderError", "WebGLを初期化できませんでした");
    }
}

function createNodeMesh() {
    const geometry = new SphereGeometry(1, 10, 8);
    const material = new MeshBasicMaterial({
        color: 0xffffff,
        toneMapped: false,
        transparent: true,
        opacity: 0.94,
        blending: AdditiveBlending,
        depthWrite: false,
    });
    const mesh = new InstancedMesh(geometry, material, props.graph.nodes.length);
    mesh.instanceMatrix.setUsage(DynamicDrawUsage);
    nodeBuffer.update(props.graph.nodes, performance.now(), props.selectedId, props.activeOnly);
    (mesh.instanceMatrix.array as Float32Array).set(nodeBuffer.matrixData);
    for (let index = 0; index < props.graph.nodes.length; index += 1) {
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

function createEdges() {
    const positions: number[] = [];
    const colors: number[] = [];
    const color = new Color();
    for (const node of props.graph.nodes) {
        if (!hasHierarchyEdge(node)) continue;
        const parent = props.graph.get(node.parentId);
        if (!parent) continue;
        positions.push(parent.x, parent.y, parent.z, node.x, node.y, node.z);
        color.set(isGrandRootEdge(node) ? "#7b8798" : parent.color);
        colors.push(color.r, color.g, color.b);
        color.set(isGrandRootEdge(node) ? "#7b8798" : node.color);
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

function updateEdges(now: number) {
    if (!hierarchyEdges) return;
    const positionAttribute = hierarchyEdges.geometry.getAttribute(
        "position"
    ) as Float32BufferAttribute;
    const colorAttribute = hierarchyEdges.geometry.getAttribute("color") as Float32BufferAttribute;
    const posArray = positionAttribute.array as Float32Array;
    const colArray = colorAttribute.array as Float32Array;
    const baseColors = hierarchyEdges.geometry.userData.baseColors as Float32Array;

    let offset = 0;
    let colOffset = 0;

    for (const node of props.graph.nodes) {
        if (!hasHierarchyEdge(node)) continue;
        const parent = props.graph.get(node.parentId);
        if (!parent) continue;

        const alphaScale = isGrandRootEdge(node) ? 0.26 : 1;
        const alpha =
            Math.min(parent.visibilityAlpha, node.visibilityAlpha) *
            Math.min(parent.emphasis, node.emphasis) *
            alphaScale;

        const wxParent = Math.sin(now * 0.0008 + parent.index * 1.2) * 1.5;
        const wyParent = Math.cos(now * 0.0009 + parent.index * 0.8) * 1.5;
        const wzParent = Math.sin(now * 0.0007 + parent.index * 1.5) * 1.5;

        const wxNode = Math.sin(now * 0.0008 + node.index * 1.2) * 1.5;
        const wyNode = Math.cos(now * 0.0009 + node.index * 0.8) * 1.5;
        const wzNode = Math.sin(now * 0.0007 + node.index * 1.5) * 1.5;

        posArray[offset++] = parent.x + wxParent;
        posArray[offset++] = parent.y + wyParent;
        posArray[offset++] = parent.z + wzParent;
        posArray[offset++] = node.x + wxNode;
        posArray[offset++] = node.y + wyNode;
        posArray[offset++] = node.z + wzNode;

        colArray[colOffset] = baseColors[colOffset]! * alpha;
        colOffset++;
        colArray[colOffset] = baseColors[colOffset]! * alpha;
        colOffset++;
        colArray[colOffset] = baseColors[colOffset]! * alpha;
        colOffset++;

        colArray[colOffset] = baseColors[colOffset]! * alpha;
        colOffset++;
        colArray[colOffset] = baseColors[colOffset]! * alpha;
        colOffset++;
        colArray[colOffset] = baseColors[colOffset]! * alpha;
        colOffset++;
    }
    positionAttribute.needsUpdate = true;
    colorAttribute.needsUpdate = true;
}

type HierarchyEdgeNode = ChannelGraph["nodes"][number] & { parentId: string };

function hasHierarchyEdge(node: ChannelGraph["nodes"][number]): node is HierarchyEdgeNode {
    return node.parentId !== null;
}

function isGrandRootEdge(node: HierarchyEdgeNode) {
    return node.parentId === "grand_root";
}

function createSelectionPath() {
    const geometry = new BufferGeometry();
    geometry.setAttribute("position", new Float32BufferAttribute([], 3));
    geometry.setAttribute("color", new Float32BufferAttribute([], 3));
    const line = new Line(
        geometry,
        new LineBasicMaterial({
            vertexColors: true,
            transparent: true,
            opacity: selectionPathOpacity,
            blending: AdditiveBlending,
            depthWrite: false,
            toneMapped: false,
        })
    );
    line.visible = false;
    line.frustumCulled = false;
    return line;
}

function draw(now: number) {
    frame = requestAnimationFrame(draw);
    if (!renderer || !scene || !camera || !nodes || !composer || document.hidden) return;
    const delta = Math.min((now - lastFrame) / 1000, 0.1);
    lastFrame = now;
    props.graph.update(delta);
    for (const event of props.graph.takeVisualEvents()) effects?.play(event, now);
    effects?.update(now);
    nodeBuffer.update(props.graph.nodes, now, props.selectedId, props.activeOnly);
    (nodes.instanceMatrix.array as Float32Array).set(nodeBuffer.matrixData);
    nodes.instanceMatrix.needsUpdate = true;
    updateEdges(now);
    updateSelectionPathPositions(now);
    if (hierarchyEdges) hierarchyEdges.visible = !props.activeOnly;
    cameraController?.updateTransition(props.graph, now);
    if (!customRotationActive) controls?.update();
    constrainRotationCenterToViewport();
    updateHoverCursor();
    composer.render();
}

function updateSelectionPathPositions(now: number) {
    if (!selectionPath || !selectionPath.visible || !props.selectedId) return;
    const path = props.graph.path(props.selectedId);
    if (path.length < 2) return;

    const positionAttribute = selectionPath.geometry.getAttribute(
        "position"
    ) as Float32BufferAttribute;
    const posArray = positionAttribute.array as Float32Array;
    let offset = 0;

    for (const node of path) {
        const wxNode = Math.sin(now * 0.0008 + node.index * 1.2) * 1.5;
        const wyNode = Math.cos(now * 0.0009 + node.index * 0.8) * 1.5;
        const wzNode = Math.sin(now * 0.0007 + node.index * 1.5) * 1.5;

        posArray[offset++] = node.x + wxNode;
        posArray[offset++] = node.y + wyNode;
        posArray[offset++] = node.z + wzNode;
    }
    positionAttribute.needsUpdate = true;
}

function resize() {
    const element = host.value;
    if (!element || !renderer || !camera || !composer) return;
    const width = element.clientWidth;
    const height = element.clientHeight;
    camera.aspect = width / Math.max(1, height);
    camera.updateProjectionMatrix();
    renderer.setSize(width, height);
    composer.setSize(width, height);
}

function onPointerDown(event: PointerEvent) {
    if (props.focusId) cameraController?.cancelTransition();
    pointerDown.set(event.clientX, event.clientY);
    pointerLast.copy(pointerDown);
    pointerMoved = false;
    pointerButton = event.button;
    customRotationActive = false;
    if (event.button === 0) renderer?.domElement.setPointerCapture(event.pointerId);
}

function centerInitialCamera() {
    const root = props.graph.get("grand_root");
    if (!root) return;
    cameraController?.centerAt(root);
}

function updateCameraTarget(id: string | undefined) {
    cameraController?.focus(props.graph, id);
}

function updateSelectionPath(id: string | undefined) {
    if (!selectionPath || !id) {
        if (selectionPath) selectionPath.visible = false;
        return;
    }
    const path = props.graph.path(id);
    if (path.length < 2) {
        selectionPath.visible = false;
        return;
    }
    const positions = path.flatMap(node => [node.x, node.y, node.z]);
    selectionPath.geometry.setAttribute("position", new Float32BufferAttribute(positions, 3));
    selectionPath.geometry.setAttribute(
        "color",
        new Float32BufferAttribute(selectionPathColors(path), 3)
    );
    selectionPath.geometry.computeBoundingSphere();
    selectionPath.visible = true;
}

function selectionPathColors(path: ChannelGraph["nodes"][number][]) {
    const colors: number[] = [];
    const selectedColor = new Color(path.at(-1)?.color ?? "#ffffff");
    const rootColor = new Color(rootEdgeColor).multiplyScalar(
        rootEdgeVisualOpacity / selectionPathOpacity
    );
    for (const [index, node] of path.entries()) {
        const color = index === 0 && node.id === "grand_root" ? rootColor : selectedColor;
        colors.push(color.r, color.g, color.b);
    }
    return colors;
}

function onPointerMove(event: PointerEvent) {
    const deltaX = event.clientX - pointerLast.x;
    const deltaY = event.clientY - pointerLast.y;
    pointerLast.set(event.clientX, event.clientY);

    if (Math.hypot(event.clientX - pointerDown.x, event.clientY - pointerDown.y) > 8) {
        pointerMoved = true;
    }
    if (pointerMoved && pointerButton === 0) {
        customRotationActive = true;
        rotateAroundSelectedNode(deltaX, deltaY);
    }
    if (event.buttons === 0) {
        hoverPointer.set(event.clientX, event.clientY);
        hoverPending = true;
    }
}

function rotateAroundSelectedNode(deltaX: number, deltaY: number) {
    const element = renderer?.domElement;
    if (!element) return;
    const pivotNode = props.focusId
        ? props.graph.get(props.focusId)
        : props.graph.get("grand_root");
    if (!pivotNode) return;
    cameraController?.rotateAround(pivotNode, deltaX, deltaY, element.clientHeight);
}

function constrainRotationCenterToViewport() {
    if (props.focusId) {
        const pivotNode = props.graph.get(props.focusId);
        if (pivotNode) cameraController?.constrainPivotToViewport(pivotNode);
        return;
    }
    cameraController?.constrainPointsToViewport(props.graph.nodes.filter(isNodeRendered));
}

function onPointerUp(event: PointerEvent) {
    if (event.button === 0 && renderer?.domElement.hasPointerCapture(event.pointerId)) {
        renderer.domElement.releasePointerCapture(event.pointerId);
    }
    pointerButton = -1;
    customRotationActive = false;
    controls?.update();
    hoverPointer.set(event.clientX, event.clientY);
    hoverPending = true;
    if (pointerMoved || !renderer || !camera || !nodes) return;
    const bounds = renderer.domElement.getBoundingClientRect();
    const pickedId = pickNodeAt(event.clientX - bounds.left, event.clientY - bounds.top, bounds);

    if (pickedId === props.selectedId) return;

    emit("select", pickedId);
}

function onPointerLeave() {
    hoverPending = false;
    renderer?.domElement.classList.remove("pickable");
}

function updateHoverCursor() {
    if (!hoverPending || !renderer || !camera || !nodes) return;
    hoverPending = false;
    const bounds = renderer.domElement.getBoundingClientRect();
    const hoveredId = pickNodeAt(hoverPointer.x - bounds.left, hoverPointer.y - bounds.top, bounds);
    renderer.domElement.classList.toggle("pickable", hoveredId !== undefined);
}

function pickNodeAt(x: number, y: number, bounds: DOMRect) {
    const candidates: {
        id: string;
        distance: number;
        depth: number;
        leaf: boolean;
    }[] = [];

    for (const node of props.graph.nodes) {
        if (!isNodeRendered(node)) continue;
        projectedNode.set(node.x, node.y, node.z).project(camera!);
        if (projectedNode.z < -1 || projectedNode.z > 1) continue;
        const screenX = ((projectedNode.x + 1) / 2) * bounds.width;
        const screenY = ((1 - projectedNode.y) / 2) * bounds.height;
        const distance = Math.hypot(screenX - x, screenY - y);
        if (distance <= NODE_PICK_RADIUS) {
            candidates.push({
                id: node.id,
                distance,
                depth: node.depth,
                leaf: node.children.length === 0,
            });
        }
    }

    candidates.sort((left, right) => left.distance - right.distance);
    const nearest = candidates[0];
    if (!nearest) return undefined;
    const nearby = candidates.filter(candidate => candidate.distance <= nearest.distance + 10);
    nearby.sort((left, right) => {
        if (left.leaf !== right.leaf) return left.leaf ? -1 : 1;
        if (left.depth !== right.depth) return right.depth - left.depth;
        return left.distance - right.distance;
    });
    return nearby[0]?.id;
}

function isNodeRendered(node: ChannelGraph["nodes"][number]) {
    if (node.visibilityAlpha < 0.05) return false;
    return (
        !props.activeOnly ||
        node.relativeScore > 0.08 ||
        node.id === "grand_root" ||
        node.id === props.selectedId
    );
}

function onContextLost(event: Event) {
    event.preventDefault();
    cancelAnimationFrame(frame);
    emit("renderError", "WebGLコンテキストが失われました");
}

function resetFrameClock() {
    lastFrame = performance.now();
}

function dispose() {
    cancelAnimationFrame(frame);
    resizeObserver?.disconnect();
    document.removeEventListener("visibilitychange", resetFrameClock);
    const canvas = renderer?.domElement;
    canvas?.removeEventListener("pointerdown", onPointerDown);
    canvas?.removeEventListener("pointermove", onPointerMove);
    canvas?.removeEventListener("pointerup", onPointerUp);
    canvas?.removeEventListener("pointerleave", onPointerLeave);
    canvas?.removeEventListener("webglcontextlost", onContextLost);
    controls?.dispose();
    effects?.dispose();
    effects = undefined;
    hierarchyEdges = undefined;
    selectionPath = undefined;
    cameraController = undefined;
    composer?.dispose();
    composer = undefined;
    scene?.traverse(object => {
        if ("geometry" in object && object.geometry instanceof BufferGeometry) {
            object.geometry.dispose();
        }
        if ("material" in object) {
            const material = object.material;
            if (Array.isArray(material)) material.forEach(item => item.dispose());
            else if (
                material instanceof MeshBasicMaterial ||
                material instanceof LineBasicMaterial
            ) {
                material.dispose();
            }
        }
    });
    renderer?.dispose();
    canvas?.remove();
}

watch(
    () => props.graph,
    () => {
        dispose();
        nodeBuffer = new NodeBuffer(props.graph.nodes.length);
        initialise();
    }
);

watch(
    () => props.selectedId,
    id => {
        updateSelectionPath(id);
    },
    { immediate: true }
);

watch(
    () => [props.focusId, props.focusRevision] as const,
    ([id]) => {
        updateCameraTarget(id);
    },
    { immediate: true }
);

onMounted(initialise);
onBeforeUnmount(dispose);
</script>

<template>
    <div
        ref="host"
        class="galaxy"
    />
</template>

<style scoped>
.galaxy {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    touch-action: none;
}

.galaxy :deep(canvas) {
    display: block;
    cursor: grab;
}

.galaxy :deep(canvas.pickable) {
    cursor: pointer;
}

.galaxy :deep(canvas:active) {
    cursor: grabbing;
}
</style>
