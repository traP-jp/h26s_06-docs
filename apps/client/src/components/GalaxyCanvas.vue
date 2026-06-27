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
import { EffectPool } from "../rendering/effectPool";

const props = defineProps<{
    graph: ChannelGraph;
    selectedId?: string;
    focusId?: string;
    activeOnly: boolean;
    cameraResetKey?: number;
}>();

const emit = defineEmits<{
    select: [id: string | undefined];
    messageNodeReached: [id: string];
    renderError: [message: string];
}>();

type GraphNode = ChannelGraph["nodes"][number];

const KEYBOARD_PAN_SPEED = 420;

const host = useTemplateRef<HTMLDivElement>("host");

let renderer: WebGLRenderer | undefined;
let scene: Scene | undefined;
let camera: PerspectiveCamera | undefined;
let controls: OrbitControls | undefined;
let composer: EffectComposer | undefined;
let effects: EffectPool | undefined;
let nodes: InstancedMesh | undefined;
let hierarchyEdges: LineSegments<BufferGeometry, LineBasicMaterial> | undefined;
let selectionPath: Line<BufferGeometry, LineBasicMaterial> | undefined;

let frame = 0;
let lastFrame = performance.now();
let pointerDown = new Vector2();
let pointerMoved = false;
let resizeObserver: ResizeObserver | undefined;
let nodeBuffer = new NodeBuffer(props.graph.nodes.length);

const cameraPivot = new Vector3();
const cameraPanOffset = new Vector2();
const pressedKeys = new Set<string>();
let hasCameraPivot = false;
const projectedNode = new Vector3();
const instanceColor = new Color();

function getGrandRootNode(): GraphNode | undefined {
    return (
        props.graph.get("grand_root") ??
        props.graph.nodes.find(node => node.depth === 0) ??
        props.graph.nodes[0]
    );
}

function getFocusedNode(id: string | undefined = props.focusId): GraphNode | undefined {
    if (id) {
        const node = props.graph.get(id);
        if (node) return node;
    }

    return getGrandRootNode();
}

function getNodeBasePosition(node: GraphNode): Vector3 {
    return new Vector3(node.x, node.y, node.z);
}

function getNodeRenderedPosition(node: GraphNode): Vector3 {
    const matrixArray = nodes?.instanceMatrix.array as Float32Array | undefined;
    const offset = node.index * 16;

    if (matrixArray && matrixArray.length >= offset + 15) {
        return new Vector3(
            matrixArray[offset + 12] ?? node.x,
            matrixArray[offset + 13] ?? node.y,
            matrixArray[offset + 14] ?? node.z,
        );
    }

    return getNodeBasePosition(node);
}

function getFocusedTarget(id: string | undefined = props.focusId): Vector3 {
    const node = getFocusedNode(id);

    if (!node) {
        return new Vector3(0, 0, 0);
    }

    return getNodeRenderedPosition(node);
}

function getCameraDirection(): Vector3 {
    if (!camera || !controls) {
        return new Vector3(0, 0.12, 1).normalize();
    }

    const direction = camera.position.clone().sub(controls.target);

    if (direction.lengthSq() < 0.001) {
        direction.set(0, 0.12, 1);
    }

    return direction.normalize();
}

function getGraphExtentFrom(target: Vector3): number {
    return Math.max(
        360,
        ...props.graph.nodes.map(node => getNodeRenderedPosition(node).distanceTo(target)),
    );
}

function cameraObstacleRadius(node: GraphNode): number {
    const heat = node.relativeScore;
    const baseScale =
        node.id === "grand_root"
            ? 4.2
            : node.depth <= 1
              ? 3
              : Math.max(0.42, 2.4 * 0.72 ** (node.depth - 1));

    return (baseScale + heat * 6) * node.emphasis * node.visibilityAlpha;
}

function collectSubtree(root: GraphNode): GraphNode[] {
    const result: GraphNode[] = [root];
    const stack = [...root.children];

    while (stack.length > 0) {
        const index = stack.pop();
        if (index === undefined) continue;

        const node = props.graph.nodes[index];
        if (!node) continue;

        result.push(node);
        stack.push(...node.children);
    }

    return result;
}

function calculateRadiusAroundTarget(
    target: Vector3,
    nodesToFit: readonly GraphNode[],
): number {
    let radius = 30;

    for (const node of nodesToFit) {
        const position = getNodeRenderedPosition(node);
        const visualPadding = Math.max(24, cameraObstacleRadius(node) * 12);

        radius = Math.max(radius, position.distanceTo(target) + visualPadding);
    }

    return radius;
}

