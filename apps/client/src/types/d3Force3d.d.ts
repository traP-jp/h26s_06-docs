declare module "d3-force-3d" {
    export interface SimulationNodeDatum {
        index?: number;
        x?: number;
        y?: number;
        z?: number;
        vx?: number;
        vy?: number;
        vz?: number;
        fx?: number | null;
        fy?: number | null;
        fz?: number | null;
    }

    export interface SimulationLinkDatum<Node extends SimulationNodeDatum> {
        source: Node | number | string;
        target: Node | number | string;
        index?: number;
    }

    type NodeAccessor<Node, Value> = (node: Node, index: number, nodes: Node[]) => Value;
    type LinkAccessor<Link, Value> = (link: Link, index: number, links: Link[]) => Value;

    export interface Force<Node extends SimulationNodeDatum> {
        (alpha: number): void;
        initialize?: (nodes: Node[], random: () => number, dimensions: number) => void;
    }

    export interface ForceSimulation<Node extends SimulationNodeDatum> {
        stop(): this;
        tick(iterations?: number): this;
        alpha(value: number): this;
        alphaDecay(value: number): this;
        velocityDecay(value: number): this;
        force(name: string, force: Force<Node> | null): this;
    }

    export interface ForceLink<
        Node extends SimulationNodeDatum,
        Link extends SimulationLinkDatum<Node>,
    > extends Force<Node> {
        id(accessor: NodeAccessor<Node, string | number>): this;
        distance(distance: number | LinkAccessor<Link, number>): this;
        strength(strength: number | LinkAccessor<Link, number>): this;
        iterations(iterations: number): this;
    }

    export interface ForceManyBody<Node extends SimulationNodeDatum> extends Force<Node> {
        strength(strength: number | NodeAccessor<Node, number>): this;
        distanceMax(distance: number): this;
        theta(theta: number): this;
    }

    export interface ForceCollide<Node extends SimulationNodeDatum> extends Force<Node> {
        radius(radius: number | NodeAccessor<Node, number>): this;
        strength(strength: number): this;
        iterations(iterations: number): this;
    }

    export interface ForcePosition<Node extends SimulationNodeDatum> extends Force<Node> {
        strength(strength: number | NodeAccessor<Node, number>): this;
    }

    export function forceSimulation<Node extends SimulationNodeDatum>(
        nodes?: Node[],
        dimensions?: number
    ): ForceSimulation<Node>;
    export function forceLink<
        Node extends SimulationNodeDatum,
        Link extends SimulationLinkDatum<Node>,
    >(links?: Link[]): ForceLink<Node, Link>;
    export function forceManyBody<Node extends SimulationNodeDatum>(): ForceManyBody<Node>;
    export function forceCollide<Node extends SimulationNodeDatum>(): ForceCollide<Node>;
    export function forceX<Node extends SimulationNodeDatum>(
        x?: number | NodeAccessor<Node, number>
    ): ForcePosition<Node>;
    export function forceY<Node extends SimulationNodeDatum>(
        y?: number | NodeAccessor<Node, number>
    ): ForcePosition<Node>;
    export function forceZ<Node extends SimulationNodeDatum>(
        z?: number | NodeAccessor<Node, number>
    ): ForcePosition<Node>;
}
