import type { ChannelDictionary, TriggerPayload } from "../types/api";

const PALETTE = [
    "#00bfff",
    "#655cff",
    "#ec3fa5",
    "#ff633c",
    "#f4b400",
    "#20c878",
    "#168cff",
    "#a33ce8",
    "#e8f0ff",
];
const DENSE_CHILD_THRESHOLD = 24;
const DENSE_EMPHASIZED_CHILDREN = 12;
const CONDENSED_EMPHASIS = 0.22;
const ANCESTOR_SCORE_FACTOR = 0.45;
const SCORE_DECAY_TIME_SCALE = 300;
const RELATIVE_SCORE_SCALE_FLOOR = 2.2;
export const ACTIVE_RELATIVE_SCORE_THRESHOLD = 0.08;

export interface ChannelNode {
    index: number;
    id: string;
    name: string;
    parentId: string | null;
    children: number[];
    islandId: number;
    depth: number;
    currentScore: number;
    targetScore: number;
    relativeScore: number;
    x: number;
    y: number;
    z: number;
    targetX: number;
    targetY: number;
    targetZ: number;
    visibilityAlpha: number;
    isLayoutActive: boolean;
    isExpansionOrigin: boolean;
    emphasis: number;
    activeDescendantScore: number;
    color: string;
}

export type VisualEvent =
    | { type: "message"; channelId: string }
    | { type: "mov"; fromId?: string; toId: string };

export class ChannelGraph {
    readonly nodes: ChannelNode[];
    private readonly nodeMap = new Map<string, number>();
    private readonly parentIndices: Int32Array;
    private readonly visualEvents: VisualEvent[] = [];
    private readonly pendingMessageRevealIds = new Set<string>();
    private snapNextSync = false;
    private scoreScale = RELATIVE_SCORE_SCALE_FLOOR;

    constructor(channels: ChannelDictionary) {
        const ordered = orderChannels(channels);
        this.scoreScale = initialScoreScale(ordered);

        this.nodes = ordered.map((channel, index) => {
            const score = sanitizeScore(channel.score);
            this.nodeMap.set(channel.id, index);
            return {
                index,
                id: channel.id,
                name: channel.name || channel.id,
                parentId: channel.parentId || null,
                children: [],
                islandId: channel.islandId ?? -1,
                depth: channel.depth ?? 0,
                currentScore: score,
                targetScore: score,
                relativeScore: Math.min(1, score / this.scoreScale),
                x: 0,
                y: 0,
                z: 0,
                targetX: 0,
                targetY: 0,
                targetZ: 0,
                visibilityAlpha: 0,
                isLayoutActive: false,
                isExpansionOrigin: false,
                emphasis: 1,
                activeDescendantScore: 0,
                color:
                    channel.id === "grand_root"
                        ? "#ffffff"
                        : PALETTE[Math.max(0, channel.islandId ?? 0) % PALETTE.length]!,
            };
        });

        this.parentIndices = new Int32Array(this.nodes.length);
        this.parentIndices.fill(-1);

        for (const channel of ordered) {
            const node = this.get(channel.id);
            if (!node) continue;
            if (node.parentId) {
                this.parentIndices[node.index] = this.nodeMap.get(node.parentId) ?? -1;
            }
            node.children = channel.children
                ?.map(id => this.nodeMap.get(id))
                .filter((index): index is number => index !== undefined);
        }
        this.updateActiveDescendantScores();
    }

    get(id: string) {
        const index = this.nodeMap.get(id);
        return index === undefined ? undefined : this.nodes[index];
    }

    path(id: string) {
        const result: ChannelNode[] = [];
        let node = this.get(id);
        while (node) {
            result.unshift(node);
            node = node.parentId ? this.get(node.parentId) : undefined;
        }
        return result;
    }

    navigationTargets(id: string, preferredChildId?: string) {
        const node = this.get(id);
        if (!node) {
            return {};
        }

        const parent = node.parentId ? this.get(node.parentId) : undefined;
        const children = this.sortedNodes(node.children);
        const siblings = parent ? this.sortedNodes(parent.children) : [];
        const siblingIndex = siblings.findIndex(sibling => sibling.id === node.id);
        const preferredChild = preferredChildId
            ? children.find(child => child.id === preferredChildId)
            : undefined;

        return {
            parentId: parent?.id,
            childId: preferredChild?.id ?? children[0]?.id,
            previousSiblingId:
                siblingIndex >= 0 && siblings.length > 1
                    ? siblings[(siblingIndex - 1 + siblings.length) % siblings.length]?.id
                    : undefined,
            nextSiblingId:
                siblingIndex >= 0 && siblings.length > 1
                    ? siblings[(siblingIndex + 1) % siblings.length]?.id
                    : undefined,
        };
    }