function getFitDistance(radius: number, padding = 1.25): number {
    if (!camera) return radius * 2;

    const minDistance = controls?.minDistance ?? 80;
    const verticalFov = (camera.fov * Math.PI) / 180;
    const horizontalFov =
        2 * Math.atan(Math.tan(verticalFov / 2) * camera.aspect);
    const fitFov = Math.min(verticalFov, horizontalFov);

    const distance = (radius * padding) / Math.sin(fitFov / 2);

    return Math.max(minDistance, distance);
}

function getCameraDistanceForTarget(
    target: Vector3,
    id: string | undefined = props.focusId,
): number {
    const focusedNode = getFocusedNode(id);
    const nodesToFit = id && focusedNode ? collectSubtree(focusedNode) : props.graph.nodes;
    const radius = calculateRadiusAroundTarget(target, nodesToFit);

    return getFitDistance(radius, id ? 1.35 : 1.3);
}

function centerCameraNow(id: string | undefined = props.focusId): void {
    if (!camera || !controls) return;

    resetCameraPanOffset();

    const target = getFocusedTarget(id);
    const distance = getCameraDistanceForTarget(target, id);
    const direction = getCameraDirection();

    camera.far = Math.max(camera.far, distance * 4);
    camera.updateProjectionMatrix();

    controls.maxDistance = Math.max(controls.maxDistance, distance * 1.6);
    controls.target.copy(target);

    camera.position.copy(target.clone().addScaledVector(direction, distance));
    camera.lookAt(target);

    controls.update();
    rememberCameraPivot(target);
    applyCameraViewOffset();
}

function rememberCameraPivot(target: Vector3): void {
    cameraPivot.copy(target);
    hasCameraPivot = true;
}

function syncRotationPivotBeforeControls(): void {
    if (!camera || !controls) return;

    const target = getFocusedTarget(props.focusId);

    if (hasCameraPivot) {
        const delta = target.clone().sub(cameraPivot);

        if (delta.lengthSq() > 0.000001) {
            camera.position.add(delta);
        }
    }

    // OrbitControls の回転中心は常に grand_root または選択中ノードへ追随させる。
    controls.target.copy(target);
    camera.lookAt(target);
    rememberCameraPivot(target);
}

function restoreRotationPivotAfterControls(): void {
    if (!camera || !controls) return;

    const target = getFocusedTarget(props.focusId);
    const panDelta = controls.target.clone().sub(target);

    if (panDelta.lengthSq() > 0.000001) {
        camera.position.sub(panDelta);
        accumulateCameraViewOffset(panDelta, target);
    }

    controls.target.copy(target);
    camera.lookAt(target);
    rememberCameraPivot(target);
    applyCameraViewOffset();
}

function accumulateCameraViewOffset(panDelta: Vector3, target: Vector3): void {
    if (!camera) return;

    const element = host.value;
    if (!element) return;

    const width = element.clientWidth;
    const height = element.clientHeight;
    if (width <= 0 || height <= 0) return;

    camera.updateMatrixWorld();

    const right = new Vector3().setFromMatrixColumn(camera.matrixWorld, 0).normalize();
    const up = new Vector3().setFromMatrixColumn(camera.matrixWorld, 1).normalize();
    const distance = Math.max(1, camera.position.distanceTo(target));
    const verticalSize = 2 * distance * Math.tan((camera.fov * Math.PI) / 360);
    const horizontalSize = verticalSize * camera.aspect;

    cameraPanOffset.x += (panDelta.dot(right) / horizontalSize) * width;
    cameraPanOffset.y -= (panDelta.dot(up) / verticalSize) * height;
}

function applyCameraViewOffset(): void {
    if (!camera) return;

    const element = host.value;
    if (!element) return;

    const width = element.clientWidth;
    const height = element.clientHeight;
    if (width <= 0 || height <= 0) return;

    if (cameraPanOffset.lengthSq() <= 0.000001) {
        camera.clearViewOffset();
    } else {
        camera.setViewOffset(
            width,
            height,
            cameraPanOffset.x,
            cameraPanOffset.y,
            width,
            height,
        );
    }
}

function resetCameraPanOffset(): void {
    cameraPanOffset.set(0, 0);
    camera?.clearViewOffset();
}

function updateKeyboardCameraMovement(delta: number): void {
    if (!camera) return;

    const direction = new Vector2(
        Number(pressedKeys.has("KeyD")) - Number(pressedKeys.has("KeyA")),
        Number(pressedKeys.has("KeyS")) - Number(pressedKeys.has("KeyW")),
    );

    if (direction.lengthSq() <= 0) return;

    direction.normalize().multiplyScalar(KEYBOARD_PAN_SPEED * delta);
    cameraPanOffset.add(direction);
    applyCameraViewOffset();
}

function isEditableEventTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) return false;

    return (
        target.isContentEditable ||
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement
    );
}

function onKeyDown(event: KeyboardEvent): void {
    if (isEditableEventTarget(event.target)) return;

    if (event.code === "KeyR") {
        if (event.repeat) return;

        event.preventDefault();
        centerCameraNow(props.focusId);
        return;
    }

    if (["KeyW", "KeyA", "KeyS", "KeyD"].includes(event.code)) {
        event.preventDefault();
        pressedKeys.add(event.code);
    }
}

function onKeyUp(event: KeyboardEvent): void {
    pressedKeys.delete(event.code);
}

function clearPressedKeys(): void {
    pressedKeys.clear();
}

function initialise() {
    const element = host.value;
    if (!element) return;

    try {
        scene = new Scene();
        camera = new PerspectiveCamera(52, 1, 0.1, 3000);

        const initialTarget = getFocusedTarget();
        const graphExtent = getGraphExtentFrom(initialTarget);
        const initialDistance = Math.max(620, graphExtent * 1.65);
        const initialDirection = new Vector3(0, 0.12, 1).normalize();

        camera.far = Math.max(3000, initialDistance * 4);
        camera.position.copy(
            initialTarget.clone().addScaledVector(initialDirection, initialDistance),
        );
        camera.lookAt(initialTarget);
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
        controls.minDistance = 80;
        controls.maxDistance = Math.max(1400, initialDistance * 2.5);

        // 右ドラッグ pan を残す。
        controls.enablePan = true;
        controls.screenSpacePanning = true;

        controls.target.copy(initialTarget);
        controls.update();

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
                0.08,
            ),
        );

        renderer.compile(scene, camera);

        renderer.domElement.addEventListener("pointerdown", onPointerDown);
        renderer.domElement.addEventListener("pointermove", onPointerMove);
        renderer.domElement.addEventListener("pointerup", onPointerUp);
        renderer.domElement.addEventListener("contextmenu", onContextMenu);
        renderer.domElement.addEventListener("webglcontextlost", onContextLost);

        resizeObserver = new ResizeObserver(resize);
        resizeObserver.observe(element);

        document.addEventListener("visibilitychange", resetFrameClock);
        document.addEventListener("keydown", onKeyDown);
        document.addEventListener("keyup", onKeyUp);
        window.addEventListener("blur", clearPressedKeys);

        resize();
        updateSelectionPath(props.selectedId);
        centerCameraNow(props.focusId);

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
            nodeBuffer.colorData[offset + 2] ?? 1,
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
        if (!node.parentId) continue;

        const parent = props.graph.get(node.parentId);
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
        }),
    );

    lines.frustumCulled = false;

    return lines;
}

function updateEdges(now: number) {
    if (!hierarchyEdges) return;

    const positionAttribute = hierarchyEdges.geometry.getAttribute(
        "position",
    ) as Float32BufferAttribute;
    const colorAttribute = hierarchyEdges.geometry.getAttribute("color") as Float32BufferAttribute;
    const posArray = positionAttribute.array as Float32Array;
    const colArray = colorAttribute.array as Float32Array;
    const baseColors = hierarchyEdges.geometry.userData.baseColors as Float32Array;

    let offset = 0;
    let colOffset = 0;

    for (const node of props.graph.nodes) {
        if (!node.parentId) continue;

        const parent = props.graph.get(node.parentId);
        if (!parent) continue;

        const alpha =
            Math.min(parent.visibilityAlpha, node.visibilityAlpha) *
            Math.min(parent.emphasis, node.emphasis);

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

function createSelectionPath() {
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
        }),
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

    for (const event of props.graph.takeVisualEvents()) {
        effects?.play(event, now);
    }

    effects?.update(now);

    nodeBuffer.update(props.graph.nodes, now, props.selectedId, props.activeOnly);
    (nodes.instanceMatrix.array as Float32Array).set(nodeBuffer.matrixData);
    nodes.instanceMatrix.needsUpdate = true;

    updateEdges(now);
    updateSelectionPathPositions();

    if (hierarchyEdges) hierarchyEdges.visible = !props.activeOnly;

    syncRotationPivotBeforeControls();
    controls?.update();
    restoreRotationPivotAfterControls();
    updateKeyboardCameraMovement(delta);
    composer.render();
}

