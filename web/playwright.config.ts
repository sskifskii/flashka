import { defineConfig, devices } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  webServer: {
    command: "cd .. && go run ./cmd/server",
    url: "http://localhost:8080/health/ready",
    reuseExistingServer: true,
    timeout: 120_000,
  },
  use: { baseURL: "http://localhost:8080", trace: "retain-on-failure" },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"] } },
    {
      name: "mobile",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 360, height: 800 },
      },
    },
  ],
});
