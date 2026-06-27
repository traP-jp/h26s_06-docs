export interface CameraFramingPoint {
    x: number;
    y: number;
    z: number;
}

interface CameraFrame {
    target: CameraFramingPoint;
    distance: number;
}

const DEFAULT_PADDING = 1.18;
const DEFAULT_MARGIN = 14;
const DEFAULT_MIN_DISTANCE = 160;

export function calculateCameraFrame(
    points: readonly CameraFramingPoint[],
    cameraDirection: CameraFramingPoint,
    verticalFieldOfViewDegrees: number,
    aspect: number,
    targetPoint?: CameraFramingPoint
): CameraFrame {
    const direction = normalize(cameraDirection, { x: 0, y: 0, z: 1 });
    const referenceUp = Math.abs(direction.y) < 0.98 ? { x: 0, y: 1, z: 0 } : { x: 0, y: 0, z: 1 };
    const right = normalize(cross(referenceUp, direction), { x: 1, y: 0, z: 0 });
    const up = cross(direction, right);
    const framePoints = points.length > 0 ? points : [{ x: 0, y: 0, z: 0 }];

    const rightRange = projectedRange(framePoints, right);
    const upRange = projectedRange(framePoints, up);
    const depthRange = projectedRange(framePoints, direction);
    const target =
        targetPoint ??
        add(
            add(scale(right, midpoint(rightRange)), scale(up, midpoint(upRange))),
            scale(direction, midpoint(depthRange))
        );

    const verticalTangent = Math.tan((verticalFieldOfViewDegrees * Math.PI) / 360);
    const horizontalTangent = verticalTangent * Math.max(aspect, 0.01);
    let distance = DEFAULT_MIN_DISTANCE;

    for (const point of framePoints) {
        const relative = subtract(point, target);
        const horizontalExtent = Math.abs(dot(relative, right)) + DEFAULT_MARGIN;
        const verticalExtent = Math.abs(dot(relative, up)) + DEFAULT_MARGIN;
        const depth = dot(relative, direction);
        distance = Math.max(
            distance,
            depth + (horizontalExtent * DEFAULT_PADDING) / horizontalTangent,
            depth + (verticalExtent * DEFAULT_PADDING) / verticalTangent
        );
    }

    return { target, distance };
}

function projectedRange(points: readonly CameraFramingPoint[], axis: CameraFramingPoint) {
    let minimum = Number.POSITIVE_INFINITY;
    let maximum = Number.NEGATIVE_INFINITY;
    for (const point of points) {
        const projection = dot(point, axis);
        minimum = Math.min(minimum, projection);
        maximum = Math.max(maximum, projection);
    }
    return { minimum, maximum };
}

function midpoint(range: { minimum: number; maximum: number }) {
    return (range.minimum + range.maximum) / 2;
}

function dot(left: CameraFramingPoint, right: CameraFramingPoint) {
    return left.x * right.x + left.y * right.y + left.z * right.z;
}

function cross(left: CameraFramingPoint, right: CameraFramingPoint): CameraFramingPoint {
    return {
        x: left.y * right.z - left.z * right.y,
        y: left.z * right.x - left.x * right.z,
        z: left.x * right.y - left.y * right.x,
    };
}

function add(left: CameraFramingPoint, right: CameraFramingPoint): CameraFramingPoint {
    return { x: left.x + right.x, y: left.y + right.y, z: left.z + right.z };
}

function subtract(left: CameraFramingPoint, right: CameraFramingPoint): CameraFramingPoint {
    return { x: left.x - right.x, y: left.y - right.y, z: left.z - right.z };
}

function scale(point: CameraFramingPoint, amount: number): CameraFramingPoint {
    return { x: point.x * amount, y: point.y * amount, z: point.z * amount };
}

function normalize(point: CameraFramingPoint, fallback: CameraFramingPoint): CameraFramingPoint {
    const length = Math.hypot(point.x, point.y, point.z);
    return length < 0.001 ? fallback : scale(point, 1 / length);
}
