export interface Point3 {
    x: number;
    y: number;
    z: number;
}

export interface CameraFramePoint {
    position: Point3;
    radius: number;
}

export interface CameraFrameOptions {
    cameraDirection: Point3;
    verticalFovDegrees: number;
    aspect: number;
    viewportMargin?: number;
    minimumDistance?: number;
}

const DEFAULT_VIEWPORT_MARGIN = 0.18;
const MINIMUM_TANGENT = 0.001;

export function calculateFramedCameraDistance(
    target: Point3,
    points: readonly CameraFramePoint[],
    options: CameraFrameOptions
) {
    const direction = normalize(options.cameraDirection);
    const basis = createCameraBasis(direction);
    const verticalTangent =
        Math.tan((Math.max(1, options.verticalFovDegrees) * Math.PI) / 360) *
        viewportScale(options.viewportMargin);
    const horizontalTangent = verticalTangent * Math.max(0.01, options.aspect);
    let distance = options.minimumDistance ?? 0;

    for (const point of points) {
        const offset = subtract(point.position, target);
        const alongViewDirection = dot(offset, direction);
        const horizontal = Math.abs(dot(offset, basis.right)) + Math.max(0, point.radius);
        const vertical = Math.abs(dot(offset, basis.up)) + Math.max(0, point.radius);

        distance = Math.max(
            distance,
            alongViewDirection + horizontal / Math.max(MINIMUM_TANGENT, horizontalTangent),
            alongViewDirection + vertical / Math.max(MINIMUM_TANGENT, verticalTangent)
        );
    }

    return distance;
}

function viewportScale(margin = DEFAULT_VIEWPORT_MARGIN) {
    return Math.max(0.05, 1 - Math.min(0.45, Math.max(0, margin)));
}

function createCameraBasis(direction: Point3) {
    const reference =
        Math.abs(direction.y) < 0.92 ? { x: 0, y: 1, z: 0 } : { x: 1, y: 0, z: 0 };
    const right = normalize(cross(reference, direction));
    const up = normalize(cross(direction, right));
    return { right, up };
}

function subtract(left: Point3, right: Point3): Point3 {
    return { x: left.x - right.x, y: left.y - right.y, z: left.z - right.z };
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

function normalize(point: Point3) {
    const length = Math.hypot(point.x, point.y, point.z);
    if (length < 0.001) return { x: 0, y: 0, z: 1 };
    return { x: point.x / length, y: point.y / length, z: point.z / length };
}