    applyTrigger(trigger: TriggerPayload) {
        const id = trigger.type === "msg" ? trigger.ch : trigger.to;
        let visibilityChanged = false;

        const getActiveAncestor = (nodeId?: string) => {
            let n = nodeId ? this.get(nodeId) : undefined;
            while (n && !n.isLayoutActive) {
                n = n.parentId ? this.get(n.parentId) : undefined;
            }
            return n?.id;
        };

        const effectiveFrom = getActiveAncestor(trigger.type === "mov" ? trigger.from : undefined);
        const effectiveTo = getActiveAncestor(trigger.type === "mov" ? trigger.to : undefined);

        if (trigger.type === "msg" && this.get(trigger.ch)) {
            visibilityChanged = this.prepareMessagePath(trigger.ch);
            this.enqueueVisualEvent({ type: "message", channelId: trigger.ch });
        } else if (
            trigger.type === "mov" &&
            effectiveFrom &&
            effectiveTo &&
            effectiveFrom !== effectiveTo
        ) {
            this.enqueueVisualEvent({ type: "mov", fromId: effectiveFrom, toId: effectiveTo });
        }

        let node = this.get(id);
        let heat = trigger.delta ?? 0;
        while (node) {
            node.currentScore += heat;
            node.targetScore += heat;
            heat *= ANCESTOR_SCORE_FACTOR;
            node = node.parentId ? this.get(node.parentId) : undefined;
        }

        return visibilityChanged;
    }

    sync(deltas: Record<string, number>) {
        const comparisons: {
            channelId: string;
            clientValue: number;
            trueValue: number;
            difference: number;
        }[] = [];

        for (const [id, score] of Object.entries(deltas)) {
            const node = this.get(id);
            if (!node) continue;
            comparisons.push({
                channelId: id,
                clientValue: node.currentScore,
                trueValue: score,
                difference: score - node.currentScore,
            });
            node.targetScore = score;
            if (this.snapNextSync) node.currentScore = score;
        }
        console.table(comparisons);
        this.snapNextSync = false;
    }

    requestSyncSnap() {
        this.snapNextSync = true;
    }

    takeVisualEvents() {
        return this.visualEvents.splice(0);
    }

    clearVisualEvents() {
        this.visualEvents.length = 0;
        this.pendingMessageRevealIds.clear();
    }

    revealMessageNode(id: string) {
        return this.pendingMessageRevealIds.delete(id);
    }

    applyLayout(positions: Float32Array, isInitial = false) {
        if (positions.length !== this.nodes.length * 3) {
            throw new Error("Layout position count does not match channel count");
        }
        for (const node of this.nodes) {
            const offset = node.index * 3;
            node.targetX = positions[offset] ?? 0;
            node.targetY = positions[offset + 1] ?? 0;
            node.targetZ = positions[offset + 2] ?? 0;
            if (isInitial) {
                node.x = node.targetX;
                node.y = node.targetY;
                node.z = node.targetZ;
            }
        }
    }

    private readonly clickedIds = new Set<string>();
    private emphasisGeneration = 0;
    private lastVisibilitySelection: string | null | undefined;

