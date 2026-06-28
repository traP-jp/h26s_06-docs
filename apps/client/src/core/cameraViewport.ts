export interface CameraViewportPoint {
    x: number;
    y: number;
}

export function calculateCameraViewportShift(
    points: readonly CameraViewportPoint[],
    viewportLimit: number
) {
    if (points.length === 0) return { x: 0, y: 0 };

    let minX = Number.POSITIVE_INFINITY;
    let maxX = Number.NEGATIVE_INFINITY;
    let minY = Number.POSITIVE_INFINITY;
    let maxY = Number.NEGATIVE_INFINITY;
    for (const point of points) {
        minX = Math.min(minX, point.x);
        maxX = Math.max(maxX, point.x);
        minY = Math.min(minY, point.y);
        maxY = Math.max(maxY, point.y);
    }

    return {
        x:
            maxX < -viewportLimit
                ? -viewportLimit - maxX
                : minX > viewportLimit
                  ? viewportLimit - minX
                  : 0,
        y:
            maxY < -viewportLimit
                ? -viewportLimit - maxY
                : minY > viewportLimit
                  ? viewportLimit - minY
                  : 0,
    };
}
