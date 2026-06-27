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
}

const ARROW_TO_TARGET = {
    ArrowUp: "childId",
    ArrowDown: "parentId",
    ArrowLeft: "previousSiblingId",
    ArrowRight: "nextSiblingId",
} as const;

type ArrowKey = keyof typeof ARROW_TO_TARGET;

export function useKeyboardManager({ selected, selectedId }: KeyboardManagerOptions): void {
    function handleKeyDown(event: KeyboardEvent): void {
        if (isEditableEventTarget(event.target)) return;
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

function isEditableEventTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) return false;

    return (
        target.isContentEditable ||
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement
    );
}
