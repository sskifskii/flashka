import { defineConfig, devices } from "@playwright/test";
const baseURL = "http://localhost:18080";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  webServer: {
    command: "cd .. && go run ./cmd/server",
    url: `${baseURL}/health/ready`,
    reuseExistingServer: false,
    env: { HTTP_ADDR: ":18080", TRUSTED_ORIGIN: baseURL },
    timeout: 120_000,
  },
  use: { baseURL, trace: "retain-on-failure" },
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
