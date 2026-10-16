export type ElasticMeshOptions = {
  text: string;
  color: string;
  colorEnd?: string;
  theme: "light" | "dark";
  intro?: boolean;
  dpr?: number;
  onReady?: () => void;
  onIntroComplete?: () => void;
  onError?: () => void;
};

export function mountElasticMesh(
  container: HTMLElement,
  options: ElasticMeshOptions,
): {
  available: boolean;
  destroy(): void;
  update(options: Partial<ElasticMeshOptions> & { light?: boolean }): void;
};
