import type styles from "./styles.json";

export type CompanionEngine = {
  setEmotion: (id: string) => void;
  setActive: (active: boolean) => void;
  renderStatic: () => void;
  destroy: () => void;
  ball: {
    perform: (name: string) => boolean;
    cancelAction: () => void;
    setReduced: (reduced: boolean) => void;
    setResolution: (size: 256 | 512) => boolean;
  };
};
export type CompanionReaction = {
  id: string;
  emotion: string;
  group: "life" | "emotion" | "work" | "action";
  action?: string;
  duration: number;
};
export const companionReactions: readonly CompanionReaction[];
export function createCompanion(
  element: HTMLElement,
  options: {
    character: "ally";
    emotion: string;
    style?: keyof typeof styles;
    resolution?: 256 | 512;
    autostart: false;
    lite: false;
    idle: false;
  },
): CompanionEngine;
