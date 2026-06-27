import { describe, expect, test } from "bun:test";

import { calculateCameraAvoidance } from "../src/core/cameraAvoidance";

describe("calculateCameraAvoidance", () => {
    test("does not move the camera when the view is clear", () => {
        expect(
            calculateCameraAvoidance({ x: 0, y: 0, z: 100 }, { x: 0, y: 0, z: 0 }, [
                { position: { x: 30, y: 0, z: 50 }, radius: 8 },
            ])
        ).toEqual({ x: 0, y: 0, z: 0 });
    });

    test("moves laterally by only the amount needed to clear an obstacle", () => {
        const offset = calculateCameraAvoidance({ x: 0, y: 0, z: 100 }, { x: 0, y: 0, z: 0 }, [
            { position: { x: 3, y: 0, z: 50 }, radius: 8 },
        ]);

        expect(Math.hypot(offset.x, offset.y, offset.z)).toBeGreaterThan(0);
        expect(Math.hypot(offset.x, offset.y, offset.z)).toBeLessThan(30);
        expect(Math.abs(offset.z)).toBeLessThan(1);
    });

    test("caps movement for obstacles close to the target", () => {
        const offset = calculateCameraAvoidance({ x: 0, y: 0, z: 100 }, { x: 0, y: 0, z: 0 }, [
            { position: { x: 0, y: 0, z: 8 }, radius: 30 },
        ]);

        expect(Math.hypot(offset.x, offset.y, offset.z)).toBeLessThanOrEqual(72);
    });
});
