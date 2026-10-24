import { startApplication } from "@neuxally/frontend/start";

const { configureDesktopPlatform } = await import("./platform");
configureDesktopPlatform();
startApplication();
