import { describe, expect, mock, test } from "bun:test";

import { KeyboardController, type KeyboardNavigation } from "../src/core/keyboardController";

interface ControllerState {
    selectedId?: string;
    navigation?: KeyboardNavigation;
    settingsOpen: boolean;
}

function setup(initialState: Partial<ControllerState> = {}) {
    const state: ControllerState = {
        settingsOpen: false,
        ...initialState,
    };
    const onMuteToggle = mock();
    const onSettingsOpen = mock(() => {
        state.settingsOpen = true;
    });
    const onSettingsClose = mock(() => {
        state.settingsOpen = false;
    });
    const controller = new KeyboardController({
        getSelected: () => (state.navigation ? { navigation: state.navigation } : undefined),
        getSelectedId: () => state.selectedId,
        setSelectedId: id => {
            state.selectedId = id;
        },
        isSettingsOpen: () => state.settingsOpen,
        onMuteToggle,
        onSettingsOpen,
        onSettingsClose,
    });

    return { controller, state, onMuteToggle, onSettingsOpen, onSettingsClose };
}

describe("KeyboardController", () => {
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

    test("does not change the selection when the navigation target is absent", () => {
        const { controller, state } = setup({ selectedId: "selected", navigation: {} });

        expect(controller.navigate("childId")).toBe(false);
        expect(state.selectedId).toBe("selected");
    });
});