function updateSelectionPathPositions() {
    if (!selectionPath || !selectionPath.visible || !props.selectedId) return;

    const path = props.graph.path(props.selectedId);
    if (path.length < 2) return;

    const positionAttribute = selectionPath.geometry.getAttribute(
        "position",
    ) as Float32BufferAttribute;
    const posArray = positionAttribute.array as Float32Array;

    let offset = 0;

    for (const node of path) {
        const position = getNodeRenderedPosition(node);

        posArray[offset++] = position.x;
        posArray[offset++] = position.y;
        posArray[offset++] = position.z;
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

    centerCameraNow(props.focusId);
}

function onPointerDown(event: PointerEvent) {
    pointerDown.set(event.clientX, event.clientY);
    pointerMoved = false;
}

function onPointerMove(event: PointerEvent) {
    if (Math.hypot(event.clientX - pointerDown.x, event.clientY - pointerDown.y) > 8) {
        pointerMoved = true;
    }
}

function onPointerUp(event: PointerEvent) {
    if (event.button !== 0) return;
    if (pointerMoved || !renderer || !camera || !nodes) return;

    const bounds = renderer.domElement.getBoundingClientRect();
    const pickedId = pickNodeAt(event.clientX - bounds.left, event.clientY - bounds.top, bounds);
    const nextSelectedId = pickedId === props.selectedId ? undefined : pickedId;

    emit("select", nextSelectedId);
}

function onContextMenu(event: Event) {
    event.preventDefault();
}

function pickNodeAt(x: number, y: number, bounds: DOMRect) {
    const candidates: {
        id: string;
        distance: number;
        depth: number;
        leaf: boolean;
    }[] = [];

    for (const node of props.graph.nodes) {
        if (node.visibilityAlpha < 0.05) continue;

        if (
            props.activeOnly &&
            node.relativeScore <= 0.08 &&
            node.id !== "grand_root" &&
            node.id !== props.selectedId
        ) {
            continue;
        }

        projectedNode.copy(getNodeRenderedPosition(node)).project(camera!);

        if (projectedNode.z < -1 || projectedNode.z > 1) continue;

        const screenX = ((projectedNode.x + 1) / 2) * bounds.width;
        const screenY = ((1 - projectedNode.y) / 2) * bounds.height;
        const distance = Math.hypot(screenX - x, screenY - y);

        if (distance <= 28) {
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

    const positions = path.flatMap(node => {
        const position = getNodeRenderedPosition(node);
        return [position.x, position.y, position.z];
    });

    selectionPath.geometry.setAttribute("position", new Float32BufferAttribute(positions, 3));
    selectionPath.geometry.computeBoundingSphere();
    selectionPath.material.color.set(path.at(-1)?.color ?? "#ffffff");
    selectionPath.visible = true;
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
    document.removeEventListener("keydown", onKeyDown);
    document.removeEventListener("keyup", onKeyUp);
    window.removeEventListener("blur", clearPressedKeys);
    clearPressedKeys();

    const canvas = renderer?.domElement;

    canvas?.removeEventListener("pointerdown", onPointerDown);
    canvas?.removeEventListener("pointermove", onPointerMove);
    canvas?.removeEventListener("pointerup", onPointerUp);
    canvas?.removeEventListener("contextmenu", onContextMenu);
    canvas?.removeEventListener("webglcontextlost", onContextLost);

    controls?.dispose();
    effects?.dispose();

    effects = undefined;
    hierarchyEdges = undefined;
    selectionPath = undefined;

    composer?.dispose();
    composer = undefined;

    scene?.traverse(object => {
        if ("geometry" in object && object.geometry instanceof BufferGeometry) {
            object.geometry.dispose();
        }

        if ("material" in object) {
            const material = object.material;

            if (Array.isArray(material)) {
                material.forEach(item => item.dispose());
            } else if (
                material instanceof MeshBasicMaterial ||
                material instanceof LineBasicMaterial
            ) {
                material.dispose();
            }
        }
    });

    renderer?.dispose();
    canvas?.remove();

    renderer = undefined;
    scene = undefined;
    camera = undefined;
    controls = undefined;
    nodes = undefined;
}

watch(
    () => props.graph,
    () => {
        dispose();
        nodeBuffer = new NodeBuffer(props.graph.nodes.length);
        initialise();
    },
);

watch(
    () => props.selectedId,
    id => {
        updateSelectionPath(id);
    },
    { immediate: true },
);

watch(
    () => props.focusId,
    id => {
        centerCameraNow(id);
    },
);

watch(
    () => props.cameraResetKey,
    (next, previous) => {
        if (next === previous) return;
        centerCameraNow(props.focusId);
    },
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
    overflow: hidden;
    touch-action: none;
}

.galaxy :deep(canvas) {
    display: block;
    width: 100%;
    height: 100%;
    cursor: grab;
}

.galaxy :deep(canvas:active) {
    cursor: grabbing;
}
</style>
