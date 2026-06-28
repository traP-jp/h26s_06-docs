import { describe, expect, test } from "bun:test";

import { calculateFramedCameraDistance } from "../src/core/cameraFraming";

describe("calculateFramedCameraDistance", () => {
    test("keeps the existing distance when every point already fits", () => {
        const distance = calculateFramedCameraDistance(
            { x: 0, y: 0, z: 0 },
            [
                {
                    position: { x: 8, y: 5, z: 0 },
                    radius: 4,
                },
            ],
            {
                cameraDirection: { x: 0, y: 0, z: 1 },
                verticalFovDegrees: 52,
                aspect: 16 / 9,
                minimumDistance: 120,
            }
        );

        expect(distance).toBe(120);
    });

    test("increases distance when a focused node group would sit outside the viewport", () => {
        const distance = calculateFramedCameraDistance(
            { x: 0, y: 0, z: 0 },
            [
                {
                    position: { x: 130, y: 0, z: 0 },
                    radius: 12,
                },
            ],
            {
                cameraDirection: { x: 0, y: 0, z: 1 },
                verticalFovDegrees: 52,
                aspect: 1,
                minimumDistance: 80,
            }
        );

        expect(distance).toBeGreaterThan(300);
    });

    test("accounts for points closer to the camera along the view direction", () => {
        const distance = calculateFramedCameraDistance(
            { x: 0, y: 0, z: 0 },
            [
                {
                    position: { x: 0, y: 42, z: 75 },
                    radius: 8,
                },
            ],
            {
                cameraDirection: { x: 0, y: 0, z: 1 },
                verticalFovDegrees: 52,
                aspect: 16 / 9,
                minimumDistance: 80,
            }
        );

        expect(distance).toBeGreaterThan(175);
    });
});
