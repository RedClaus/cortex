/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_USE_MOCK_CORTEX: string;
  readonly VITE_CORTEX_URL: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
