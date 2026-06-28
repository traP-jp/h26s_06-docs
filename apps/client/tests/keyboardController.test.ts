import { describe, expect, mock, test } from "bun:test";

import { KeyboardController, type KeyboardNavigation } from "../src/core/keyboardController";

interface ControllerState {
    selectedId?: string;
    lastSelectedId?: string;
    navigation?: KeyboardNavigation;
    shortcutsOpen: boolean;
    settingsOpen: boolean;
}

function setup(initialState: Partial<ControllerState> = {}) {
    const state: ControllerState = {
        shortcutsOpen: false,
        settingsOpen: false,
        ...initialState,
    };
    const onMuteToggle = mock();
    const onSettingsOpen = mock(() => {
        state.settingsOpen = true;
    });
    const onShortcutsClose = mock(() => {
        state.shortcutsOpen = false;
    });
    const onSettingsClose = mock(() => {
        state.settingsOpen = false;
    });
    const controller = new KeyboardController({
        getSelected: () => (state.navigation ? { navigation: state.navigation } : undefined),
        getSelectedId: () => state.selectedId,
        getLastSelectedId: () => state.lastSelectedId,
        setSelectedId: id => {
            state.selectedId = id;
        },
        isShortcutsOpen: () => state.shortcutsOpen,
        isSettingsOpen: () => state.settingsOpen,
        onMuteToggle,
        onShortcutsClose,
        onSettingsOpen,
        onSettingsClose,
    });

    return {
        controller,
        state,
        onMuteToggle,
        onShortcutsClose,
        onSettingsOpen,
        onSettingsClose,
    };
}

describe("KeyboardController", () => {
    test("closes shortcuts on Escape before closing settings", () => {
        const { controller, state, onShortcutsClose, onSettingsClose } = setup({
            selectedId: "selected",
            shortcutsOpen: true,
            settingsOpen: true,
        });

        controller.handleEscape();

        expect(onShortcutsClose).toHaveBeenCalledTimes(1);
        expect(onSettingsClose).not.toHaveBeenCalled();
        expect(state.settingsOpen).toBe(true);
        expect(state.selectedId).toBe("selected");
    });

    test("closes settings on Escape before changing the selection", () => {
        const { controller, state, onSettingsClose } = setup({
            selectedId: "selected",
            settingsOpen: true,
        });

        controller.handleEscape();

        expect(onSettingsClose).toHaveBeenCalledTimes(1);
        expect(state.selectedId).toBe("selected");
    });

    test("clears the selection on Escape when settings are closed", () => {
        const { controller, state } = setup({ selectedId: "selected" });

        controller.handleEscape();

        expect(state.selectedId).toBeUndefined();
    });

    test("opens settings on Escape when there is no selection", () => {
        const { controller, onSettingsOpen } = setup();

        controller.handleEscape();

        expect(onSettingsOpen).toHaveBeenCalledTimes(1);
    });

    test("toggles mute", () => {
        const { controller, onMuteToggle } = setup();

        controller.toggleMute();

        expect(onMuteToggle).toHaveBeenCalledTimes(1);
    });

    test("selects the requested navigation target", () => {
        const { controller, state } = setup({
            selectedId: "selected",
            navigation: { parentId: "parent" },
        });

        expect(controller.navigate("parentId")).toBe(true);
        expect(state.selectedId).toBe("parent");
    });

    test("restores the last selection before navigating when no node is selected", () => {
        const { controller, state } = setup({
            lastSelectedId: "previous",
            navigation: { parentId: "parent" },
        });

        expect(controller.navigate("parentId")).toBe(true);
        expect(state.selectedId).toBe("previous");
    });

    test("selects grand root without a current or previous selection", () => {
        const { controller, state } = setup({
            navigation: { parentId: "parent" },
        });

        expect(controller.navigate("parentId")).toBe(true);
        expect(state.selectedId).toBe("grand_root");
    });

    test("does not change the selection when the navigation target is absent", () => {
        const { controller, state } = setup({ selectedId: "selected", navigation: {} });

        expect(controller.navigate("childId")).toBe(false);
        expect(state.selectedId).toBe("selected");
    });
});
