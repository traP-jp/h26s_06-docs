export interface KeyboardNavigation {
    parentId?: string;
    childId?: string;
    previousSiblingId?: string;
    nextSiblingId?: string;
}

export type KeyboardNavigationTarget = keyof KeyboardNavigation;

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
}
