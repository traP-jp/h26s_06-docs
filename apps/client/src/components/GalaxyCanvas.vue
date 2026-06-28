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
    ShaderMaterial,
    SphereGeometry,
    Vector2,
    Vector3,
    WebGLRenderer,
} from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";

import {
    ACTIVE_RELATIVE_SCORE_THRESHOLD,
    type ChannelDisplayMode,
    type ChannelGraph,
} from "../core/channelGraph";
import type {
    CameraMoveDirection,
    CameraRotationDirection,
    CameraZoomDirection,
} from "../core/keyboardController";
import { NodeBuffer } from "../core/nodeBuffer";
import { CameraController } from "../rendering/cameraController";
import { EffectPool } from "../rendering/effectPool";
import { HierarchyEdgeBuffer } from "../rendering/hierarchyEdgeBuffer";
import { nodeFragmentShader, nodeVertexShader } from "../rendering/nodeShaders";
import { nodeWaverX, nodeWaverY, nodeWaverZ } from "../rendering/nodeWaver";

const props = defineProps<{
    graph: ChannelGraph;
    selectedId?: string;
    focusId?: string;
    focusRevision: number;
    activeOnly: boolean;
    displayMode: ChannelDisplayMode;
}>();
const emit = defineEmits<{
    select: [id: string | undefined];
    messageNodeReached: [id: string];
    activityChange: [activity: number];
    renderError: [message: string];
}>();
const host = useTemplateRef<HTMLDivElement>("host");
const selectionPathOpacity = 0.82;
const rootEdgeColor = "#7b8798";
const rootEdgeVisualOpacity = 0.26;
const idleAutoRotationDelay = 5000;
const idleAutoRotationRadiansPerSecond = 0.012;
const keyboardPanAcceleration = 920;
const keyboardPanMaxSpeed = 520;
const keyboardPanDamping = 7.5;
const keyboardRotationAcceleration = 760;
const keyboardRotationMaxSpeed = 430;
const keyboardRotationDamping = 8;
const keyboardZoomAcceleration = 3.2;
const keyboardZoomMaxSpeed = 1.8;
const keyboardZoomDamping = 6;
const keyboardMotionEpsilon = 0.01;
const shootingStarPoolSize = 40;
const shootingStarMeanInterval = 3000;
const shootingStarClusterChance = 0.18;
const shootingStarClusterMax = 5;

interface ShootingStar {
    active: boolean;
    startedAt: number;
    duration: number;
    start: Vector3;
    direction: Vector3;
    length: number;
    travel: number;
    brightness: number;
}

let renderer: WebGLRenderer | undefined;
let scene: Scene | undefined;
let camera: PerspectiveCamera | undefined;
let controls: OrbitControls | undefined;
let cameraController: CameraController | undefined;
let composer: EffectComposer | undefined;
let effects: EffectPool | undefined;
let nodes: InstancedMesh<SphereGeometry, ShaderMaterial> | undefined;
let shootingStars: LineSegments<BufferGeometry, LineBasicMaterial> | undefined;
let hierarchyEdges: LineSegments<BufferGeometry, LineBasicMaterial> | undefined;
let selectionPath: Line<BufferGeometry, LineBasicMaterial> | undefined;
let frame = 0;
let lastFrame = performance.now();
let lastCameraInteractionAt = performance.now();
let pointerDown = new Vector2();
let pointerLast = new Vector2();
let pointerMoved = false;
let pointerButton = -1;
let customRotationActive = false;
let hoverPending = false;
let resizeObserver: ResizeObserver | undefined;
let nodeBuffer = new NodeBuffer(props.graph.nodes.length);
let hierarchyEdgeBuffer = new HierarchyEdgeBuffer(props.graph.nodes);
let lastActivity = -1;
let activitySelectedId: string | undefined;

function updateSelectedActivity(allowIncrease = false) {
    const selectedId = props.selectedId;
    const relativeScore = selectedId ? (props.graph.get(selectedId)?.relativeScore ?? 0) : 0;
    const activity = Math.round(Math.min(1, Math.max(0, relativeScore)) * 100);

    if (selectedId !== activitySelectedId) {
        activitySelectedId = selectedId;
        lastActivity = activity;
        emit("activityChange", activity);
        return;
    }
    if (activity > lastActivity && !allowIncrease) return;
    if (activity === lastActivity) return;

    lastActivity = activity;
    emit("activityChange", activity);
}

