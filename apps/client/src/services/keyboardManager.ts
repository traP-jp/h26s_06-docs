import { onBeforeUnmount, onMounted } from "vue";
import type { Ref } from "vue";

interface NavigationTargets {
    parentId?: string;
    childId?: string;
    previousSiblingId?: string;
    nextSiblingId?: string;
}

interface SelectedChannel {
    navigation?: NavigationTargets;
}

interface KeyboardManagerOptions {
    selected: Readonly<Ref<SelectedChannel | undefined>>;
    selectedId: Ref<string | undefined>;

    muted?: Ref<boolean>;
    settingsOpen?: Ref<boolean>;

    onMuteToggle?: () => void;
    onSettingsOpen?: () => void;
    onSettingsClose?: () => void;
}

const ARROW_TO_TARGET = {
    ArrowUp: "childId",
    ArrowDown: "parentId",
    ArrowLeft: "previousSiblingId",
    ArrowRight: "nextSiblingId",
} as const;

type ArrowKey = keyof typeof ARROW_TO_TARGET;

export function useKeyboardManager({
    selected,
    selectedId,
    muted,
    settingsOpen,
    onMuteToggle,
    onSettingsOpen,
    onSettingsClose,
}: KeyboardManagerOptions): void {
    function handleKeyDown(event: KeyboardEvent): void {
        if (event.key === "Escape") {
            event.preventDefault();

            if (settingsOpen?.value) {
                if (onSettingsClose) {
                    onSettingsClose();
                } else {
                    settingsOpen.value = false;
                }

                return;
            }

            if (selectedId.value) {
                selectedId.value = undefined;
                return;
            }

            if (onSettingsOpen) {
                onSettingsOpen();
            } else if (settingsOpen) {
                settingsOpen.value = true;
            }

            return;
        }

        if (isEditableEventTarget(event.target)) return;

        if (isMuteKey(event)) {
            event.preventDefault();

            if (onMuteToggle) {
                onMuteToggle();
            } else if (muted) {
                muted.value = !muted.value;
            }

            return;
        }

        if (!isArrowKey(event.key)) return;

        const targetKey = ARROW_TO_TARGET[event.key];
        const nextSelectedId = selected.value?.navigation?.[targetKey];

        if (!nextSelectedId) return;

        event.preventDefault();
        selectedId.value = nextSelectedId;
    }

    onMounted(() => {
        window.addEventListener("keydown", handleKeyDown);
    });

    onBeforeUnmount(() => {
        window.removeEventListener("keydown", handleKeyDown);
    });
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