import type { ChannelNode } from "../core/channelGraph";

export interface ActivityChannel {
    id: string;
    name: string;
    heat: number;
    color: string;
}

export function rankActivityChannels(nodes: readonly ChannelNode[], limit = 5): ActivityChannel[] {
    if (limit <= 0) return [];

    return nodes
        .filter(node => node.id !== "grand_root" && node.relativeScore > 0)
        .toSorted(
            (left, right) =>
                right.relativeScore - left.relativeScore ||
                right.currentScore - left.currentScore ||
                left.name.localeCompare(right.name, undefined, {
                    numeric: true,
                    sensitivity: "base",
                })
        )
        .slice(0, limit)
        .map(node => ({
            id: node.id,
            name: node.name,
            heat: Math.round(Math.min(1, Math.max(0, node.relativeScore)) * 100),
            color: node.color,
        }));
}