function handleActivityNodeReached(id: string) {
    if (id === props.selectedId) updateSelectedActivity(true);
}
let hierarchyEdgeBaseColors = createEdgeBaseColors();
const projectedNode = new Vector3();
const hoverPointer = new Vector2();
const instanceColor = new Color();
const activeCameraMoveDirections = new Set<CameraMoveDirection>();
const activeCameraZoomDirections = new Set<CameraZoomDirection>();
const activeCameraRotationDirections = new Set<CameraRotationDirection>();
const keyboardPanVelocity = new Vector2();
const keyboardRotationVelocity = new Vector2();
const keyboardPanInput = new Vector2();
const keyboardRotationInput = new Vector2();
let keyboardZoomVelocity = 0;
let nextShootingStarAt = performance.now() + randomShootingStarInterval();
const shootingStarPool: ShootingStar[] = Array.from({ length: shootingStarPoolSize }, () => ({
    active: false,
    startedAt: 0,
    duration: 0,
    start: new Vector3(),
    direction: new Vector3(),
    length: 0,
    travel: 0,
    brightness: 0,
}));
const shootingStarViewDirection = new Vector3();
const shootingStarRight = new Vector3();
const shootingStarUp = new Vector3();
const shootingStarHead = new Vector3();
const shootingStarTail = new Vector3();
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

        resetShootingStars();
        shootingStars = createShootingStars();
        scene.add(shootingStars);
        nodes = createNodeMesh();
        scene.add(nodes);
        hierarchyEdges = createEdges();
        scene.add(hierarchyEdges);
        selectionPath = createSelectionPath();
        scene.add(selectionPath);
        effects = new EffectPool(
            scene,
            props.graph,
            id => emit("messageNodeReached", id),
            handleActivityNodeReached
        );
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
        window.addEventListener("keydown", noteCameraInteraction);
        window.addEventListener("mousedown", noteCameraInteraction);
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
    const geometry = new SphereGeometry(1, 18, 12);
    const material = new ShaderMaterial({
        uniforms: {
            uTime: { value: 0 },
        },
        vertexShader: nodeVertexShader,
        fragmentShader: nodeFragmentShader,
        toneMapped: false,
        transparent: true,
        blending: AdditiveBlending,
        depthWrite: false,
    });
    const mesh = new InstancedMesh(geometry, material, props.graph.nodes.length);
    mesh.instanceMatrix.setUsage(DynamicDrawUsage);
    nodeBuffer.update(
        props.graph.nodes,
        performance.now(),
        props.selectedId,
        props.activeOnly,
        props.displayMode
    );
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

function createShootingStars() {
    const positions = new Float32Array(shootingStarPoolSize * 6);
    const colors = new Float32Array(shootingStarPoolSize * 6);
    const geometry = new BufferGeometry();
    geometry.setAttribute(
        "position",
        new Float32BufferAttribute(positions, 3).setUsage(DynamicDrawUsage)
    );
    geometry.setAttribute(
        "color",
        new Float32BufferAttribute(colors, 3).setUsage(DynamicDrawUsage)
    );
    geometry.setDrawRange(0, 0);
    const lines = new LineSegments(
        geometry,
        new LineBasicMaterial({
            vertexColors: true,
            transparent: true,
            opacity: 0.72,
            blending: AdditiveBlending,
            depthWrite: false,
            toneMapped: false,
        })
    );
    lines.frustumCulled = false;
    lines.renderOrder = -10;
    return lines;
}

