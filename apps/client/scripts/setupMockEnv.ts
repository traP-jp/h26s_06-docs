import { copyFileSync, existsSync } from "node:fs";
import { join } from "node:path";

const envPath = join(__dirname, "../../../mocks/server/.env");
const examplePath = join(__dirname, "../../../mocks/server/.env.example");

if (!existsSync(envPath) && existsSync(examplePath)) {
    copyFileSync(examplePath, envPath);
    console.log("Created .env from .env.example for mocks/server");
} else if (existsSync(envPath)) {
    console.log(".env already exists for mocks/server");
} else {
    console.warn("Could not find .env.example to copy");
}
