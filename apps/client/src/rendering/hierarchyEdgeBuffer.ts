import { ACTIVE_RELATIVE_SCORE_THRESHOLD, type ChannelNode } from "../core/channelGraph";

/**
 * 表示対象の階層エッジを、描画ループ内で配列を生成せずに抽出する。
 * ChannelGraph のノードは親が子より前に並ぶため、末尾から活動度を伝播できる。
 */
export class HierarchyEdgeBuffer {
    readonly nodeIndices: Int32Array;
    readonly activity: Float32Array;
    readonly parentIndices: Int32Array;
    readonly capacity: number;
    count = 0;

    private readonly propagatedActivity: Float32Array;
    private showingActiveOnly: boolean | undefined;

    constructor(nodes: readonly ChannelNode[]) {
        this.parentIndices = new Int32Array(nodes.length);
        this.parentIndices.fill(-1);

        const indexById = new Map(nodes.map(node => [node.id, node.index]));
        let capacity = 0;
        for (const node of nodes) {
            if (!node.parentId) continue;
            const parentIndex = indexById.get(node.parentId);
            if (parentIndex === undefined) continue;
            this.parentIndices[node.index] = parentIndex;
            capacity += 1;
        }

        this.capacity = capacity;
        this.nodeIndices = new Int32Array(capacity);
        this.activity = new Float32Array(capacity);
        this.propagatedActivity = new Float32Array(nodes.length);
    }

    update(nodes: readonly ChannelNode[], activeOnly: boolean) {
        if (!activeOnly) {
            if (this.showingActiveOnly !== false) this.includeAllEdges(nodes);
            this.showingActiveOnly = false;
            return;
        }

        this.showingActiveOnly = true;
        this.propagatedActivity.fill(0);

        for (const node of nodes) {
            if (node.relativeScore > ACTIVE_RELATIVE_SCORE_THRESHOLD) {
                this.propagatedActivity[node.index] = node.relativeScore;
            }
        }

        for (let index = nodes.length - 1; index >= 0; index -= 1) {
            const activity = this.propagatedActivity[index] ?? 0;
            const parentIndex = this.parentIndices[index] ?? -1;
            if (
                activity > 0 &&
                parentIndex >= 0 &&
                activity > (this.propagatedActivity[parentIndex] ?? 0)
            ) {
                this.propagatedActivity[parentIndex] = activity;
            }
        }

        this.count = 0;
        for (const node of nodes) {
            if ((this.parentIndices[node.index] ?? -1) < 0) continue;
            const activity = this.propagatedActivity[node.index] ?? 0;
            if (activity <= 0) continue;
            this.nodeIndices[this.count] = node.index;
            this.activity[this.count] = activity;
            this.count += 1;
        }
    }

    private includeAllEdges(nodes: readonly ChannelNode[]) {
        this.count = 0;
        for (const node of nodes) {
            if ((this.parentIndices[node.index] ?? -1) < 0) continue;
            this.nodeIndices[this.count] = node.index;
            this.activity[this.count] = 1;
            this.count += 1;
        }
    }
}
