import { describe, expect, test } from "bun:test";

import { calculateCameraViewportShift } from "../src/core/cameraViewport";

describe("calculateCameraViewportShift", () => {
    test("allows the grand root outside while part of the tree remains visible", () => {
        const shift = calculateCameraViewportShift(
            [
                { x: -1.4, y: 0 },
                { x: 0.4, y: 0.3 },
            ],
            0.92
        );

        expect(shift).toEqual({ x: 0, y: 0 });
    });

    test("moves the tree back when all projected points leave the viewport", () => {
        const shift = calculateCameraViewportShift(
            [
                { x: 1.2, y: -0.3 },
                { x: 1.6, y: 0.5 },
            ],
            0.92
        );

        expect(shift.x).toBeCloseTo(-0.28);
        expect(shift.y).toBe(0);
    });

    test("corrects each axis independently", () => {
        const shift = calculateCameraViewportShift(
            [
                { x: -1.4, y: 1.3 },
                { x: -1.1, y: 1.6 },
            ],
            0.92
        );

        expect(shift.x).toBeCloseTo(0.18);
        expect(shift.y).toBeCloseTo(-0.38);
    });
});
