import { PerspectiveCamera, Quaternion, Vector3 } from "three";
import type { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";

import { calculateCameraAvoidance } from "../core/cameraAvoidance";
import { calculateCameraFrame } from "../core/cameraFraming";
import { calculateCameraViewportShift } from "../core/cameraViewport";
import type { ChannelGraph } from "../core/channelGraph";

const TRANSITION_DURATION = 720;
const ROTATION_CENTER_VIEWPORT_LIMIT = 0.92;

interface CameraTransition {
    startedAt: number;
    startPosition: Vector3;
    startTarget: Vector3;
    direction: Vector3;
    distance: number;
    targetOffset: Vector3;
    positionOffset: Vector3;
    nodeId: string;
}

export class CameraController {
    private transition: CameraTransition | undefined;
    private readonly projectedRotationCenter = new Vector3();
    private readonly desiredRotationCenter = new Vector3();
    private readonly projectedPoint = new Vector3();
    private readonly viewportOrigin = new Vector3();
    private readonly shiftedViewportOrigin = new Vector3();

    constructor(
        private readonly camera: PerspectiveCamera,
        private readonly controls: OrbitControls
    ) {}

    centerAt(position: { x: number; y: number; z: number }) {
        const distance = this.camera.position.distanceTo(this.controls.target);
        this.controls.target.set(position.x, position.y, position.z);
        this.camera.position.set(position.x, position.y, position.z + distance);
        this.camera.lookAt(this.controls.target);
        this.controls.update();
        this.transition = undefined;
    }

    focus(graph: ChannelGraph, id: string | undefined) {
        const direction = this.camera.position.clone().sub(this.controls.target);
        if (direction.lengthSq() < 0.001) direction.set(0, 0, 1);
        direction.normalize();

        if (!id) {
            this.transition = {
                startedAt: performance.now(),
                startPosition: this.camera.position.clone(),
                startTarget: this.controls.target.clone(),
                direction,
                distance: 800,
                targetOffset: new Vector3(),
                positionOffset: new Vector3(),
                nodeId: "grand_root",
            };
            return;
        }

        const node = graph.get(id);
        if (!node) return;
        const expandedNodes = [
            node,
            ...node.children
                .map(index => graph.nodes[index])
                .filter(
                    (child): child is ChannelGraph["nodes"][number] =>
                        child !== undefined && child.isLayoutActive
                ),
        ];
        const frame = calculateCameraFrame(
            expandedNodes.map(expandedNode => ({
                x: expandedNode.targetX,
                y: expandedNode.targetY,
                z: expandedNode.targetZ,
            })),
            direction,
            this.camera.fov,
            this.camera.aspect,
            { x: node.targetX, y: node.targetY, z: node.targetZ }
        );
        const target = new Vector3(frame.target.x, frame.target.y, frame.target.z);
        const targetOffset = target
            .clone()
            .sub(new Vector3(node.targetX, node.targetY, node.targetZ));
        const basePosition = target.clone().addScaledVector(direction, frame.distance);
        const path = graph.path(id);
        const pathIds = new Set(path.map(pathNode => pathNode.id));
        const obstacles = graph.nodes
            .filter(
                candidate =>
                    !pathIds.has(candidate.id) &&
                    candidate.visibilityAlpha > 0.15 &&
                    candidate.isLayoutActive
            )
            .map(candidate => ({
                position: {
                    x: candidate.targetX,
                    y: candidate.targetY,
                    z: candidate.targetZ,
                },
                radius: cameraObstacleRadius(candidate),
            }))
            .filter(obstacle => obstacle.radius >= 4.5);
        const offset = calculatePathAvoidance(
            basePosition,
            path.map(pathNode => ({
                x: pathNode.targetX,
                y: pathNode.targetY,
                z: pathNode.targetZ,
            })),
            obstacles
        );

        this.transition = {
            startedAt: performance.now(),
            startPosition: this.camera.position.clone(),
            startTarget: this.controls.target.clone(),
            direction,
            distance: frame.distance,
            targetOffset,
            positionOffset: new Vector3(offset.x, offset.y, offset.z),
            nodeId: id,
        };
    }

    cancelTransition() {
        this.transition = undefined;
    }

    updateTransition(graph: ChannelGraph, now: number) {
        if (!this.transition) return;
        const progress = Math.min(1, (now - this.transition.startedAt) / TRANSITION_DURATION);
        const eased = 1 - (1 - progress) ** 3;
        const node = graph.get(this.transition.nodeId);
        if (!node) {
            this.transition = undefined;
            return;
        }

        const target = new Vector3(node.targetX, node.targetY, node.targetZ).add(
            this.transition.targetOffset
        );
        const position = target
            .clone()
            .addScaledVector(this.transition.direction, this.transition.distance)
            .add(this.transition.positionOffset);
        this.camera.position.lerpVectors(this.transition.startPosition, position, eased);
        this.controls.target.lerpVectors(this.transition.startTarget, target, eased);

        if (progress >= 1) this.transition = undefined;
    }

    rotateAround(
        pivot: { x: number; y: number; z: number },
        deltaX: number,
        deltaY: number,
        viewportHeight: number
    ) {
        const pivotPosition = new Vector3(pivot.x, pivot.y, pivot.z);
        const worldUp = this.camera.up.clone().normalize();
        const rotationScale = (Math.PI * 2) / Math.max(1, viewportHeight);
        this.rotatePose(pivotPosition, worldUp, -deltaX * rotationScale);

        const cameraRight = new Vector3(1, 0, 0)
            .applyQuaternion(this.camera.quaternion)
            .normalize();
        const pitch = -deltaY * rotationScale;
        const nextOffset = this.camera.position
            .clone()
            .sub(this.controls.target)
            .applyAxisAngle(cameraRight, pitch);
        const nextPolarAngle = nextOffset.angleTo(worldUp);
        if (nextPolarAngle > 0.08 && nextPolarAngle < Math.PI - 0.08) {
            this.rotatePose(pivotPosition, cameraRight, pitch);
        }
    }

    constrainPivotToViewport(pivot: { x: number; y: number; z: number }) {
        this.projectedRotationCenter.set(pivot.x, pivot.y, pivot.z).project(this.camera);
        const clampedX = Math.max(
            -ROTATION_CENTER_VIEWPORT_LIMIT,
            Math.min(ROTATION_CENTER_VIEWPORT_LIMIT, this.projectedRotationCenter.x)
        );
        const clampedY = Math.max(
            -ROTATION_CENTER_VIEWPORT_LIMIT,
            Math.min(ROTATION_CENTER_VIEWPORT_LIMIT, this.projectedRotationCenter.y)
        );
        if (
            clampedX === this.projectedRotationCenter.x &&
            clampedY === this.projectedRotationCenter.y
        ) {
            return;
        }

        this.desiredRotationCenter
            .set(clampedX, clampedY, this.projectedRotationCenter.z)
            .unproject(this.camera);
        const correction = new Vector3(pivot.x, pivot.y, pivot.z).sub(this.desiredRotationCenter);
        this.camera.position.add(correction);
        this.controls.target.add(correction);
        this.camera.updateMatrixWorld();
    }

    constrainPointsToViewport(points: readonly { x: number; y: number; z: number }[]) {
        const projectedPoints: { x: number; y: number; z: number }[] = [];
        for (const point of points) {
            this.projectedPoint.set(point.x, point.y, point.z).project(this.camera);
            if (
                this.projectedPoint.z < -1 ||
                this.projectedPoint.z > 1 ||
                !Number.isFinite(this.projectedPoint.x) ||
                !Number.isFinite(this.projectedPoint.y)
            ) {
                continue;
            }
            projectedPoints.push({
                x: this.projectedPoint.x,
                y: this.projectedPoint.y,
                z: this.projectedPoint.z,
            });
        }

        const shift = calculateCameraViewportShift(projectedPoints, ROTATION_CENTER_VIEWPORT_LIMIT);
        if (shift.x === 0 && shift.y === 0) return;

        const depth =
            projectedPoints.reduce((sum, point) => sum + point.z, 0) / projectedPoints.length;
        this.viewportOrigin.set(0, 0, depth).unproject(this.camera);
        this.shiftedViewportOrigin.set(shift.x, shift.y, depth).unproject(this.camera);
        const correction = this.viewportOrigin.sub(this.shiftedViewportOrigin);
        this.camera.position.add(correction);
        this.controls.target.add(correction);
        this.camera.updateMatrixWorld();
    }

    private rotatePose(pivot: Vector3, axis: Vector3, angle: number) {
        if (Math.abs(angle) < 0.000_001) return;
        const rotation = new Quaternion().setFromAxisAngle(axis, angle);
        this.camera.position.sub(pivot).applyAxisAngle(axis, angle).add(pivot);
        this.controls.target.sub(pivot).applyAxisAngle(axis, angle).add(pivot);
        this.camera.quaternion.premultiply(rotation);
        this.camera.updateMatrixWorld();
    }
}

function cameraObstacleRadius(node: ChannelGraph["nodes"][number]) {
    const heat = node.relativeScore;
    const baseScale =
        node.id === "grand_root"
            ? 4.2
            : node.depth <= 1
              ? 3
              : Math.max(0.42, 2.4 * 0.72 ** (node.depth - 1));
    return (baseScale + heat * 6) * node.emphasis * node.visibilityAlpha;
}

function calculatePathAvoidance(
    cameraPosition: { x: number; y: number; z: number },
    path: readonly { x: number; y: number; z: number }[],
    obstacles: Parameters<typeof calculateCameraAvoidance>[2]
) {
    let strongest = { x: 0, y: 0, z: 0 };
    let strongestLength = 0;
    for (const pathNode of path) {
        const offset = calculateCameraAvoidance(cameraPosition, pathNode, obstacles);
        const length = Math.hypot(offset.x, offset.y, offset.z);
        if (length > strongestLength) {
            strongest = offset;
            strongestLength = length;
        }
    }
    return strongest;
}
