import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: '.',
  testMatch: 'visual.spec.ts',
  snapshotPathTemplate: '{testDir}/references/{arg}{ext}',
  use: { baseURL: 'http://127.0.0.1:6006' },
});