    updateVisibility(selectedId?: string, k: number = 3) {
        let changed = false;
        const activePaths = new Set<string>();
        const requiredEmphasisIds = new Set<string>();
        const normalizedSelection = selectedId ?? null;
        if (this.lastVisibilitySelection !== normalizedSelection) {
            this.lastVisibilitySelection = normalizedSelection;
            this.emphasisGeneration += 1;
        }

        if (!selectedId) {
            this.clickedIds.clear();
        } else {
            const path = this.path(selectedId);
            const pathIds = new Set(path.map(p => p.id));
            for (const id of pathIds) this.pendingMessageRevealIds.delete(id);

            for (const id of this.clickedIds) {
                if (!pathIds.has(id)) {
                    this.clickedIds.delete(id);
                }
            }
            this.clickedIds.add(selectedId);

            for (const p of path) {
                activePaths.add(p.id);
                requiredEmphasisIds.add(p.id);
            }

            const traverse = (nodeIndex: number, level: number) => {
                if (level > 1) return;
                const node = this.nodes[nodeIndex];
                if (!node) return;
                activePaths.add(node.id);
                for (const childIndex of node.children) {
                    traverse(childIndex, level + 1);
                }
            };

            for (const id of this.clickedIds) {
                const node = this.get(id);
                if (node) traverse(node.index, 0);
            }
        }

        const scoreActiveAncestorIds = new Set<string>();
        for (const node of this.nodes) {
            if (node.relativeScore <= ACTIVE_RELATIVE_SCORE_THRESHOLD) continue;
            let parent = node.parentId ? this.get(node.parentId) : undefined;
            while (parent) {
                scoreActiveAncestorIds.add(parent.id);
                parent = parent.parentId ? this.get(parent.parentId) : undefined;
            }
        }

        const emphasizedIds = this.pickDenseChildren(requiredEmphasisIds);
        for (const node of this.nodes) {
            const isExpansionOrigin = node.id === selectedId && node.children.length > 0;
            if (node.isExpansionOrigin !== isExpansionOrigin) {
                node.isExpansionOrigin = isExpansionOrigin;
                changed = true;
            }

            let shouldBeActive = false;
            if (this.pendingMessageRevealIds.has(node.id)) {
                shouldBeActive = true;
            } else if (node.depth < k) {
                shouldBeActive = true;
            } else if (node.relativeScore > ACTIVE_RELATIVE_SCORE_THRESHOLD) {
                shouldBeActive = true;
            } else if (scoreActiveAncestorIds.has(node.id)) {
                shouldBeActive = true;
            } else if (activePaths.has(node.id)) {
                shouldBeActive = true;
            } else if (node.id === "grand_root") {
                shouldBeActive = true;
            }

            if (node.isLayoutActive !== shouldBeActive) {
                if (shouldBeActive && !node.isLayoutActive) {
                    const parent = node.parentId ? this.get(node.parentId) : undefined;
                    if (parent) {
                        node.x = parent.x;
                        node.y = parent.y;
                        node.z = parent.z;
                        node.targetX = parent.targetX;
                        node.targetY = parent.targetY;
                        node.targetZ = parent.targetZ;
                    }
                } else if (!shouldBeActive && node.isLayoutActive) {
                    this.pendingMessageRevealIds.delete(node.id);
                    const parent = node.parentId ? this.get(node.parentId) : undefined;
                    if (parent) {
                        node.targetX = parent.targetX;
                        node.targetY = parent.targetY;
                        node.targetZ = parent.targetZ;
                    }
                }
                node.isLayoutActive = shouldBeActive;
                changed = true;
            }

            const emphasis = emphasizedIds.has(node.id) ? 1 : CONDENSED_EMPHASIS;
            if (node.emphasis !== emphasis) {
                node.emphasis = emphasis;
                changed = true;
            }
        }
        this.updateActiveDescendantScores();
        return changed;
    }

    private updateActiveDescendantScores() {
        for (const node of this.nodes) {
            node.activeDescendantScore =
                node.relativeScore > ACTIVE_RELATIVE_SCORE_THRESHOLD ? node.relativeScore : 0;
        }

        for (let index = this.nodes.length - 1; index >= 0; index -= 1) {
            const node = this.nodes[index];
            if (!node) continue;
            const parentIndex = this.parentIndices[index] ?? -1;
            if (parentIndex < 0) continue;
            const parent = this.nodes[parentIndex];
            if (!parent || node.activeDescendantScore <= parent.activeDescendantScore) continue;
            parent.activeDescendantScore = node.activeDescendantScore;
        }
    }

    private pickDenseChildren(requiredIds: ReadonlySet<string>) {
        const emphasizedIds = new Set(this.nodes.map(node => node.id));

        for (const parent of this.nodes) {
            if (parent.children.length <= DENSE_CHILD_THRESHOLD) continue;

            const candidates = parent.children
                .map(index => this.nodes[index])
                .filter((node): node is ChannelNode => node !== undefined)
                .sort(
                    (left, right) =>
                        emphasisRank(left.index, parent.index, this.emphasisGeneration) -
                        emphasisRank(right.index, parent.index, this.emphasisGeneration)
                );

            for (const child of candidates) emphasizedIds.delete(child.id);
            for (const child of candidates.slice(0, DENSE_EMPHASIZED_CHILDREN)) {
                emphasizedIds.add(child.id);
            }
            for (const child of candidates) {
                if (
                    requiredIds.has(child.id) ||
                    child.relativeScore > ACTIVE_RELATIVE_SCORE_THRESHOLD
                ) {
                    emphasizedIds.add(child.id);
                }
            }
        }

        return emphasizedIds;
    }