function createEdges() {
    const positions = new Float32Array(hierarchyEdgeBuffer.capacity * 6);
    const colors = new Float32Array(hierarchyEdgeBuffer.capacity * 6);
    const geometry = new BufferGeometry();
    geometry.setAttribute(
        "position",
        new Float32BufferAttribute(positions, 3).setUsage(DynamicDrawUsage)
    );
    geometry.setAttribute(
        "color",
        new Float32BufferAttribute(colors, 3).setUsage(DynamicDrawUsage)
    );
    geometry.setDrawRange(0, 0);
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
    if (props.displayMode === "all") {
        hierarchyEdges.geometry.setDrawRange(0, 0);
        return;
    }
    const positionAttribute = hierarchyEdges.geometry.getAttribute(
        "position"
    ) as Float32BufferAttribute;
    const colorAttribute = hierarchyEdges.geometry.getAttribute("color") as Float32BufferAttribute;
    const posArray = positionAttribute.array as Float32Array;
    const colArray = colorAttribute.array as Float32Array;
    hierarchyEdgeBuffer.update(props.graph.nodes, props.activeOnly);
    hierarchyEdges.geometry.setDrawRange(0, hierarchyEdgeBuffer.count * 2);

    let offset = 0;
    for (let edgeIndex = 0; edgeIndex < hierarchyEdgeBuffer.count; edgeIndex += 1) {
        const nodeIndex = hierarchyEdgeBuffer.nodeIndices[edgeIndex] ?? -1;
        const parentIndex = hierarchyEdgeBuffer.parentIndices[nodeIndex] ?? -1;
        const node = props.graph.nodes[nodeIndex];
        const parent = props.graph.nodes[parentIndex];
        if (!node || !parent) continue;

        const alphaScale = isGrandRootEdge(node) ? 0.26 : 1;
        const activityScale = props.activeOnly
            ? 0.3 + Math.sqrt(hierarchyEdgeBuffer.activity[edgeIndex] ?? 0) * 0.7
            : 1;
        const alpha =
            Math.min(parent.visibilityAlpha, node.visibilityAlpha) *
            Math.min(parent.emphasis, node.emphasis) *
            alphaScale *
            activityScale;

        posArray[offset++] = parent.x + nodeWaverX(now, parent.index);
        posArray[offset++] = parent.y + nodeWaverY(now, parent.index);
        posArray[offset++] = parent.z + nodeWaverZ(now, parent.index);
        posArray[offset++] = node.x + nodeWaverX(now, node.index);
        posArray[offset++] = node.y + nodeWaverY(now, node.index);
        posArray[offset++] = node.z + nodeWaverZ(now, node.index);

        const baseOffset = nodeIndex * 6;
        const colorOffset = edgeIndex * 6;
        for (let component = 0; component < 6; component += 1) {
            colArray[colorOffset + component] =
                (hierarchyEdgeBaseColors[baseOffset + component] ?? 0) * alpha;
        }
    }
    positionAttribute.needsUpdate = true;
    colorAttribute.needsUpdate = true;
}

type HierarchyEdgeNode = ChannelGraph["nodes"][number] & { parentId: string };

function hasHierarchyEdge(node: ChannelGraph["nodes"][number]): node is HierarchyEdgeNode {
    return node.parentId !== null;
}

function isGrandRootEdge(node: ChannelGraph["nodes"][number]) {
    return node.parentId === "grand_root";
}

function createEdgeBaseColors() {
    const colors = new Float32Array(props.graph.nodes.length * 6);
    const color = new Color();
    for (const node of props.graph.nodes) {
        if (!hasHierarchyEdge(node)) continue;
        const parentIndex = hierarchyEdgeBuffer.parentIndices[node.index] ?? -1;
        const parent = props.graph.nodes[parentIndex];
        if (!parent) continue;
        const offset = node.index * 6;
        color.set(isGrandRootEdge(node) ? rootEdgeColor : parent.color);
        colors[offset] = color.r;
        colors[offset + 1] = color.g;
        colors[offset + 2] = color.b;
        color.set(isGrandRootEdge(node) ? rootEdgeColor : node.color);
        colors[offset + 3] = color.r;
        colors[offset + 4] = color.g;
        colors[offset + 5] = color.b;
    }
    return colors;
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
    updateSelectedActivity();
    for (const event of props.graph.takeVisualEvents()) effects?.play(event, now);
    effects?.update(now);
    updateShootingStars(now);
    nodeBuffer.update(
        props.graph.nodes,
        now,
        props.selectedId,
        props.activeOnly,
        props.displayMode
    );
    (nodes.instanceMatrix.array as Float32Array).set(nodeBuffer.matrixData);
    nodes.instanceMatrix.needsUpdate = true;
    updateEdges(now);
    updateSelectionPathPositions(now);
    updateKeyboardCameraMotion(delta);
    cameraController?.updateTransition(props.graph, now);
    updateIdleAutoRotation(now, delta);
    const timeUniform = nodes.material.uniforms.uTime;
    if (timeUniform) timeUniform.value = now * 0.001;
    if (!customRotationActive) controls?.update();
    constrainRotationCenterToViewport();
    updateHoverCursor();
    composer.render();
}

