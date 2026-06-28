export interface KeyboardNavigation {
    parentId?: string;
    childId?: string;
    previousSiblingId?: string;
    nextSiblingId?: string;
}

export type KeyboardNavigationTarget = keyof KeyboardNavigation;
export type CameraMoveDirection = "up" | "down" | "left" | "right";
export type CameraZoomDirection = "in" | "out";
export type CameraRotationDirection = "up" | "down" | "left" | "right";

interface SelectedChannel {
    navigation?: KeyboardNavigation;
}

interface KeyboardControllerOptions {
    getSelected: () => SelectedChannel | undefined;
    getSelectedId: () => string | undefined;
    setSelectedId: (id: string | undefined) => void;
    isSettingsOpen: () => boolean;
    onMuteToggle: () => void;
    onSettingsOpen: () => void;
    onSettingsClose: () => void;
    onCameraMoveChange: (direction: CameraMoveDirection, active: boolean) => void;
    onCameraZoomChange: (direction: CameraZoomDirection, active: boolean) => void;
    onCameraRotateChange: (direction: CameraRotationDirection, active: boolean) => void;
    onCameraControlsRelease: () => void;
}

export class KeyboardController {
    constructor(private readonly options: KeyboardControllerOptions) {}

    handleEscape(): void {
        if (this.options.isSettingsOpen()) {
            this.options.onSettingsClose();
            return;
        }

        if (this.options.getSelectedId()) {
            this.options.setSelectedId(undefined);
            return;
        }

        this.options.onSettingsOpen();
    }

    toggleMute(): void {
        this.options.onMuteToggle();
    }

    navigate(target: KeyboardNavigationTarget): boolean {
        const nextSelectedId = this.options.getSelected()?.navigation?.[target];
        if (!nextSelectedId) return false;

        this.options.setSelectedId(nextSelectedId);
        return true;
    }

    setCameraMoveActive(direction: CameraMoveDirection, active: boolean): void {
        this.options.onCameraMoveChange(direction, active);
    }

    setCameraZoomActive(direction: CameraZoomDirection, active: boolean): void {
        this.options.onCameraZoomChange(direction, active);
    }

    setCameraRotationActive(direction: CameraRotationDirection, active: boolean): void {
        this.options.onCameraRotateChange(direction, active);
    }

    releaseCameraControls(): void {
        this.options.onCameraControlsRelease();
    }
}
