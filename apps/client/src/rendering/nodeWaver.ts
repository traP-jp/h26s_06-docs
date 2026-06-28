import { Vector3 } from "three";

import type { ChannelNode } from "../core/channelGraph";

export function nodeWaverX(now: number, index: number) {
    return Math.sin(now * 0.0008 + index * 1.2) * 1.5;
}

export function nodeWaverY(now: number, index: number) {
    return Math.cos(now * 0.0009 + index * 0.8) * 1.5;
}

export function nodeWaverZ(now: number, index: number) {
    return Math.sin(now * 0.0007 + index * 1.5) * 1.5;
}

export function setAnimatedNodePosition(target: Vector3, node: ChannelNode, now: number) {
    target.set(
        node.x + nodeWaverX(now, node.index),
        node.y + nodeWaverY(now, node.index),
        node.z + nodeWaverZ(now, node.index)
    );
    return target;
}