function updateShootingStars(now: number) {
    if (!shootingStars || !camera) return;
    while (now >= nextShootingStarAt) {
        const count =
            Math.random() < shootingStarClusterChance
                ? 2 + Math.floor(Math.random() * (shootingStarClusterMax - 1))
                : 1;
        for (let index = 0; index < count; index += 1) {
            spawnShootingStar(now, index, count);
        }
        nextShootingStarAt += randomShootingStarInterval();
    }

    const positionAttribute = shootingStars.geometry.getAttribute(
        "position"
    ) as Float32BufferAttribute;
    const colorAttribute = shootingStars.geometry.getAttribute("color") as Float32BufferAttribute;
    const posArray = positionAttribute.array as Float32Array;
    const colArray = colorAttribute.array as Float32Array;
    let offset = 0;

    for (const star of shootingStarPool) {
        if (!star.active) continue;
        const progress = (now - star.startedAt) / star.duration;
        if (progress < 0) continue;
        if (progress >= 1) {
            star.active = false;
            continue;
        }

        const fade = Math.sin(progress * Math.PI) ** 1.35 * star.brightness;
        shootingStarHead.copy(star.start).addScaledVector(star.direction, star.travel * progress);
        shootingStarTail.copy(shootingStarHead).addScaledVector(star.direction, -star.length);

        posArray[offset] = shootingStarTail.x;
        posArray[offset + 1] = shootingStarTail.y;
        posArray[offset + 2] = shootingStarTail.z;
        posArray[offset + 3] = shootingStarHead.x;
        posArray[offset + 4] = shootingStarHead.y;
        posArray[offset + 5] = shootingStarHead.z;

        colArray[offset] = 0.2 * fade;
        colArray[offset + 1] = 0.34 * fade;
        colArray[offset + 2] = 0.52 * fade;
        colArray[offset + 3] = 0.78 * fade;
        colArray[offset + 4] = 0.92 * fade;
        colArray[offset + 5] = 1.0 * fade;
        offset += 6;
    }

    shootingStars.geometry.setDrawRange(0, (offset / 3) | 0);
    positionAttribute.needsUpdate = true;
    colorAttribute.needsUpdate = true;
}

function spawnShootingStar(now: number, clusterIndex: number, clusterCount: number) {
    if (!camera) return;
    const star = shootingStarPool.find(item => !item.active) ?? shootingStarPool[0];
    if (!star) return;
    const depthBase = controls ? camera.position.distanceTo(controls.target) : 800;
    const depth = Math.min(camera.far * 0.58, Math.max(720, depthBase + 620));
    const viewportHeight = 2 * Math.tan((camera.fov * Math.PI) / 360) * depth;
    const viewportWidth = viewportHeight * camera.aspect;
    const clusterSpread = clusterCount <= 1 ? 0 : viewportHeight * 0.045;

    camera.getWorldDirection(shootingStarViewDirection).normalize();
    shootingStarRight.set(1, 0, 0).applyQuaternion(camera.quaternion).normalize();
    shootingStarUp.set(0, 1, 0).applyQuaternion(camera.quaternion).normalize();

    const clusterOffset = (clusterIndex - (clusterCount - 1) / 2) * clusterSpread;
    const x = (Math.random() * 1.35 - 0.68) * viewportWidth + clusterOffset * 0.35;
    const y = (Math.random() * 1.24 - 0.62) * viewportHeight + clusterOffset;
    const screenDirectionX = 0.72 + Math.random() * 0.28;
    const screenDirectionY = -(0.3 + Math.random() * 0.28);
    const direction = shootingStarRight
        .clone()
        .multiplyScalar(screenDirectionX)
        .addScaledVector(shootingStarUp, screenDirectionY)
        .normalize();

    star.active = true;
    star.startedAt = now + clusterIndex * (45 + Math.random() * 65);
    star.duration = 680 + Math.random() * 420;
    star.length = viewportHeight * (0.035 + Math.random() * 0.045);
    star.travel = viewportHeight * (0.22 + Math.random() * 0.2);
    star.brightness = 0.54 + Math.random() * 0.46;
    star.direction.copy(direction);
    star.start
        .copy(camera.position)
        .addScaledVector(shootingStarViewDirection, depth)
        .addScaledVector(shootingStarRight, x)
        .addScaledVector(shootingStarUp, y);
}

