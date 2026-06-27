import { calculateLayout } from "../core/layout";
import type { LayoutNode } from "../core/layout";

self.onmessage = (event: MessageEvent<LayoutNode[]>) => {
    const positions = calculateLayout(event.data);
    self.postMessage(positions, { transfer: [positions.buffer] });
};
