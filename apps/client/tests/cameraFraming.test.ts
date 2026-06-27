import { describe, expect, test } from "bun:test";

import { calculateCameraFrame } from "../src/core/cameraFraming";

describe("calculateCameraFrame", () => {
    test("centers the target on the expanded nodes", () => {
        const frame = calculateCameraFrame(
            [
                { x: 20, y: -40, z: 0 },
                { x: 180, y: 80, z: 0 },
            ],
            { x: 0, y: 0, z: 1 },
            52,
            16 / 9
        );

        expect(frame.target.x).toBeCloseTo(100);
        expect(frame.target.y).toBeCloseTo(20);
        expect(frame.target.z).toBeCloseTo(0);
    });

    test("uses the horizontal field of view on a narrow viewport", () => {
        const points = [
            { x: -100, y: 0, z: 0 },
            { x: 100, y: 0, z: 0 },
        ];
        const wide = calculateCameraFrame(points, { x: 0, y: 0, z: 1 }, 52, 2);
        const narrow = calculateCameraFrame(points, { x: 0, y: 0, z: 1 }, 52, 0.5);

        expect(narrow.distance).toBeGreaterThan(wide.distance * 2);
    });

    test("accounts for nodes closer to the camera", () => {
        const flat = calculateCameraFrame(
            [
                { x: -80, y: 0, z: 0 },
                { x: 80, y: 0, z: 0 },
            ],
            { x: 0, y: 0, z: 1 },
            52,
            1
        );
        const deep = calculateCameraFrame(
            [
                { x: -80, y: 0, z: 80 },
                { x: 80, y: 0, z: -80 },
            ],
            { x: 0, y: 0, z: 1 },
            52,
            1
        );

        expect(deep.distance).toBeGreaterThan(flat.distance);
    });

    test("does not pull back for a large number of tightly grouped nodes", () => {
        const sparse = calculateCameraFrame([{ x: 0, y: 0, z: 0 }], { x: 0, y: 0, z: 1 }, 52, 1);
        const dense = calculateCameraFrame(
            Array.from({ length: 100 }, () => ({ x: 0, y: 0, z: 0 })),
            { x: 0, y: 0, z: 1 },
            52,
            1
        );

        expect(dense.distance).toBe(sparse.distance);
    });

    test("uses the selected node as the camera target while fitting its children", () => {
        const target = { x: 20, y: -10, z: 5 };
        const frame = calculateCameraFrame(
            [target, { x: 180, y: 80, z: 0 }],
            { x: 0, y: 0, z: 1 },
            52,
            16 / 9,
            target
        );

        expect(frame.target).toEqual(target);
        expect(frame.distance).toBeGreaterThan(160);
    });
});