function resetShootingStars() {
    for (const star of shootingStarPool) star.active = false;
    nextShootingStarAt = performance.now() + randomShootingStarInterval();
}

function randomShootingStarInterval() {
    return -Math.log(Math.max(0.001, Math.random())) * shootingStarMeanInterval;
}

function updateIdleAutoRotation(now: number, delta: number) {
    if (customRotationActive || now - lastCameraInteractionAt < idleAutoRotationDelay) return;
    const element = renderer?.domElement;
    if (!element) return;
    const rotationScale = (Math.PI * 2) / Math.max(1, element.clientHeight);
    const deltaX = -(idleAutoRotationRadiansPerSecond * delta) / rotationScale;
    rotateAroundSelectedNode(deltaX, 0);
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
        posArray[offset++] = node.x + nodeWaverX(now, node.index);
        posArray[offset++] = node.y + nodeWaverY(now, node.index);
        posArray[offset++] = node.z + nodeWaverZ(now, node.index);
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

function noteCameraInteraction() {
    lastCameraInteractionAt = performance.now();
}

function setCameraMoveActive(direction: CameraMoveDirection, active: boolean) {
    setActiveDirection(activeCameraMoveDirections, direction, active);
}

function setCameraZoomActive(direction: CameraZoomDirection, active: boolean) {
    setActiveDirection(activeCameraZoomDirections, direction, active);
}

function setCameraRotationActive(direction: CameraRotationDirection, active: boolean) {
    setActiveDirection(activeCameraRotationDirections, direction, active);
}

function releaseCameraControls() {
    activeCameraMoveDirections.clear();
    activeCameraZoomDirections.clear();
    activeCameraRotationDirections.clear();
    resetKeyboardCameraMotion();
}

function setActiveDirection<T>(directions: Set<T>, direction: T, active: boolean) {
    if (active) directions.add(direction);
    else directions.delete(direction);
}

function updateKeyboardCameraMotion(delta: number) {
    const element = renderer?.domElement;
    if (!element || !cameraController) return;

    const panInput = cameraMoveInput();
    const rotationInput = cameraRotationInput();
    const zoomInput = cameraZoomInput();
    const hasInput = panInput.lengthSq() > 0 || rotationInput.lengthSq() > 0 || zoomInput !== 0;

    if (!hasInput && !hasKeyboardCameraMotion()) return;
    if (hasInput) cameraController.cancelTransition();

    keyboardPanVelocity.set(
        updateKeyboardVelocity(
            keyboardPanVelocity.x,
            panInput.x,
            keyboardPanAcceleration,
            keyboardPanMaxSpeed,
            keyboardPanDamping,
            delta
        ),
        updateKeyboardVelocity(
            keyboardPanVelocity.y,
            panInput.y,
            keyboardPanAcceleration,
            keyboardPanMaxSpeed,
            keyboardPanDamping,
            delta
        )
    );
    keyboardRotationVelocity.set(
        updateKeyboardVelocity(
            keyboardRotationVelocity.x,
            rotationInput.x,
            keyboardRotationAcceleration,
            keyboardRotationMaxSpeed,
            keyboardRotationDamping,
            delta
        ),
        updateKeyboardVelocity(
            keyboardRotationVelocity.y,
            rotationInput.y,
            keyboardRotationAcceleration,
            keyboardRotationMaxSpeed,
            keyboardRotationDamping,
            delta
        )
    );
    keyboardZoomVelocity = updateKeyboardVelocity(
        keyboardZoomVelocity,
        zoomInput,
        keyboardZoomAcceleration,
        keyboardZoomMaxSpeed,
        keyboardZoomDamping,
        delta
    );

    if (!hasKeyboardCameraMotion()) return;

    noteCameraInteraction();
    if (keyboardPanVelocity.lengthSq() > 0) {
        cameraController.moveInView(
            keyboardPanVelocity.x * delta,
            keyboardPanVelocity.y * delta,
            element.clientHeight
        );
    }
    if (Math.abs(keyboardZoomVelocity) > 0) {
        cameraController.zoomBy(Math.exp(-keyboardZoomVelocity * delta));
    }
    if (keyboardRotationVelocity.lengthSq() > 0) {
        rotateAroundSelectedNode(
            keyboardRotationVelocity.x * delta,
            keyboardRotationVelocity.y * delta
        );
    }
}

function cameraMoveInput() {
    keyboardPanInput.set(
        directionAxis(activeCameraMoveDirections, "left", "right"),
        directionAxis(activeCameraMoveDirections, "up", "down")
    );
    return normalizeKeyboardInput(keyboardPanInput);
}

function cameraRotationInput() {
    keyboardRotationInput.set(
        directionAxis(activeCameraRotationDirections, "left", "right"),
        directionAxis(activeCameraRotationDirections, "up", "down")
    );
    return normalizeKeyboardInput(keyboardRotationInput);
}

function cameraZoomInput() {
    return (
        Number(activeCameraZoomDirections.has("in")) - Number(activeCameraZoomDirections.has("out"))
    );
}

function directionAxis<T>(directions: Set<T>, negative: T, positive: T) {
    return Number(directions.has(positive)) - Number(directions.has(negative));
}

function normalizeKeyboardInput(input: Vector2) {
    const length = input.length();
    if (length > 1) input.multiplyScalar(1 / length);
    return input;
}

function updateKeyboardVelocity(
    current: number,
    input: number,
    acceleration: number,
    maxSpeed: number,
    damping: number,
    delta: number
) {
    if (input !== 0) {
        return clamp(current + input * acceleration * delta, -maxSpeed, maxSpeed);
    }

    const next = current * Math.exp(-damping * delta);
    return Math.abs(next) < keyboardMotionEpsilon ? 0 : next;
}

function hasKeyboardCameraMotion() {
    return (
        keyboardPanVelocity.lengthSq() > keyboardMotionEpsilon ** 2 ||
        keyboardRotationVelocity.lengthSq() > keyboardMotionEpsilon ** 2 ||
        Math.abs(keyboardZoomVelocity) > keyboardMotionEpsilon
    );
}

function resetKeyboardCameraMotion() {
    keyboardPanVelocity.set(0, 0);
    keyboardRotationVelocity.set(0, 0);
    keyboardZoomVelocity = 0;
}

function clamp(value: number, min: number, max: number) {
    return Math.max(min, Math.min(max, value));
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
    resetKeyboardCameraMotion();
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
        node.relativeScore > ACTIVE_RELATIVE_SCORE_THRESHOLD ||
        node.activeDescendantScore > 0 ||
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
    nextShootingStarAt = lastFrame + randomShootingStarInterval();
}

function dispose() {
    cancelAnimationFrame(frame);
    resizeObserver?.disconnect();
    document.removeEventListener("visibilitychange", resetFrameClock);
    window.removeEventListener("keydown", noteCameraInteraction);
    window.removeEventListener("mousedown", noteCameraInteraction);
    const canvas = renderer?.domElement;
    canvas?.removeEventListener("pointerdown", onPointerDown);
    canvas?.removeEventListener("pointermove", onPointerMove);
    canvas?.removeEventListener("pointerup", onPointerUp);
    canvas?.removeEventListener("pointerleave", onPointerLeave);
    canvas?.removeEventListener("webglcontextlost", onContextLost);
    controls?.dispose();
    effects?.dispose();
    effects = undefined;
    shootingStars = undefined;
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
                material instanceof ShaderMaterial ||
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
        hierarchyEdgeBuffer = new HierarchyEdgeBuffer(props.graph.nodes);
        hierarchyEdgeBaseColors = createEdgeBaseColors();
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

defineExpose({
    setCameraMoveActive,
    setCameraZoomActive,
    setCameraRotationActive,
    releaseCameraControls,
});
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
