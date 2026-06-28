import type {
    CameraMoveDirection,
    CameraRotationDirection,
    CameraZoomDirection,
    KeyboardController,
    KeyboardNavigationTarget,
} from "../core/keyboardController";

const ARROW_TO_TARGET = {
    ArrowUp: "childId",
    ArrowDown: "parentId",
    ArrowLeft: "previousSiblingId",
    ArrowRight: "nextSiblingId",
} as const satisfies Record<string, KeyboardNavigationTarget>;

type ArrowKey = keyof typeof ARROW_TO_TARGET;

const CAMERA_MOVE_KEYS = {
    w: "up",
    s: "down",
    a: "left",
    d: "right",
} as const satisfies Record<string, CameraMoveDirection>;

const CAMERA_ZOOM_KEYS = {
    e: "in",
    q: "out",
} as const satisfies Record<string, CameraZoomDirection>;

const CAMERA_ROTATION_KEYS = {
    i: "down",
    k: "up",
    j: "right",
    l: "left",
} as const satisfies Record<string, CameraRotationDirection>;

type CameraMoveKey = keyof typeof CAMERA_MOVE_KEYS;
type CameraZoomKey = keyof typeof CAMERA_ZOOM_KEYS;
type CameraRotationKey = keyof typeof CAMERA_ROTATION_KEYS;
type CameraControlKey = CameraMoveKey | CameraZoomKey | CameraRotationKey;

export class KeyboardManager {
    private readonly activeCameraKeys = new Set<CameraControlKey>();

    constructor(private readonly controller: KeyboardController) {}

    start(): void {
        window.addEventListener("keydown", this.handleKeyDown);
        window.addEventListener("keyup", this.handleKeyUp);
        window.addEventListener("blur", this.releaseCameraKeys);
    }

    stop(): void {
        window.removeEventListener("keydown", this.handleKeyDown);
        window.removeEventListener("keyup", this.handleKeyUp);
        window.removeEventListener("blur", this.releaseCameraKeys);
        this.releaseCameraKeys();
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

        if (isPlainKeyEvent(event)) {
            const key = event.key.toLowerCase();
            if (isCameraControlKey(key)) {
                event.preventDefault();
                this.setCameraKeyActive(key, true);
                return;
            }
        }

        if (!isArrowKey(event.key)) return;

        if (this.controller.navigate(ARROW_TO_TARGET[event.key])) {
            event.preventDefault();
        }
    };

    private readonly handleKeyUp = (event: KeyboardEvent): void => {
        const key = event.key.toLowerCase();
        if (!isCameraControlKey(key) || !this.activeCameraKeys.has(key)) return;

        event.preventDefault();
        this.setCameraKeyActive(key, false);
    };

    private readonly releaseCameraKeys = (): void => {
        this.activeCameraKeys.clear();
        this.controller.releaseCameraControls();
    };

    private setCameraKeyActive(key: CameraControlKey, active: boolean): void {
        if (active) {
            if (this.activeCameraKeys.has(key)) return;
            this.activeCameraKeys.add(key);
        } else {
            this.activeCameraKeys.delete(key);
        }

        if (isCameraMoveKey(key)) {
            this.controller.setCameraMoveActive(CAMERA_MOVE_KEYS[key], active);
            return;
        }
        if (isCameraZoomKey(key)) {
            this.controller.setCameraZoomActive(CAMERA_ZOOM_KEYS[key], active);
            return;
        }
        this.controller.setCameraRotationActive(CAMERA_ROTATION_KEYS[key], active);
    }
}

function isArrowKey(key: string): key is ArrowKey {
    return key in ARROW_TO_TARGET;
}

function isCameraMoveKey(key: string): key is CameraMoveKey {
    return key in CAMERA_MOVE_KEYS;
}

function isCameraZoomKey(key: string): key is CameraZoomKey {
    return key in CAMERA_ZOOM_KEYS;
}

function isCameraRotationKey(key: string): key is CameraRotationKey {
    return key in CAMERA_ROTATION_KEYS;
}

function isCameraControlKey(key: string): key is CameraControlKey {
    return isCameraMoveKey(key) || isCameraZoomKey(key) || isCameraRotationKey(key);
}

function isPlainKeyEvent(event: KeyboardEvent): boolean {
    return !event.ctrlKey && !event.metaKey && !event.altKey;
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
