import { calculateLayout } from "../core/layout";
import type { LayoutRequest } from "../core/layout";

self.onmessage = (event: MessageEvent<LayoutRequest>) => {
    const positions = calculateLayout(event.data.nodes, event.data.options);
    self.postMessage(positions, { transfer: [positions.buffer] });
};
