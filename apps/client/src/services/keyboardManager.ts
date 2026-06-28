import type { KeyboardController, KeyboardNavigationTarget } from "../core/keyboardController";

const ARROW_TO_TARGET = {
    ArrowUp: "childId",
    ArrowDown: "parentId",
    ArrowLeft: "previousSiblingId",
    ArrowRight: "nextSiblingId",
} as const satisfies Record<string, KeyboardNavigationTarget>;

type ArrowKey = keyof typeof ARROW_TO_TARGET;

export class KeyboardManager {
    constructor(private readonly controller: KeyboardController) {}

    start(): void {
        window.addEventListener("keydown", this.handleKeyDown);
    }

    stop(): void {
        window.removeEventListener("keydown", this.handleKeyDown);
    }

    private readonly handleKeyDown = (event: KeyboardEvent): void => {
        if (event.key === "Escape") {
            event.preventDefault();
            this.controller.handleEscape();
            return;
        }

        if (isEditableEventTarget(event.target)) return;

        if (isMuteKey(event)) {
            event.preventDefault();
            this.controller.toggleMute();
            return;
        }

        if (!isArrowKey(event.key)) return;

        if (this.controller.navigate(ARROW_TO_TARGET[event.key])) {
            event.preventDefault();
        }
    };
}

function isArrowKey(key: string): key is ArrowKey {
    return key in ARROW_TO_TARGET;
}

function isMuteKey(event: KeyboardEvent): boolean {
    return (
        event.key.toLowerCase() === "m" &&
        !event.repeat &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey
    );
}

function isEditableEventTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) return false;

    return (
        target.isContentEditable ||
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement
    );
}
