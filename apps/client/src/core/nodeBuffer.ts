import {
    ACTIVE_RELATIVE_SCORE_THRESHOLD,
    type ChannelDisplayMode,
    type ChannelNode,
} from "./channelGraph";

const MATRIX_SIZE = 16;
const COLOR_SIZE = 3;
const ACTIVE_ANCESTOR_VISUAL_WEIGHT = 0.42;
const ALL_CHANNEL_NODE_SCALE = 1.42;

/**
 * ChannelGraph と GPU の境界。行列と色を連続した TypedArray に保持し、
 * Three.js の InstancedBufferAttribute へ一括転送する。
 */
export class NodeBuffer {
    readonly matrixData: Float32Array;
    readonly colorData: Float32Array;

    constructor(readonly count: number) {
        this.matrixData = new Float32Array(count * MATRIX_SIZE);
        this.colorData = new Float32Array(count * COLOR_SIZE);
    }

    update(
        nodes: readonly ChannelNode[],
        now: number,
        selectedId?: string,
        activeOnly = false,
        displayMode: ChannelDisplayMode = "collapsed"
    ) {
        const displayModeScale = displayMode === "all" ? ALL_CHANNEL_NODE_SCALE : 1;
        for (let index = 0; index < nodes.length; index += 1) {
            const node = nodes[index];
            if (!node) continue;
            const heat = node.relativeScore;
            const pulse = heat > 0.72 ? Math.sin(now * 0.008) * 0.18 : 0;
            const selectedScale = node.id === selectedId ? 1.8 : 1;
            const visible =
                !activeOnly ||
                node.relativeScore > ACTIVE_RELATIVE_SCORE_THRESHOLD ||
                node.activeDescendantScore > 0 ||
                node.id === "grand_root" ||
                node.id === selectedId;
            const activeAncestorOnly =
                activeOnly &&
                node.activeDescendantScore > 0 &&
                node.relativeScore <= ACTIVE_RELATIVE_SCORE_THRESHOLD &&
                node.id !== "grand_root" &&
                node.id !== selectedId;
            const displayWeight = activeAncestorOnly ? ACTIVE_ANCESTOR_VISUAL_WEIGHT : 1;
            const baseScale =
                node.id === "grand_root"
                    ? 4.2
                    : node.depth <= 1
                      ? 3
                      : Math.max(0.42, 2.4 * 0.72 ** (node.depth - 1));
            const scale =
                (baseScale + heat * 6) *
                (1 + pulse) *
                selectedScale *
                node.emphasis *
                displayWeight *
                displayModeScale *
                Number(visible) *
                node.visibilityAlpha;

            const waverX = Math.sin(now * 0.0008 + index * 1.2) * 1.5;
            const waverY = Math.cos(now * 0.0009 + index * 0.8) * 1.5;
            const waverZ = Math.sin(now * 0.0007 + index * 1.5) * 1.5;
            writeMatrix(this.matrixData, index * MATRIX_SIZE, node, scale, waverX, waverY, waverZ);
            writeColor(
                this.colorData,
                index * COLOR_SIZE,
                node.color,
                heat,
                node.emphasis * displayWeight
            );
        }
    }
}

function writeMatrix(
    target: Float32Array,
    offset: number,
    node: ChannelNode,
    scale: number,
    wx: number,
    wy: number,
    wz: number
) {
    target[offset] = scale;
    target[offset + 1] = 0;
    target[offset + 2] = 0;
    target[offset + 3] = 0;
    target[offset + 4] = 0;
    target[offset + 5] = scale;
    target[offset + 6] = 0;
    target[offset + 7] = 0;
    target[offset + 8] = 0;
    target[offset + 9] = 0;
    target[offset + 10] = scale;
    target[offset + 11] = 0;
    target[offset + 12] = node.x + wx;
    target[offset + 13] = node.y + wy;
    target[offset + 14] = node.z + wz;
    target[offset + 15] = 1;
}

function writeColor(
    target: Float32Array,
    offset: number,
    hex: string,
    heat: number,
    emphasis: number
) {
    const value = Number.parseInt(hex.slice(1), 16);
    const brightness = (0.9 + Math.min(heat, 0.1)) * emphasis;
    target[offset] = srgbToLinear((value >> 16) / 255) * brightness;
    target[offset + 1] = srgbToLinear(((value >> 8) & 255) / 255) * brightness;
    target[offset + 2] = srgbToLinear((value & 255) / 255) * brightness;
}

function srgbToLinear(value: number) {
    return value <= 0.040_45 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4);
}
