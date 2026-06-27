import type { ChannelNode } from "../core/channelGraph";
import type { LayoutNode } from "../core/layout";

export async function calculateChannelLayout(nodes: readonly ChannelNode[]) {
    const input = createLayoutInput(nodes);
    if (typeof Worker === "undefined") return await calculateFallback(input);

    return await new Promise<Float32Array>((resolve, reject) => {
        const worker = new Worker(new URL("../workers/layout.worker.ts", import.meta.url), {
            type: "module",
        });
        worker.onmessage = (event: MessageEvent<Float32Array>) => {
            worker.terminate();
            resolve(event.data);
        };
        worker.onerror = event => {
            worker.terminate();
            reject(new Error(event.message || "layout worker failed"));
        };
        worker.postMessage(input);
    }).catch(() => calculateFallback(input));
}

async function calculateFallback(input: LayoutNode[]) {
    const { calculateLayout } = await import("../core/layout");
    return calculateLayout(input);
}

function createLayoutInput(nodes: readonly ChannelNode[]): LayoutNode[] {
    const indexById = new Map(nodes.map(node => [node.id, node.index]));
    return nodes.map(node => ({
        index: node.index,
        parentIndex: node.parentId ? (indexById.get(node.parentId) ?? -1) : -1,
        children: [...node.children],
        depth: node.depth,
        islandId: node.islandId,
        isLayoutActive: node.isLayoutActive,
        isExpansionOrigin: node.isExpansionOrigin,
        emphasis: node.emphasis,
        relativeScore: node.relativeScore,
        x: node.targetX,
        y: node.targetY,
        z: node.targetZ,
    }));
}