    private prepareMessagePath(channelId: string) {
        let changed = false;
        for (const node of this.path(channelId)) {
            if (node.isLayoutActive) continue;

            const parent = node.parentId ? this.get(node.parentId) : undefined;
            if (parent) {
                node.x = parent.x;
                node.y = parent.y;
                node.z = parent.z;
                node.targetX = parent.targetX;
                node.targetY = parent.targetY;
                node.targetZ = parent.targetZ;
            }
            node.isLayoutActive = true;
            this.pendingMessageRevealIds.add(node.id);
            changed = true;
        }
        return changed;
    }

    update(deltaSeconds: number) {
        const decay = Math.exp(-deltaSeconds / SCORE_DECAY_TIME_SCALE);
        const blend = 1 - Math.exp(-deltaSeconds * 3.5);
        const spatialBlend = 1 - Math.exp(-deltaSeconds * 6.0);

        let observedMaxScore = RELATIVE_SCORE_SCALE_FLOOR;
        for (const node of this.nodes) {
            node.currentScore *= decay;
            node.targetScore *= decay;
            node.currentScore += (node.targetScore - node.currentScore) * blend;
            if (node.currentScore < 0.01) node.currentScore = 0;
            if (node.targetScore < 0.01) node.targetScore = 0;
            observedMaxScore = Math.max(observedMaxScore, node.currentScore, node.targetScore);
        }

        const scaleBlend =
            observedMaxScore > this.scoreScale
                ? 1 - Math.exp(-deltaSeconds * 8.0)
                : 1 - Math.exp(-deltaSeconds / SCORE_DECAY_TIME_SCALE);
        this.scoreScale += (observedMaxScore - this.scoreScale) * scaleBlend;

        for (const node of this.nodes) {
            node.relativeScore = Math.min(1, node.currentScore / this.scoreScale);
        }
        this.updateActiveDescendantScores();

        for (const node of this.nodes) {
            node.x += (node.targetX - node.x) * spatialBlend;
            node.y += (node.targetY - node.y) * spatialBlend;
            node.z += (node.targetZ - node.z) * spatialBlend;

            const targetAlpha =
                node.isLayoutActive && !this.pendingMessageRevealIds.has(node.id) ? 1.0 : 0.0;
            const alphaBlend =
                targetAlpha < node.visibilityAlpha
                    ? 1 - Math.exp(-deltaSeconds * 24.0) // 素早くフェードアウト
                    : spatialBlend; // ゆっくりフェードイン
            node.visibilityAlpha += (targetAlpha - node.visibilityAlpha) * alphaBlend;
        }
    }

    private enqueueVisualEvent(event: VisualEvent) {
        if (this.visualEvents.length >= 128) this.visualEvents.shift();
        this.visualEvents.push(event);
    }

    private sortedNodes(indices: readonly number[]) {
        return indices
            .map(index => this.nodes[index])
            .filter((node): node is ChannelNode => node !== undefined)
            .toSorted((left, right) =>
                left.name.localeCompare(right.name, undefined, {
                    numeric: true,
                    sensitivity: "base",
                })
            );
    }
}

function emphasisRank(index: number, parentIndex: number, generation: number) {
    let value = index ^ Math.imul(parentIndex + 1, 0x9e37_79b1);
    value ^= Math.imul(generation + 1, 0x85eb_ca6b);
    value = Math.imul(value ^ (value >>> 16), 0x7feb_352d);
    value = Math.imul(value ^ (value >>> 15), 0x846c_a68b);
    return (value ^ (value >>> 16)) >>> 0;
}

function sanitizeScore(score: number | undefined): number {
    return typeof score === "number" && Number.isFinite(score) ? Math.max(0, score) : 0;
}

function initialScoreScale(channels: readonly import("../types/api").InitChannel[]): number {
    return Math.max(
        RELATIVE_SCORE_SCALE_FLOOR,
        ...channels.map(channel => sanitizeScore(channel.score))
    );
}

function orderChannels(channels: ChannelDictionary) {
    const ordered: import("../types/api").InitChannel[] = [];
    const visited = new Set<string>();
    const visit = (id: string) => {
        const channel = channels[id];
        if (!channel || visited.has(id)) return;
        visited.add(id);
        ordered.push(channel);
        channel.children.forEach(visit);
    };
    visit("grand_root");
    Object.keys(channels).forEach(visit);
    return ordered;
}
