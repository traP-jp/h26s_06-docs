export interface Point3 {
    x: number;
    y: number;
    z: number;
}

export interface CameraObstacle {
    position: Point3;
    radius: number;
}

const MAX_CAMERA_SHIFT = 72;
const CLEARANCE = 3;
const MAX_PASSES = 3;

export function calculateCameraAvoidance(
    camera: Point3,
    target: Point3,
    obstacles: readonly CameraObstacle[]
): Point3 {
    let offset = { x: 0, y: 0, z: 0 };

    for (let pass = 0; pass < MAX_PASSES; pass += 1) {
        const shiftedCamera = add(camera, offset);
        const view = subtract(target, shiftedCamera);
        const viewLengthSquared = lengthSquared(view);
        if (viewLengthSquared < 0.001) break;

        let correction: Point3 | undefined;
        let largestCorrection = 0;

        for (const obstacle of obstacles) {
            if (obstacle.radius <= 0) continue;
            const cameraToObstacle = subtract(obstacle.position, shiftedCamera);
            const progress = dot(cameraToObstacle, view) / viewLengthSquared;
            if (progress <= 0.08 || progress >= 0.95) continue;

            const closestPoint = add(shiftedCamera, scale(view, progress));
            const obstacleOffset = subtract(obstacle.position, closestPoint);
            const separation = Math.sqrt(lengthSquared(obstacleOffset));
            const penetration = obstacle.radius + CLEARANCE - separation;
            if (penetration <= 0) continue;

            const movementAtObstacle = Math.max(0.2, 1 - progress);
            const requiredCorrection = penetration / movementAtObstacle;
            if (requiredCorrection <= largestCorrection) continue;

            const away =
                separation > 0.001
                    ? scale(obstacleOffset, -1 / separation)
                    : perpendicularDirection(view);
            correction = scale(away, requiredCorrection);
            largestCorrection = requiredCorrection;
        }

        if (!correction) break;
        offset = clampLength(add(offset, correction), MAX_CAMERA_SHIFT);
        if (lengthSquared(offset) >= MAX_CAMERA_SHIFT * MAX_CAMERA_SHIFT) break;
    }

    return offset;
}

function perpendicularDirection(direction: Point3) {
    const reference =
        Math.abs(direction.y) < Math.sqrt(lengthSquared(direction)) * 0.9
            ? { x: 0, y: 1, z: 0 }
            : { x: 1, y: 0, z: 0 };
    return normalize(cross(direction, reference));
}

function add(left: Point3, right: Point3): Point3 {
    return { x: left.x + right.x, y: left.y + right.y, z: left.z + right.z };
}

function subtract(left: Point3, right: Point3): Point3 {
    return { x: left.x - right.x, y: left.y - right.y, z: left.z - right.z };
}

function scale(point: Point3, factor: number): Point3 {
    return { x: point.x * factor, y: point.y * factor, z: point.z * factor };
}

function dot(left: Point3, right: Point3) {
    return left.x * right.x + left.y * right.y + left.z * right.z;
}

function cross(left: Point3, right: Point3): Point3 {
    return {
        x: left.y * right.z - left.z * right.y,
        y: left.z * right.x - left.x * right.z,
        z: left.x * right.y - left.y * right.x,
    };
}

function lengthSquared(point: Point3) {
    return dot(point, point);
}

function normalize(point: Point3) {
    const length = Math.sqrt(lengthSquared(point));
    return length < 0.001 ? { x: 1, y: 0, z: 0 } : scale(point, 1 / length);
}

function clampLength(point: Point3, maximum: number) {
    const length = Math.sqrt(lengthSquared(point));
    return length > maximum ? scale(point, maximum / length) : point;
}
